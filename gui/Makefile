BINARY = ssl-vision-processor-gui

# ./... would also descend into frontend/node_modules, where npm packages ship
# stray Go source (flatted/golang, for one). Enumerate our own trees instead.
GO_PACKAGES = ./cmd/... ./internal/... ./frontend

# What the frontend build reads: everything in frontend/ except dependencies,
# build and test output, and generated code (which has its own rules below).
FRONTEND_SOURCES = $(shell find frontend -type f \
	-not -path "frontend/node_modules/*" \
	-not -path "frontend/dist/*" \
	-not -path "frontend/src/proto/*" \
	-not -path "frontend/src/lib/generated/*" \
	-not -path "frontend/test-results/*" \
	-not -path "frontend/playwright-report/*")

PROTO_SOURCES = $(shell find ../proto/proto -type f 2>/dev/null)

# What the OpenAPI spec is built from: the host's Go code.
GO_SOURCES = $(shell find cmd internal -name '*.go' -not -name '*_test.go') go.mod go.sum

# sha256sum, or macOS's equivalent outside nix.
SHA256SUM = $(shell command -v sha256sum >/dev/null 2>&1 && echo sha256sum || echo "shasum -a 256")

.DEFAULT_GOAL := help

.PHONY: help run install test check e2e vet build go-test frontend-test fmt-check generate-check \
	multicast-note e2e-if-chrome npm proto redoc comments openapi frontend generate clean

## Running

help: ## List these targets
	@awk 'BEGIN { FS = ":.*## " } \
		/^## / { printf "\n%s\n", substr($$0, 4) } \
		/^[a-z][a-z0-9-]*:.*## / { printf "  make %-10s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# Run from the repo root, as vision_processor is: the host's defaults
# (vision.yml, the presets in config/legacy/, img/, logs/) are relative to it.
run: frontend ## Build everything and run the host from the repo root; flags go in ARGS="..."
	go build -o bin/$(BINARY) ./cmd/$(BINARY)
	cd .. && gui/bin/$(BINARY) $(ARGS)

install: frontend ## go install the host
	go install -v ./cmd/$(BINARY)

## Checking

test: go-test frontend-test ## Go tests (-race), and the frontend's checks and unit tests

check: multicast-note fmt-check vet go-test frontend-test generate-check e2e-if-chrome ## Everything CI checks for gui/

e2e: frontend ## Browser tests: the real host in Google Chrome (frontend/e2e/)
	go build -o $(E2E_BIN) ./cmd/$(BINARY)
	cd frontend && GUI_BIN=$(E2E_BIN) npx playwright test; status=$$?; rm -f $(E2E_BIN); exit $$status

vet: frontend ## go vet every package
	go vet $(GO_PACKAGES)

build: frontend ## go build every package, without installing
	go build $(GO_PACKAGES)

go-test: frontend
	go test $(GO_PACKAGES) -race

frontend-test: frontend
	cd frontend && npm run check && npm run lint && npm run check:text && npm test && npm run format:check

fmt-check: .proto
	@unformatted=$$(gofmt -l $$(find cmd internal frontend -name '*.go' -not -path '*/node_modules/*')); \
		if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi

generate-check: generate
	@git diff --exit-code -- frontend/src/lib/alerts/alert-codes.json || \
		{ echo "alert-codes.json was stale; commit the regenerated file"; exit 1; }

# The real-multicast tests skip, rather than fail, when loopback multicast is
# off; say so up front so a green run isn't mistaken for full coverage.
multicast-note:
	@../scripts/lo-multicast.sh status >/dev/null || \
		echo "note: loopback multicast is off, so the socket and video tests will skip (../scripts/lo-multicast.sh on)"

# Playwright drives the installed Google Chrome, which nix doesn't provide.
e2e-if-chrome:
	@if command -v google-chrome >/dev/null 2>&1 || [ -x /opt/google/chrome/chrome ]; then \
		$(MAKE) --no-print-directory e2e; \
	else \
		echo "note: Google Chrome not found, skipping the browser tests (make e2e)"; \
	fi

# The binary goes to a temp dir so a stale one in the tree is never what's
# tested.
E2E_BIN := $(shell mktemp -u -t vision-gui-e2e-bin.XXXXXX)

## Building

# npm ci, rerun whenever the lockfile changes, so a pull that adds a package
# installs it. npm writes this file on every install.
NPM = frontend/node_modules/.package-lock.json

$(NPM): frontend/package.json frontend/package-lock.json
	cd frontend && npm ci
	touch $@

