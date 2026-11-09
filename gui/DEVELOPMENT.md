# Developing the GUI

How to build, test, and release `gui`: the Go host and browser UI for vision_processor. It owns `vision.yml` (the
field geometry, per camera settings, and locked calibrations), absorbs camera calibrations from vision_processor
instances over multicast, and serves the frontend, a JSON API, and a WebSocket for live updates, all on one port.

- To run the GUI rather than change it, see [README.md](README.md).
- For how it fits together and why, see [ARCHITECTURE.md](ARCHITECTURE.md).
- For the frontend's layout and scripts, see [frontend/README.md](frontend/README.md).
- For running vision_processor at a venue, see the root [README.md](../README.md).

## Requirements

The root `flake.nix` provides everything needed: `go`, `buf`, `nodejs_22`, and the C++ toolchain for the rest of
the repo. Run `nix develop` from the repo root before any command below, and `git submodule update --init` once
for the protobuf definitions in `proto/`.

Without nix, you need `buf` 1.72.0 on your `PATH` (`go install github.com/bufbuild/buf/cmd/buf@v1.72.0`), a Go toolchain matching the `go` directive in `gui/go.mod`, and
Node 22 (`frontend/.nvmrc`). `protoc-gen-go` and `protoc-gen-es` do not need to be installed separately:
`go tool protoc-gen-go` is resolved from `gui/go.mod`'s `tool` directive, and `protoc-gen-es` is a
`frontend/package.json` devDependency.

The browser tests (`make e2e`, part of `make check`) drive the installed Google Chrome, which nix doesn't
provide; `make check` skips them when it isn't found. The socket and video tests need multicast on loopback
(`../scripts/lo-multicast.sh on`, root, until reboot) and skip without it.

## Commands

Run these from `gui/`; `make` alone lists them. Each builds whatever it needs first: npm packages (reinstalled
when `frontend/package-lock.json` changes), the protobuf bindings, the embedded defaults, the API docs viewer, the
OpenAPI spec and the TypeScript types generated from it, and the frontend. None of those are committed.

| Command              | Effect                                                                                          |
| -------------------- | ----------------------------------------------------------------------------------------------- |
| `make run`           | Build everything and run the host from the repo root on `:8085`. Flags go in `ARGS="..."`.      |
| `make test`          | `go test -race` for every package, and the frontend's type checks, lint, unit tests and format. |
| `make check`         | Everything CI checks: gofmt, vet, the tests, the committed generated file, the browser tests.   |
| `make e2e`           | Playwright browser tests: the real host in the installed Google Chrome.                         |
| `make vet`           | `go vet` for every package.                                                                     |
| `make build`         | `go build` for every package, without installing.                                               |
| `make install`       | `go install` the host.                                                                          |
| `make openapi`       | Write the API's OpenAPI spec to `openapi.json`.                                                 |
| `make generate`      | Regenerate the one committed generated file, `frontend/src/lib/alerts/alert-codes.json`.        |
| `make release`       | Build the static release binary, its tarball, and `SHA256SUMS` in `bin/release/`.               |
| `make release-check` | Build the release, then run it as a new user would: no arguments, an empty home.                |
| `make clean`         | Remove everything built, generated or fetched.                                                  |

`make proto` needs no network beyond the checked out `proto/` submodule: both of `buf.gen.yaml`'s plugins are
local, not buf.build remotes. The npm packages and the docs viewer come from npm.

For frontend only work with hot reload:

```
make frontend           # once, for the generated code
cd frontend && npm run dev
```

This proxies `/api` and `/ws` to the Go host, so `gui`'s own server still needs to be running (`make run` in a
second terminal) for the page to show anything.

## Configuration while developing

`make run` keeps everything in the repo root, rather than the installed default folders, by passing `-configDir .
-imgDir img -logFile logs/vision-processor-gui.log`. Your `ARGS` come after those, so they win. Config files there
are gitignored.

`config/vision.yml` at the repo root is a commented reference; copying it to `vision.yml` is the quickest start.
Without one, the host creates a default from the files embedded in `internal/defaults`, marked `first_run: true`.

`README.md` documents the flags and file locations that users see.

### Running vision_processor from the host

