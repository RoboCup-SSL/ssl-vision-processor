BINARY = ssl-vision-processor-gui

# ./... would also descend into frontend/node_modules, where npm packages ship
# stray Go source (flatted/golang, for one). Enumerate our own trees instead.
GO_PACKAGES = ./cmd/... ./internal/... ./frontend

FRONTEND_SOURCES = $(shell find frontend -type f \
	-not -path "frontend/node_modules/*" \
	-not -path "frontend/dist/*" \
	-not -path "frontend/src/proto/*")

PROTO_SOURCES = $(shell find ../proto/proto -type f 2>/dev/null)

.PHONY: all frontend install test go-test frontend-test vet build run proto clean

all: install

# Sentinel file: internal/vision, internal/gamecontroller and
# frontend/src/proto are generated, not committed (see buf.gen.yaml and
# .gitignore), so every Go and frontend target needs this to have run first.
# Both of buf.gen.yaml's plugins are local (go tool protoc-gen-go;
# frontend/node_modules/.bin/protoc-gen-es), so this needs no network beyond
# the already-checked-out proto/ submodule -- npm ci runs first only to make
# sure the protoc-gen-es binary exists before buf calls it.
.proto: buf.gen.yaml go.mod $(PROTO_SOURCES)
	cd frontend && npm ci
	buf generate
	touch .proto

proto: .proto

# Sentinel file, so npm reruns only when a frontend source is newer than the
# last build. //go:embed resolves at compile time, which is why every Go target
# below depends on this. Depends on .proto first: the build imports the
# generated frontend/src/proto types.
#
# `npm run build` empties dist/ first (Vite's emptyOutDir), which deletes
# dist/.gitkeep along with any stale output. Restore it every time, or a build
# run before a commit silently drops the tracked sentinel a fresh clone needs.
.frontend: .proto $(FRONTEND_SOURCES)
	cd frontend && npm run build
	touch frontend/dist/.gitkeep
	touch .frontend

frontend: .frontend

install: frontend
	go install -v $(GO_PACKAGES)

vet: frontend
	go vet $(GO_PACKAGES)

build: frontend
	go build $(GO_PACKAGES)

go-test: frontend
	go test $(GO_PACKAGES) -race

frontend-test: frontend
	cd frontend && npm run check && npm run lint && npm run format:check

# CI runs go-test and frontend-test as separate parallel jobs (see
# .github/workflows/ci.yml) -- this is the single "everything before you
# push" target for local use, composed of exactly those same two so the
# package/command lists live in one place either way.
test: go-test frontend-test

run: frontend
	go run ./cmd/$(BINARY)

clean:
	rm -rf frontend/dist frontend/src/proto internal/vision internal/gamecontroller .frontend .proto
	mkdir -p frontend/dist
	touch frontend/dist/.gitkeep