npm: $(NPM) ## Install the frontend's npm packages

# internal/vision, internal/gamecontroller and frontend/src/proto are generated,
# not committed (see buf.gen.yaml and .gitignore), so every Go and frontend
# target needs this first. Both of buf.gen.yaml's plugins are local (go tool
# protoc-gen-go; frontend/node_modules/.bin/protoc-gen-es), so this needs no
# network beyond the already checked out proto/ submodule.
.proto: buf.gen.yaml go.mod $(PROTO_SOURCES) $(NPM)
	buf generate
	touch .proto

proto: .proto ## Generate the protobuf bindings (Go and TypeScript)

# The API docs viewer: Redoc's standalone bundle (MIT), embedded in the binary
# so /api/docs works on a venue network with no internet. Fetched at a pinned
# version rather than committed; REDOC_SHA256 catches a changed download. The
# stamp names the version, so changing it fetches again.
REDOC_VERSION = 2.5.1
REDOC_SHA256 = 2d8d13e94b5a21011cc6f8260d87ca475d0f6f2faed16dd12e7394dfa3bb05b8
REDOC_STAMP = internal/api/viewer/.redoc-$(REDOC_VERSION)

$(REDOC_STAMP):
	rm -rf internal/api/viewer && mkdir -p internal/api/viewer
	tmp=$$(mktemp -d) && cd $$tmp && \
		npm pack redoc@$(REDOC_VERSION) --silent >/dev/null && \
		tar -xzf redoc-$(REDOC_VERSION).tgz package/LICENSE package/bundles/redoc.standalone.js package/bundles/redoc.standalone.js.LICENSE.txt && \
		echo "$(REDOC_SHA256)  package/bundles/redoc.standalone.js" | $(SHA256SUM) -c --quiet && \
		cp package/bundles/redoc.standalone.js package/bundles/redoc.standalone.js.LICENSE.txt $(CURDIR)/internal/api/viewer/ && \
		cp package/LICENSE $(CURDIR)/internal/api/viewer/REDOC-LICENSE && \
		rm -rf $$tmp
	touch $@

redoc: $(REDOC_STAMP) ## Fetch the API docs viewer (Redoc)

# Go doc comments for the OpenAPI spec (go generate ./internal/api). Not
# committed, like internal/vision; every Go build gets it through openapi.json
# below.
COMMENTS = internal/api/comments_gen.go

$(COMMENTS): $(filter-out $(COMMENTS),$(GO_SOURCES))
	cd internal/api && go run ./gen

comments: $(COMMENTS)

# The API's OpenAPI spec, written by the host itself (-openapi) from its Go
# code. Not committed; the host also serves it at /api/openapi.json. Compiling
# the host embeds the docs viewer, hence the stamp.
openapi.json: .proto $(REDOC_STAMP) $(COMMENTS) $(GO_SOURCES)
	go run ./cmd/$(BINARY) -openapi $@

openapi: openapi.json ## Write the API's OpenAPI spec to openapi.json

# The frontend's API types, generated from the spec (openapi-typescript). Not
# committed, like frontend/src/proto.
frontend/src/lib/generated/api.ts: $(NPM) openapi.json
	cd frontend && npm run gen:api

# Sentinel, so the frontend rebuilds only when one of its sources changes.
# Every Go target depends on this, since the host embeds frontend/dist.
#
# The host is also compiled before the frontend's first build, to write
# openapi.json, and //go:embed fails without a file in dist/: hence the
# committed dist/.gitkeep. `npm run build` empties dist/ first (Vite's
# emptyOutDir), so restore it, or a build before a commit drops it.
.frontend: .proto frontend/src/lib/generated/api.ts $(FRONTEND_SOURCES)
	cd frontend && npm run build
	touch frontend/dist/.gitkeep
	touch .frontend

frontend: .frontend ## Build the frontend, and everything it needs

# The one committed file derived from the code: the alert codes the
# frontend's tests check. generate-check (and CI) fails when it's stale.
generate: .proto ## Regenerate the committed alert codes file
	go test ./internal/alerts -run CodesFileIsCurrent -update

clean: ## Remove everything built, generated or fetched
	rm -rf bin frontend/dist frontend/src/proto frontend/src/lib/generated internal/vision internal/gamecontroller \
		internal/api/viewer $(COMMENTS) openapi.json .frontend .proto
	mkdir -p frontend/dist
	touch frontend/dist/.gitkeep
