# CLAUDE.md — gui/

Single Go binary plus an embedded Svelte frontend: the browser-facing host for
vision-processor. Owns field geometry, absorbs calibrations from vision
processor instances over multicast, and serves the frontend, a JSON API, a
WebSocket, and debug snapshot images all on one port.

Replaces `python/geom_publisher.py` for anyone running it. Does not replace it
in `setup.sh` or for `python/dataset.py` / `overlap_benchmark.py` / `replay.py`,
which still import it directly — do not delete `geom_publisher.py`.

## Commands

All run from `gui/`.

```
make run       # regenerate proto, build frontend, run the Go host on :8085
make test      # frontend check/lint/format + go test -race, all packages
make install   # go install after a frontend build
make proto     # regenerate internal/{vision,gamecontroller} and frontend/src/proto
make clean     # wipe dist/, internal/{vision,gamecontroller}, frontend/src/proto
```

`proto` (and by extension `run`/`test`/`install`, which all depend on it)
needs `git submodule update --init` to have been run at least once, but no
network beyond that: `buf generate`'s two plugins are both local, not
buf.build remotes. Not using `nix develop`? You need `buf` itself, plus:
`go get -tool google.golang.org/protobuf/cmd/protoc-gen-go` (records the
version in `go.mod`; already a direct dependency so this needs no extra
network fetch) and `npm ci` in `frontend/` (installs `@bufbuild/protoc-gen-es`
from `package.json`).

```
cd frontend && npm run dev   # Vite HMR on :5173, proxies /api and /ws to :8085
```

## Repository layout

```
cmd/ssl-vision-processor-gui/   entry point: flags, wiring, HTTP server
internal/
  geometry/    field config + calibration merge + 1Hz publish loop
  multicast/   bridge to the SSL vision multicast group
  hub/         topic pub/sub + the /ws handler
  snapshot/    debug image listing/serving
  logging/     slog setup: tint console + lumberjack file
  vision/      generated Go protobuf, not committed -- run `make proto` (DO NOT EDIT)
  gamecontroller/  generated Go protobuf, not committed
frontend/      Svelte 5 + TypeScript + Vite, embedded via //go:embed
               (src/proto/ is generated, not committed)
buf.gen.yaml   generates both internal/{vision,gamecontroller} and
               frontend/src/proto from ../proto/proto (the submodule, read
               directly -- no vendored copy)
```

## Stack

- **Go 1.22+ `net/http`**, no router library. Method-prefixed patterns
  (`"GET /api/health"`) and `{wildcard}` path values do the routing;
  `r.PathValue(...)` reads them.
- **`gorilla/websocket`** for `/ws`. Not stdlib; there is no other serious
  option (`golang.org/x/net/websocket` is legacy/incomplete).
- **`ssl-go-tools/pkg/sslnet`** for multicast (`MulticastServer`, `UdpClient`)
  rather than hand-rolled sockets, to match how the rest of the league already
  handles multi-interface venue boxes.
- **`google.golang.org/protobuf` + `protojson`**, never `encoding/json` on a
  generated message (see Gotchas).
- **Svelte 5 runes only**, no stores added beyond what predates this work
  (`wrapper-bus.ts` still uses `svelte/store`'s `readable` -- not yet migrated,
  not a blocker).

## Architectural decisions

**Single Go binary, not Go+Python.** The original plan was "Go host, keep the
Python wrapper for geometry." That changed once it was clear the Python
service (`wrapper_backend/geometry.py`, since deleted) did bookkeeping --
merge calibrations, lay out field markings from YAML, republish at 1Hz -- and
no calibration math. All calibration math stays in C++ (`src/calib/`). The
bookkeeping is now `internal/geometry`, ported directly; `wrapper_backend/`
is gone.