With `-runCameras`, the host starts one vision_processor per camera, using the `vision_processor` it finds on `PATH`.
There's no flag to name another binary: to use your own build, put it first on `PATH`. Every camera gets a
`config_path`, `cam<id>.yml`, if it has none. Each runs in its own folder, `run/cam<id>` in the state folder
(`~/.local/state/rcssl/vision-processor-gui`), with `img` there linking to `-imgDir`. It gets the full path of its
config file. A camera whose capture device another camera already uses isn't started until that's fixed.

Settings vision_processor hot reloads (thresholds, tracking, color, debug) only rewrite the file. A change to any
other setting (camera, geometry, network, stream) restarts it once edits stop for a second. The Camera Settings
page shows its state and output and has a Restart button. A crash is retried, waiting longer each time; a clean
exit, such as a video file ending, is started again after a second.

For a bench test, from the repo root:

```
PATH=$PWD/build:$PATH make run ARGS="-configFile config-camtest.vision.yml -runCameras"
```

Stop any vision_processor already using the camera first. A restart recalibrates geometry unless the camera's
calibration is locked. `make run` passes `-imgDir img`, so snapshots still land in the repo's `img/`.

`make release-check` also runs the release binary with `-runCameras` and a stand-in vision_processor script.

### Network settings

The vision and game controller multicast addresses are in `vision.yml`, under `defaults.network`; the autoref
tracker address, which only the host listens to, is under `tracker`. All three are set on the Network page, which
the address badges in the header open. Changing one reopens the host's sockets at once. The same page chooses which
network interfaces the host uses, automatically or by unticking the ones to skip; that choice is stored under
`host.interfaces` and applies to this host only. There are no command line flags for any of these.

## Repository layout

```
cmd/ssl-vision-processor-gui/   entry point: flags, paths, wiring, HTTP server
internal/
  config/      vision.yml: working document, save and load, locked calibrations, disk watcher,
               generated config.yml files
  defaults/    files embedded in the binary: rulebook fields, robot heights. Copied in by make
  geometry/    live field template, published calibrations, the 1Hz publish loop
  multicast/   vision, game controller and tracker sockets, interface selection
  hub/         topic pub/sub and the /ws handler
  livefield/   latest detections and autoref tracker frames, for the Live Field page
  snapshot/    debug image listing and serving
  video/       live video, relayed to the browser
  supervisor/  runs and restarts every camera's vision_processor (-runCameras)
  v4l/         this host's capture devices
  logging/     slog setup
  vision/      generated protobuf bindings, not committed
  gamecontroller/  generated protobuf bindings, not committed
frontend/      Svelte 5, TypeScript, and Vite, embedded via //go:embed
scripts/       release support: the release check, third-party notices
buf.gen.yaml   buf configuration for the two packages above
```

## Testing

`go test -race` runs on every package. This code is concurrent by construction: a mutex guarded `Geometry` and a
multi goroutine WebSocket hub. Tests that exercise a file write (`internal/config`, `cmd/.../server_test.go`)
copy their fixture into `t.TempDir()` first and never operate on the checked in `testdata/` files directly.

`go test ./...` from `gui/` will try to compile stray Go source inside `frontend/node_modules`. Use
`go test ./cmd/... ./internal/... ./frontend`, which is what `Makefile` and CI both do.

`make e2e` starts two hosts: one on a fixture for the smoke tests, and one on a first-run config for
`first-run.spec.ts`. Both use a private multicast group, since the fixture's calibration must never reach a real
network.

`make release-check` exercises the release binary from outside, with and without `-runCameras`. A first run publishes
nothing, so it's safe on any network.

## Live video

The Camera Settings tab shows the selected camera's live video beside its `camera:` settings (driver, device,
resolution, exposure, gain, gamma, white balance). Those settings apply when the camera's vision_processor
restarts. The Stream page also shows the selected camera's live video. The host relays the vision_processor's H.264
stream to the browser without re-encoding, and only while someone watches. The stream cycles through the
vision_processor's views unless its `stream.raw_feed` is set.

## Releasing

Pushing a `v*` tag runs `.github/workflows/release.yml`: it builds, runs `make release-check`, and publishes a GitHub
Release with the binary, tarball, and `SHA256SUMS`. To try the same build locally, run `make release-check`.

The release must be built with `CGO_ENABLED=0`, which `make release` sets. A default build under nix links glibc
through a `/nix/store` interpreter that doesn't exist on other machines. `make release-check` fails on a dynamic binary.

The version comes from `git describe`, and `-version` and `/api/health` report it. Other builds say `dev`.
