# gui

Go host and browser UI for vision_processor. Owns the shared field geometry, absorbs camera calibrations from
vision_processor instances over multicast, and serves the frontend, a JSON API, and a WebSocket for live updates,
all on one port. See [ARCHITECTURE.md](ARCHITECTURE.md) for how it fits together and why it is built this way.

This document covers development on `gui` itself. For running vision_processor at a venue, see the root
[README.md](../README.md). For the frontend's own layout and scripts, see
[frontend/README.md](frontend/README.md).

## Requirements

The root `flake.nix` provides everything needed: `go`, `buf`, `nodejs_22`, and the C++ toolchain for the rest of
the repo. Run `nix develop` from the repo root before any command below.

Without nix, you need `buf` on your `PATH`, a Go toolchain matching the `go` directive in `gui/go.mod`, and
Node 22. `protoc-gen-go` and `protoc-gen-es` do not need to be installed separately: `go tool protoc-gen-go` is
resolved from `gui/go.mod`'s `tool` directive, and `protoc-gen-es` is a `frontend/package.json` devDependency
installed by `npm ci`.

## Commands

Run these from `gui/`.

| Command        | Effect                                                                        |
| -------------- | -------------------------------------------------------------------------------|
| `make run`     | Regenerate protobuf bindings, build the frontend, and run the Go host on `:8085`. |
| `make test`    | Frontend check, lint, and format, plus `go test -race` for every package.     |
| `make install` | `go install` after a frontend build.                                          |
| `make proto`   | Regenerate `internal/vision`, `internal/gamecontroller`, and `frontend/src/proto`. |
| `make vet`     | `go vet` for every package.                                                    |
| `make build`   | `go build` for every package, without installing.                            |
| `make clean`   | Remove the frontend build, generated protobuf code, and build sentinels.      |

`make proto` needs `git submodule update --init` to have been run once, but no network beyond that: both of
`buf.gen.yaml`'s plugins are local, not buf.build remotes. `make run`, `make test`, and `make install` all depend
on it, so a fresh checkout builds correctly without a separate manual step.

For frontend only work with hot reload:

```
cd frontend
npm run dev
```

This proxies `/api` and `/ws` to the Go host, so `gui`'s own server still needs to be running (`make run` in a
second terminal) for the page to show anything.

## Repository layout

```
cmd/ssl-vision-processor-gui/   entry point: flags, wiring, HTTP server
internal/
  geometry/    field config, calibration merge, the 1Hz publish loop, corner picker writeback
  multicast/   bridge to the SSL vision multicast group
  hub/         topic pub/sub and the /ws handler
  snapshot/    debug image listing and serving
  logging/     slog setup
  vision/      generated protobuf bindings, not committed
  gamecontroller/  generated protobuf bindings, not committed
frontend/      Svelte 5, TypeScript, and Vite, embedded via //go:embed
buf.gen.yaml   buf configuration for the two packages above
```

## Testing

`go test -race` runs on every package. This code is concurrent by construction: a mutex guarded `Geometry` and a
multi goroutine WebSocket hub. Tests that exercise a file write (`internal/geometry`, `cmd/.../server_test.go`)
copy their fixture into `t.TempDir()` first and never operate on the checked in `testdata/` files directly.

`go test ./...` from `gui/` will try to compile stray Go source inside `frontend/node_modules`. Use
`go test ./cmd/... ./internal/... ./frontend`, which is what `Makefile` and CI both do.