**JSON to the browser, protojson specifically -- never raw protobuf bytes,
never `encoding/json` on a proto message.** `encoding/json` on a generated
struct compiles and looks like it works, then renders enums as integers,
mishandles `oneof`, and ignores the well-known types. `protojson.Marshal` is
what every other league tool's JSON output already agrees on. The one
exception is the multicast wire itself (`Geometry.Encoded()`), which stays raw
protobuf bytes because that's the SSL protocol.

**`Geometry` is mutex-guarded, with a cached encoding, not a "reads don't need
the lock" type.** `proto.Marshal` writes to the message's internal size cache,
so even a read-only marshal is a mutation. `Absorb` re-marshals and caches the
result; `Encoded()` and the WS hub publish just read the cached bytes.
`Snapshot()` returns `proto.Clone` for anyone who needs the actual message.

**Incoming calibrations are validated with `proto.CheckInitialized` before
they touch stored state.** A calibration missing a required field (proto2, a
dozen required floats) is logged and dropped, not merged -- merge-then-fail-to-
encode would leave `Geometry`'s live message out of sync with its last-good
`Encoded()` bytes.

**The hub's `Subscribe`/`Publish` uses size-1, drop-stale channels** --
deliberately mirroring the semantics of the retired Python `bus.py` (`Queue`
of size 1, drain-then-put). A slow WebSocket client sees only the latest
value, never a growing backlog.

**`unsubscribe` closes the per-topic channel.** That's what lets a
per-connection forwarding goroutine's `for data := range ch` exit cleanly when
one topic is unsubscribed without tearing down the whole connection.

**Snapshot serving assumes the Go host and the vision processor share a
filesystem.** This is unchanged from the Python `snapshot.py` it replaces and
is a known limitation, not an oversight -- it does not work once vision
processors run on other hosts. See "Not yet built."

