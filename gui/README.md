# gui

Go host and browser UI for vision_processor. Owns `vision.yml` (the field geometry, per camera settings, and
locked calibrations), absorbs camera calibrations from
vision_processor instances over multicast, and serves the frontend, a JSON API, and a WebSocket for live updates,
all on one port. See [ARCHITECTURE.md](ARCHITECTURE.md) for how it fits together and why it is built this way.

This document covers development on `gui` itself. For running vision_processor at a venue, see the root
[README.md](../README.md). For the frontend's own layout and scripts, see
[frontend/README.md](frontend/README.md).

## Requirements

The root `flake.nix` provides everything needed: `go`, `buf`, `nodejs_22`, and the C++ toolchain for the rest of
the repo. Run `nix develop` from the repo root before any command below, and `git submodule update --init` once
for the protobuf definitions in `proto/`.

Without nix, you need `buf` on your `PATH`, a Go toolchain matching the `go` directive in `gui/go.mod`, and
Node 22 (`frontend/.nvmrc`). `protoc-gen-go` and `protoc-gen-es` do not need to be installed separately:
`go tool protoc-gen-go` is resolved from `gui/go.mod`'s `tool` directive, and `protoc-gen-es` is a
`frontend/package.json` devDependency.

The browser tests (`make e2e`, part of `make check`) drive the installed Google Chrome, which nix doesn't
provide; `make check` skips them when it isn't found. The socket and video tests need multicast on loopback
(`../scripts/lo-multicast.sh on`, root, until reboot) and skip without it.

## Commands

Run these from `gui/`; `make` alone lists them. Each builds whatever it needs first: npm packages (reinstalled
when `frontend/package-lock.json` changes), the protobuf bindings, the API docs viewer, the OpenAPI spec and the
TypeScript types generated from it, and the frontend. None of those are committed.

| Command         | Effect                                                                                          |
| --------------- | ----------------------------------------------------------------------------------------------- |
| `make run`      | Build everything and run the host from the repo root on `:8085`. Flags go in `ARGS="..."`.      |
| `make test`     | `go test -race` for every package, and the frontend's type checks, lint, unit tests and format. |
| `make check`    | Everything CI checks: gofmt, vet, the tests, the committed generated file, the browser tests.   |
| `make e2e`      | Playwright browser tests: the real host in the installed Google Chrome.                         |
| `make vet`      | `go vet` for every package.                                                                     |
| `make build`    | `go build` for every package, without installing.                                               |
| `make install`  | `go install` the host.                                                                          |
| `make openapi`  | Write the API's OpenAPI spec to `openapi.json`.                                                 |
| `make generate` | Regenerate the one committed generated file, `frontend/src/lib/alerts/alert-codes.json`.        |
| `make clean`    | Remove everything built, generated or fetched.                                                  |

## Configuration

The host reads and writes one file, `-config` (default `vision.yml`). Edits made in the browser apply live and
are written to the file only on Save, after a confirmation that lists every change. The host watches the file,
so a hand edit prompts the browser to load it or overwrite it.

`config/vision.yml` at the repo root is a commented reference; copying it is the quickest start. If
`-config` does not exist yet, the host creates it on startup. It imports `-importGeometry` (default
`geometry.yml`) and `-importConfig` (default `config.yml`) when they exist, and otherwise starts from
`-geometryPreset` (default `config/legacy/geometry-divB.yml`). An imported `config.yml` becomes a camera with that file as its `config_path`, so the host
regenerates it from `vision.yml` from then on. Keep a copy if you want its comments.

The vision and game controller multicast addresses are in the same file, under `defaults.network`. They are
set on the Network page, which the two address badges in the header open. Changing one reopens the host's
sockets at once. The same page chooses which network interfaces the host uses, automatically or by unticking
the ones to skip; that choice is stored under `host.interfaces` and applies to this host only. There are no
command line flags for any of these.
`make proto` needs no network beyond the checked out `proto/` submodule: both of `buf.gen.yaml`'s plugins are
local, not buf.build remotes. The npm packages and the docs viewer come from npm.

For frontend only work with hot reload:

```
make frontend           # once, for the generated code
cd frontend && npm run dev
```

This proxies `/api` and `/ws` to the Go host, so `gui`'s own server still needs to be running (`make run` in a
second terminal) for the page to show anything.

## Repository layout

```
cmd/ssl-vision-processor-gui/   entry point: flags, wiring, HTTP server
internal/
  config/      vision.yml: working document, save and load, locked calibrations, disk watcher,
               generated config.yml files
  geometry/    live field template, published calibrations, the 1Hz publish loop
  multicast/   vision and game controller sockets, interface selection
  hub/         topic pub/sub and the /ws handler
  snapshot/    debug image listing and serving
  video/       live video, relayed to the browser
  v4l/         this host's capture devices
  logging/     slog setup
  vision/      generated protobuf bindings, not committed
  gamecontroller/  generated protobuf bindings, not committed
frontend/      Svelte 5, TypeScript, and Vite, embedded via //go:embed
buf.gen.yaml   buf configuration for the two packages above
```

## Testing

`go test -race` runs on every package. This code is concurrent by construction: a mutex guarded `Geometry` and a
multi goroutine WebSocket hub. Tests that exercise a file write (`internal/config`, `cmd/.../server_test.go`)
copy their fixture into `t.TempDir()` first and never operate on the checked in `testdata/` files directly.

`go test ./...` from `gui/` will try to compile stray Go source inside `frontend/node_modules`. Use
`go test ./cmd/... ./internal/... ./frontend`, which is what `Makefile` and CI both do.

## Live video

The Camera Settings tab shows the selected camera's live video beside its `camera:` settings (driver, device,
resolution, exposure, gain, gamma, white balance). Those settings apply when the camera's vision_processor
restarts. The Stream page also shows the selected camera's live video. The host relays the vision_processor's H.264 stream to
the browser without re-encoding, and only while someone watches. The browser needs H.264 support: Google Chrome
bundles it, while Chromium and Firefox on Linux use the system's FFmpeg libraries. The stream cycles through the
vision_processor's views unless its `stream.raw_feed` is set.