**`internal/geometry.WriteLineCorners` edits vision_processor's per-instance
`config.yml` as text, not as decoded/re-encoded YAML.** That file ships full
of comments and commented-out example values meant to be hand-read; a
decode/re-encode round trip through `gopkg.in/yaml.v3` (confirmed while
building this) drops comments elsewhere in the document and drifts
indentation. A line-based splice around the `line_corners:` key touches
nothing else. This is a stand-in for the not-yet-built `internal/config`
(single, same-host instance, matching `-imgDir`'s assumption) -- see "Not yet
built" -- kept in `internal/geometry` for now since the corner picker needed
it working immediately; move it if/when `internal/config` exists.

**buf reads the proto submodule directly**: `inputs: [{directory:
../proto/proto}]` in `buf.gen.yaml`. No vendored copy to drift. Generated
output (`internal/vision`, `internal/gamecontroller`, `frontend/src/proto`) is
*not* committed -- `buf.gen.yaml`'s plugins are both `local:` (`go tool
protoc-gen-go`; `frontend/node_modules/.bin/protoc-gen-es`), not buf.build
remotes, so `make proto` needs no network beyond the already-checked-out
`proto/` submodule and runs as an ordinary Makefile prerequisite (a `.proto`
sentinel, same technique as `.frontend` below) rather than a separate manual
step. This used to be committed when the plugins were remote (buf.build
codegen calls, real network dependency); switching to local plugins removed
the reason to.

`gui/frontend/dist/.gitkeep` exists for a similar but distinct reason on the
frontend side -- `//go:embed` fails to compile if `dist/` doesn't exist, so a
fresh clone with no `npm run build` yet must still be able to `go test`. That
one stays committed: `dist/` is the frontend's actual build *output*, not
codegen from a source of truth already in the tree, so there's nothing to
regenerate it from without first running the build it's the output of.

**Discovery is multicast-announce-based, config is host-owned** -- both
still unimplemented; see below.

## Gotchas

- **proto2, not proto3.** Every field on the SSL messages is a pointer
  (`*string`, `*float32`, ...) and most are `required`. Read through the
  generated `Get*()` methods (nil-safe); write through `proto.String(...)`,
  `proto.Float32(...)`, or a literal struct with pointer fields. A `required`
  field left unset makes `proto.Marshal` return an error -- this is relied on
  deliberately (see `CheckInitialized` above), not worked around.
- **`go test ./...` from `gui/` will try to compile stray Go source inside
  `frontend/node_modules`** (at least one npm package ships a `.go` file).
  Use `go test ./cmd/... ./internal/... ./frontend`, which is what
  `Makefile`'s `GO_PACKAGES` and CI already do. Don't `go test ./...` by hand.
- **`GOTOOLCHAIN=local` is set in the root `flake.nix`.** Without it, Go
  silently fetches whatever toolchain a dependency's `go.mod` names over the
  network the moment it's newer than what's pinned, defeating the pin inside a
  sandboxed build.
- **`ReadHeaderTimeout` is set on the `http.Server`; `WriteTimeout` is not,**
  and must not be. `WriteTimeout` is an absolute deadline on the whole
  response and would kill every `/ws` connection (and any future video
  stream) after N seconds.
- **`ServeMux` route precedence is by specificity, not registration order.**
  `/api/` is registered as a catch-all `NotFoundHandler` specifically so an
  unrouted `/api/*` path 404s instead of falling through to the SPA handler
  and returning HTML with status 200 -- which would otherwise break every
  `fetch(...).json()` call against a typo'd endpoint.
- **The WS hub pings every ~54s and sets read/write deadlines** so a client
  that vanishes without a clean close (dead wifi, a closed laptop lid) is
  detected and its goroutines cleaned up, rather than leaking for the life of
  the process.
- **`internal/snapshot` validates `camID`/`view` path segments against a
  strict character set before they reach `filepath.Glob`.** An unvalidated
  `view` of `*` would glob the entire snapshot directory; this is enforced and
  tested (`TestHandleGetRejectsUnsafeSegments`), not just documented.
- **`unsubscribe` in `internal/hub` must be called exactly once.** It closes
  the channel; a second call panics on double-close, same as any Go channel.

## Testing

`go test -race` on every package, always -- this code is concurrent by
construction (a mutex-guarded `Geometry`, a multi-goroutine WS hub) and the
race detector has caught real bugs here before it shipped. Integration-style
tests are preferred over mocks where the real thing is cheap to stand up:
`internal/hub`'s WebSocket tests dial a real `httptest.Server` with a real
`gorilla/websocket` client rather than faking the protocol; `internal/geometry`
and `cmd/.../server_test.go` load real `testdata/*.yml` fixtures rather than
constructing Go structs by hand.

The full loop (this package's `Geometry`/`multicast.Bridge` against a real
`vision_processor` binary and a real camera) has been manually verified once,
end to end: the Go host published a field template, `vision_processor`
calibrated against a real skewed webcam view, and the calibration came back
and was absorbed with no wire-format surprises between C++ and Go protobuf.
Not automated -- there is no CI hardware to run it on.

## Not yet built

- **`internal/discovery`** -- parsing `SSL_VPConfig` announces off multicast
  into an instance table. The C++ side does not emit these yet.
- **`internal/config`** -- the host-owned config store and the diff-based
  reprovision loop (push desired config to an instance when its announced
  config doesn't match, rather than trying to detect death). Blocked on
  discovery above and on a C++-side HTTP config-apply endpoint (another
  contributor's work, not started).
- **Bootstrap identity.** `SSL_VPConfig.instance` is `required`, but the plan
  drops VP-local config files entirely. Nothing yet decides what a freshly
  started, unconfigured VP calls itself before it has ever been provisioned.
- **Announce contents/cadence** -- whether the announce echoes the VP's full
  current `SSL_VPConfig` (needed for diff-based reprovision to work at all) or
  just identity, and how often.
- **The imagery/video path for remote instances.** `internal/snapshot`'s
  local-filesystem assumption is a known, explicitly accepted limitation, not
  a solution. `src/rtpstreamer.cpp` already emits H.264 RTP with
  per-instance `stream_ip`/`stream_port`; bridging that to WebRTC/WHEP in Go is
  the likely direction but is undecided and unbuilt.
