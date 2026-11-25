# MVP review: ssl-vision-processor GUI host

Baseline `09cf1ee` (API Middleware Tests and Cleanup #40) → `267bae0` (`v2.0-alpha`).
The whole `gui/` tree is reviewed, with extra depth on everything new or changed since the baseline.

## Scope

In: `gui/` (Go host, Svelte frontend, OpenAPI, packaging, scripts), `.github/workflows`,
root `README.md` / `documentation.md` / `config/`, `flake.nix`, `scripts/`, `setup.sh`,
and alignment with league repos (ssl-game-controller, ssl-go-tools).

Out:
- `src/` C++, the VP↔host wire protocol, `proto/`. Findings that would need a C++ change are parked (see below).
- Security posture. Deployments are airgapped and security needs its own plan.
  Security-shaped correctness or data-loss bugs are still reported, as correctness.

## Severity

This is the first release, so first impressions and deployment quality count as ship criteria.

- **blocker**: must fix before MVP. Includes: a new user can't install, start, or finish first-run setup from the docs alone; wrong or misleading docs or UI text on the main path; a confusing or silent failure on a common error path; data loss; a broken or unreproducible release artifact.
- **major**: should fix before MVP. Inconsistency a user will notice, a rough edge on a secondary path, a test gap on critical logic, an architecture problem that will cost more once others contribute.
- **minor**: fix before MVP if it's cheap; otherwise next release.
- **nit**: style or polish.

Owner per finding: **Will** (Go/Svelte) or **Claude** (build/CI/packaging/docs infra).

## Status

Updated 2026-11-25. **Open** 28 · **Partial** 4 · **Done** 37 · **v2.1** 1 · **Dropped/merged** 3. Each finding below keeps its own *Progress* or *Done* note; this table is the index. Statuses: open, partial, done, v2.1 (deferred to that release), dropped, merged.

**Next up** (MVP gate order; see the end of this file):
All blockers are fixed. T3-3, T5-6, T8-1 and T1-2 are awaiting commit.
1. API freeze: T3-1 with T4-6, then T3-2 (Will approved `openapi-fetch`).
2. T1-3, T6-1, T6-5, T4-3.

**Needs the lab PoE machine:** T2-10.

| ID | Sev | Owner | Item | Status |
|---|---|---|---|---|
| T1-1 | blocker | Claude (Will commits) | 18 MB binary tracked in git | done |
| T1-2 | major | Claude | release tag not gated on `make check` / e2e | done |
| T1-3 | major | Claude + Will | package description promises a VP package that doesn't exist | open |
| T1-4 | minor | Claude | tarball and `-bin` amd64 only (absorbs L-3) | open |
| T1-5 | minor | Claude | stale toolchain and dependencies | partial |
| T1-6 | minor | Claude | CI skips the `v2.0-alpha` branch | open |
| T1-7 | minor | Claude | no Go linter beyond vet | done |
| T1-8 | minor | Claude | `setup.sh` still offers `geom_publisher` | open |
| T1-9 | nit | Claude | legacy `.gitlab-ci.yml` | done |
| T1-10 | nit | Claude | unpinned nix actions | done |
| T1-11 | nit | Will | `DEB_MAINTAINER` personal address | open |
| T2-1 | major | Will | HTTP bind happens last; "UI is available" logged early | done |
| T2-2 | minor | Will | publish tick is the host's only clock | done |
| T2-3 | minor | Will | `Store.apply` not atomic | done |
| T2-4 | minor | Will | `main.go` too big | open |
| T2-5 | minor | Will | CLI help | open |
| T2-6 | minor | Will | log keys and messages | open |
| T2-7 | nit | Will | staticcheck hits | done |
| T2-8 | major | Will | one vision_processor's video restarts are reported as two senders | done |
| T2-9 | major | Will | duplicate video packets (several interfaces) break the picture, no warning | done |
| T2-10 | major | Will + Claude | console warnings while vision/GC use the PoE camera NICs | open (needs lab machine) |
| T3-1 | major | Will | WS topic contract in three places | open |
| T3-2 | major | Will | generated `paths` unused | open |
| T3-3 | major | Will | naming pass before freezing API 1.0 | done |
| T3-4 | minor | Will | response-spec test coverage | done |
| T3-5 | nit | Will | OpenAPI tag descriptions | done |
| T4-1 | blocker | Will | "Temporary debug utilities" on every page | done |
| T4-2 | major | Will | edits lost while host unreachable | done |
| T4-3 | minor | Will | rejected value stays in the form | open |
| T4-4 | minor | Will | no stale-frontend detection | done |
| T4-5 | minor | Will | large components | open |
| T4-6 | minor | Will | bus still on `svelte/store` | open |
| T4-7 | nit | Will | hex colours | partial |
| T4-8 | nit | Claude | Flowbite `@source` CSS bloat | dropped |
| T5-1 | blocker | Will | placeholder Debug page | done |
| T5-2 | major | Will | crash-looping VP raises no alert | done |
| T5-3 | major | Will | SPINNAKER default driver | done |
| T5-4 | major | Will | 4 warnings on a fresh install | done |
| T5-5 | major | Will | accessibility (labels, contrast) | v2.1 |
| T5-6 | major | Will | two navigation systems | done |
| T5-7 | minor | Will | first-run wording | open |
| T5-8 | minor | Will | broken image in corner picker | done |
| T5-9 | minor | Will | Camera page edits undeliverable settings | open |
| T5-10 | — | — | phone widths | dropped |
| T5-11 | nit | Will | Gamma toggle label | done |
| T6-1 | major | Claude (Will reviews) | `ARCHITECTURE.md` drift | open |
| T6-2 | minor | Claude (Will decides) | `CLAUDE.md` vs `ARCHITECTURE.md` split | open |
| T6-3 | minor | Claude | frontend README describes old tabs | done |
| T6-4 | minor | Claude | README relative links break when shipped | done |
| T6-5 | major | Claude + Will | quick start stops at first save | open |
| T6-6 | minor | Claude | root README pointers | done |
| T6-7 | nit | Claude | README details | partial |
| T7-1 | major | Will | no component tests | open |
| T7-2 | major | Will | untested logic modules | partial |
| T7-3 | major | Will | flowbite `Input` unmount error (`test.fail`) | done |
| T7-4 | major | Will + Claude | e2e scenario gaps, axe | open |
| T7-5 | minor | Claude | frontend coverage tooling | done |
| T7-6 | nit | Claude | coverage for binary-only paths | done |
| T7-7 | nit | Will | `time.Sleep` in Go tests | done |
| T7-8 | minor | Will | `yuvInGamut` rounding tolerance | done |
| T8-1 | major | Will | four product names | done |
| T8-2 | minor | Will + Claude | page names | open |
| T8-3 | minor | Will | identifier naming across layers | open |
| T8-4 | minor | Will | text centralization | open |
| T8-5 | nit | Will | `--` as em dash | open |
| L-1 | major | Will | port 8085 collides with remote-control | done |
| L-2 | minor | Will | league-style address flags | open |
| L-3 | — | — | arm64 assets | merged (T1-4) |
| L-4 | minor | Claude | no dependency bot | open |
| L-5 | major | Will | duplicate GC flaps referee state | done |
| L-6 | nit | Claude | `CLAUDE.md` sslnet note | done |
| L-7 | nit | Will | `-verbose` alias | done |
| L-8 | nit | Claude | docker image | open |

Upstream proposals (U-1..U-5) are tracked in "League alignment", not here.

## Plan

Phase 0, automated gates: `make check`, `make e2e`, `make release-check` from a clean tree;
staticcheck, govulncheck, svelte-check, npm audit, unused deps/exports;
Go and Vitest coverage per module; bundle size; Lighthouse a11y/perf.

Tracks:
1. Repo hygiene / release: tracked artifacts, CI vs `make check` parity, release workflow, `.deb`, nix/non-nix independence, Makefile.
2. Go architecture: package graph, `main.go` wiring, lifecycle/shutdown, concurrency, error and log conventions, supervisor, config store invariants.
3. API / OpenAPI contract: route conformance, error codes, naming/verb consistency, WS topic shapes, generated TS use.
4. Frontend architecture: state modules, runes and `$effect` hygiene, oversized components, Flowbite/token rules, fetch/WS failure paths, canvas performance.
5. New-user UX (hands-on): first run → wizard → checklist; `-runCameras` webcam lab; competition mode; failure paths; save/load conflict; narrow screens, dark mode, keyboard, a11y; wording.
6. Documentation: every claim and command checked against the code; user vs contributor split; can a new user install from the `.deb` with docs alone.
7. Tests: test-type × module matrix, gaps, flakiness, stale `test.fail`, CI coverage, component tests, e2e vs track 5 scenarios.
8. Cross-cutting consistency: naming, terminology across Go/TS/UI/docs, message style, config keys, text centralization ahead of i18n.
9. League alignment: compare with ssl-game-controller and current ssl-go-tools master. What can we now use from upstream, and what of ours should go upstream? Every upstream candidate must be additive or non-breaking for existing ssl-go-tools users; anything else is listed as "keep local".

Order: 0 → 1, 3, 7 → 2, 4 → 5 → 6, 8. Track 9 runs alongside.

## Phase 0 results

Run 2026-11-11 on `267bae0` (branch `dev/will/code_review_2`), inside the nix shell (go 1.26.7, node 22.23).

| Gate | Result |
|---|---|
| `make check` | pass, 26 s. e2e: 13/13. `smoke.spec.ts:139` "leaving a page right after an edit" is a `test.fail` known bug, still failing (track 7) |
| loopback multicast tests | **skipped locally** (lo multicast off); they only run in CI |
| `make release-check` | pass |
| `svelte-check` / `tsc` (`npm run check`) | 0 errors, 0 warnings. A bare `npx svelte-check` warns about tsconfig project references; harmless |
| eslint, prettier | clean |
| staticcheck | 2 hits: `internal/config/store_test.go:506` (SA4006 unused `rev`), `internal/logging/logging.go:83` (SA1019 deprecated `tint.NewHandler`) |
| govulncheck | 10 reachable stdlib vulns fixed by go **1.26.11**; `golang.org/x/net` 0.55 → 0.60. Treated as release hygiene: bump before tagging |
| npm audit | 23 (5 high, 12 moderate, 6 low). Almost all in dev/build tooling (vite, eslint, undici, minimatch). Runtime: `svelte` (SSR-only advisories, we're CSR), `flowbite` via `flowbite-datepicker`. `npm update` within ranges clears most |
| Vitest coverage | not measurable: `@vitest/coverage-v8` not installed (track 7) |
| Bundle | one 678 kB JS chunk (192 kB gz), **294 kB CSS** (40 kB gz). Fine on a LAN; CSS size points at an unpurged Flowbite/Tailwind source scan (track 4) |

Go coverage by package (statements):

| ≥90% | 70–90% | <70% |
|---|---|---|
| alerts 99, detections 100, origins 99, v4l 97, livefield 96, peers 95, supervisor 93, referee 93, hub 90 | geometry 89, config 89, multicast 86 (more with lo multicast on), discovery 85, snapshot 84, logging 78, buildinfo 73 | **cmd 63**, api 67, video 69, defaults 60, httpjson 0 (no tests), api/gen 0 |

Lighthouse a11y/perf is deferred to track 5 (needs a running host).

## Findings

Format: `ID | severity | owner | location` then the issue, the evidence, and the fix.

### Track 1: repo hygiene / release

Overall: strong. Packaging (nfpm, debconf, sysusers, upgrade semantics, lintian, `deb-check.sh` on a real systemd runner), the static release build, `release-check.sh`, and the nix/apt CI split all hold up. The findings are about what ships and when.

**T1-1 | blocker | Claude (Will commits) | `gui/ssl-vision-processor-gui`**
An 18 MB host binary is tracked in git, added in `97a0669` ("rename binary, initial debian bundle"), even though `.gitignore:47` ignores it, because it was added before the ignore. It is on `v2.0-alpha` but **not** yet in `origin/master`.
Fix: `git rm --cached gui/ssl-vision-processor-gui`. Before `v2.0-alpha` merges to master, drop the blob from the branch: squash-merge, or rewrite `97a0669` on the branch, so master's clone never carries it.
*Progress 2026-11-17 (Claude):* history rewritten locally with `git filter-branch --index-filter` over `97a0669^..` minus `origin/master`, so master's signed commits keep their hashes. Rewritten: `v2.0-alpha` 267bae0→2f14289, `dev/will/code_review_2` 1d081ba→0395479, `dev/will/fixes_from_lab_testing_11102026` fd3f377→9e434d1, `dev/will/distribution` 97a0669→347f9d7. Verified: each tip differs from its backup only by the binary; no branch contains the blob; same commits, metadata and merge base (`5f75bea`); `make check` passes. Backups: tags `backup/*` and `refs/original`. 
*Done 2026-11-17 (Claude, Will approved):* force-pushed `v2.0-alpha` (2f14289, PR #37 updated, mergeable), `dev/will/code_review_2` (0395479) and `dev/will/fixes_from_lab_testing_11102026` (9e434d1). No remote branch contains `97a0669`. Follow-up: other clones reset or rebase onto the new tips; after about a week, delete the `backup/*` tags and `refs/original` and gc.

**T1-2 | major | Claude | `.github/workflows/release.yml`**
A `v*` tag publishes a public GitHub Release after only `release-check` and `deb-check`. `make check` and the e2e suite never run against the tagged commit; CI runs on master pushes and PRs, but a tag can point anywhere. `gh release create` also publishes immediately, with no draft.
Fix: run `make check` (including e2e) in the release job before building, or gate it on the CI workflow for that SHA. Create the release as `--draft` and publish by hand after a smoke install. Mark `-rc`/`-alpha` tags `--prerelease`.

*Done 2026-11-25 (Claude, at Will's request; uncommitted):* the release workflow runs `make check` on the tagged commit (failing without Chrome, so the browser tests can't be skipped), then drafts the release for Will to publish by hand. Tags with a suffix are pre-releases. Not run: it needs a tag push.

**T1-3 | major | Claude + Will | `gui/packaging/nfpm.yaml:26`, release notes, `gui/README.md`**
The package description says "The camera processing itself, vision_processor, is packaged separately", but no vision_processor package exists yet (the CV deb is planned). A new user who installs the `.deb` from the first release has a GUI with nothing to talk to, and nothing on the release page says how to get vision_processor. The `v*` tag also reads as a release of the whole repo while it ships only the GUI.
Fix: until the CV deb ships, change the description to "build vision_processor from source: see <link>". Give the GitHub Release a body template (`--notes-file`) saying what's included and pointing at the vision_processor build steps. Decide whether GUI releases get their own tag prefix once the CV deb exists.

**T1-4 | minor | Claude | `gui/Makefile:63-75`**
The tarball and `-bin` are built only for the host's `GOARCH` (amd64 in CI), while the debs are amd64 and arm64. arm64 users (Pi, Jetson) get the deb but no standalone binary, though `gui/README.md` presents the standalone binary as the first install path.
Fix: loop `release` over `DEB_ARCHES` (or a `RELEASE_ARCHES`), and include both in `SHA256SUMS`.

**T1-5 | minor | Claude | `flake.lock`, `gui/go.mod`, `gui/frontend/package-lock.json`**
Stale toolchain and dependencies at release time: go 1.26.7 (10 reachable stdlib advisories fixed by 1.26.11), `golang.org/x/net` 0.55 (0.60), 23 `npm audit` entries, most cleared by in-range `npm update`.
Fix: bump all three before tagging. Add `govulncheck` as a non-blocking CI step, or to a pre-release checklist.

*Progress 2026-11-18 (Claude):* Go modules bumped within their majors (`x/net` 0.55→0.63, `x/sys`, `tint`, `mediacommon`, `pion/rtp`); `npm update` within ranges took `npm audit` from 16 findings to 0 (lockfile only). `make vulncheck` (govulncheck, pinned in `go.mod`) runs in CI as a non-blocking step. **Left for Will:** `flake.lock`. `nix flake update` brings Go 1.27.3, but also protobuf 35→36 and yaml-cpp 0.8→0.9 for vision_processor's C++ build, so it was reverted. govulncheck still reports 10 stdlib advisories that only a newer Go clears.

**T1-6 | minor | Claude | `.github/workflows/ci.yml:3-6`**
CI runs on `push: [master]` and PRs only. `v2.0-alpha`, the integration branch the MVP ships from, is tested only when a PR targets it; direct pushes and merges into it (e.g. `443d12a`, `fd10e0b`) are never built.
Fix: add `v2.0-alpha` (or `v*-alpha`) to `push.branches` until it merges.

**T1-7 | minor | Claude | `gui/Makefile`, CI**
No Go linter beyond `vet`. staticcheck finds two things today (`internal/config/store_test.go:506` dead assignment; `internal/logging/logging.go:83` deprecated `tint.NewHandler`).
Fix: `go get -tool honnef.co/go/tools/cmd/staticcheck`, run it in `make check` / the `gui-go` job. Will fixes the two hits.

*Done 2026-11-18 (Claude, at Will's request):* staticcheck is a Go tool pinned in `gui/go.mod` (`go tool staticcheck`), run by `make lint`, which `make check` and the `gui-go` CI job include. Clean today.

**T1-8 | minor | Claude | `setup.sh:126-131`**
The C++ setup script still offers a `geom_publisher` user service and never mentions the GUI. A user who answers `y` and later runs the GUI has two geometry publishers. The host raises a `sender_conflict` error for that (`internal/alerts/alerts.go:107`), so it's caught, but it's an avoidable first-day trap. Also a typo: "installationa".
Fix: drop the prompt, or print "the GUI (gui/README.md) replaces this; say no unless you aren't using it", with default `N` (already the default).

**T1-9 | nit | Claude | `.gitlab-ci.yml`**
A legacy C++-only build on `ubuntu:latest`, duplicating `build-apt` in GitHub CI and pinned to nothing. Remove it, or note who still uses it.

*Done 2026-11-18 (Claude, at Will's request):* `.gitlab-ci.yml` and its `.gitignore` exception are deleted. `build-apt` in GitHub CI covers the same build.

**T1-10 | nit | Claude | `.github/workflows/*.yml`**
`DeterminateSystems/nix-installer-action@main` and `magic-nix-cache-action@main` are unpinned, so the release build's toolchain setup can change under a tag. Pin to a release tag.

*Done 2026-11-18 (Claude, at Will's request):* pinned to `nix-installer-action@v23` and `magic-nix-cache-action@v15`, the latest releases, in both workflows.

**T1-11 | nit | Will | `gui/Makefile:84`**
`DEB_MAINTAINER` defaults to a personal address. Decide whether the league package should name a team or committee alias.

### Track 2: Go architecture

Overall: sound. The import graph is acyclic and layered, with `cmd` doing the wiring and `alerts` as the only aggregator. `host`/`newHost` separates wiring from running, which is what makes the cmd tests possible. The store's revision and stale-write protocol, the supervisor's crash backoff, and context-driven shutdown are all well thought out. The findings are about startup order, the single 1 Hz tick, and log hygiene.

**T2-1 | major | Will | `cmd/ssl-vision-processor-gui/main.go:182-243`**
The HTTP listener is bound last, inside a goroutine, after the multicast loops, the geometry publisher and (with `-runCameras`) the supervisor have started. "UI is available" is logged before the bind is known to have worked. Reproduced with the port already in use:
```
INF main.go:243 UI is available url=http://127.0.0.1:28123
ERR main.go:233 http server init failed err="listen tcp 127.0.0.1:28123: bind: address already in use"
INF initiating graceful shutdown
INF multicast/endpoint.go:291 socket open endpoint=vision ...   (opened during shutdown)
```
A new user sees a URL that doesn't work. Under the system service (`Restart=on-failure`, 5 s) the host also starts and kills every supervised vision_processor in a loop. Given L-1 (port collision with remote-control), this is a likely first-day failure.
Fix: `net.Listen` synchronously right after `newHost` and before any `runBackground`, then `srv.Serve(ln)`. On `EADDRINUSE`, log a plain-language error naming the port and `-address` (and `/etc/default/ssl-vision-processor-gui` for the service), then exit 1 without starting anything.

*Done 2026-11-18 (Claude, at Will's request):* `listenUI` binds the port right after `newHost`, before any background task, and `srv.Serve(ln)` serves it. A taken port logs "can't open the GUI on <addr>: another program is using that port, … choose another port with -address (for the system service: ADDRESS in /etc/default/ssl-vision-processor-gui)" and exits 1 with no socket opened or vision_processor started. Reproduced against a taken port. New tests: `TestListenUIExplainsATakenPort`, `TestListenUIBindsAFreePort`.

**T2-2 | minor | Will | `main.go:187-207`**
The geometry publish tick is the host's only clock. Every second it also checks for suspend, re-lists interfaces and reapplies the sockets, refreshes the heights file, and publishes config state, status, alerts and processes. That ties unrelated work to `Geometry.Run`'s lifetime and makes a slow step (interface listing, heights-file stat) delay geometry publishing.
Fix: a separate `housekeeping` loop (`runBackground`) for socket/suspend/status work, leaving the geometry tick to send and publish geometry only.

*Done 2026-11-18 (Claude, at Will's request):* the geometry tick only sends and publishes geometry. Suspend checks, socket and interface re-evaluation, the heights file and the config/status/process topics run on their own 1 s `everyTick` loop (`housekeeping loop`). Endpoint methods lock, so the two loops can share the sockets. Tests: `TestEveryTick*`.

**T2-3 | minor | Will | `internal/config/store.go:262-274`, `:484-491`**
`apply` is not atomic. It pushes the field (`ApplyConfig`), then the locked calibrations (`SetLocked`), then renders every camera's `config.yml`. A failure after the first step returns an error with live geometry (and, in `update`, the `Unlock` loop) already changed but `working` not updated. `loadLocked` restores `s.path` on failure but not the live state. Unlikely in practice, since the inputs are validated first, but the failure leaves the published geometry disagreeing with the document the GUI shows.
Fix: build everything first (models, locked map, renders into memory), then commit in one step; or on error re-apply the previous `working`.

*Done 2026-11-18 (Claude, at Will's request):* `apply` builds the models and every locked calibration (`liveState`) before touching the live system, so a bad calibration changes nothing. If `SetLocked` (or `update`'s unlock loop) fails after `ApplyConfig` succeeded, `restoreLive` re-applies the working document. The unlock loop's `Unlock` also clears absorbed calibrations, which a rollback can't bring back; the camera re-sends one. Tests: `TestUpdateFailingPartwayLeavesTheLiveSystemAsItWas`, `TestUpdateWithABrokenCalibrationChangesNothing` (`fakeLive.failLocked`).

**T2-4 | minor | Will | `main.go` (964 lines)**
It mixes flag definitions, path bootstrap, firewall and ufw sniffing, socket management (`networkSockets`), topic publishing and the run loop.
Fix: split by concern (`flags.go`, `bootstrap.go`, `network.go`, `publish.go`) as `paths.go` and `status.go` already are. No behaviour change, but it's the first file a new contributor opens.

**T2-5 | minor | Will | CLI help (`main.go:42-66`)**
`-h` prints every default twice ("default: :8085 (default ":8085")", "Default: Info (default "Info")"), starts with "Usage of bin/release/…/ssl-vision-processor-gui:" and has no one-line description, no URL hint and no pointer to `-readme` or the man page. `-openapi` is a build-time flag listed among the user options.
Fix: a custom `flag.Usage` (one-line description, "open http://localhost:PORT", "see -readme"), drop the hand-written "Default:" where `flag` prints it, and list `-openapi` under a "development" heading.

**T2-6 | minor | Will | log keys and messages across `cmd/`, `internal/`**
The camera is logged as `cam` (14), `camera` (6), `id` (6) and `camera_id` (1). Key case mixes `logFile`/`runDir`/`retryIn` with `camera_id`. Messages mix styles: "Init VP GUI" (jargon, title case) next to "mode", "starting", "loaded config" (bare nouns). Users read these in the log panel and journal, and bug reports grep them.
Fix: one key per concept (`camera`, `err`, `path`, `interface`) in one case, and messages as short lower-case sentences. "Init VP GUI" should be "starting ssl-vision-processor-gui". Consider dropping source `file:line` from Info-level console output, keeping it for Debug and the file log.

**T2-7 | nit | Will | `internal/logging/logging.go:83`, `internal/config/store_test.go:506`**
The staticcheck hits from Phase 0: deprecated `tint.NewHandler` → `tint.NewTextHandler`, and an unused `rev` assignment.
*Progress 2026-11-11 (Claude):* the `store_test.go` dead assignment is fixed. `tint.NewHandler` is application code and remains.

*Done 2026-11-18 (Claude, at Will's request):* `tint.NewHandler` → `tint.NewTextHandler`. staticcheck is now clean and enforced (T1-7).

**T2-8 | major | Will | `internal/peers/watch.go`, video watch in `cmd/.../peers.go`**
Senders are told apart by IP and port. vision_processor's streamer reopens its socket, on a new port, whenever the streamed view changes size, and it cycles views. For 3 s both ports count, so one vision_processor is reported as "several senders claim the same thing". 393 such warnings in Will's log (2026-11-07/08), every one with both senders on the same machine.
Fix: for video, a new port (and RTP SSRC) from the same IP replaces the old one once the old one stops; a conflict needs both to keep sending, interleaved.

*Done 2026-11-25 (Claude, at Will's request; committed `b32e24b`):* the video watch has `Restarts`: a sender that stopped within 100 ms of another starting on its IP is the same program. Two that keep sending are still a conflict, on one machine or two.

**T2-9 | major | Will | `internal/video/assembler.go`, `internal/multicast/socket.go`**
The video socket joins its group on every used interface, so a stream that reaches the machine on two of them arrives twice from the same IP and port. The assembler reads a repeated sequence number as packet loss: with every packet doubled, a synthetic 60-frame stream yields no frames. Nothing warns about it. The double delivery was reproduced in a namespace with a local sender, only with `accept_local=1` and loose or no rp_filter: with the default `accept_local=0` the kernel drops a machine's own packets coming back in. A sender on another machine reaching both interfaces is untested. Not known to be what Will sees in the lab (T2-10).
Fix: drop packets already seen (same SSRC, recent sequence number), and alert when one sender's packets arrive on several interfaces (needs the arrival interface from IP_PKTINFO).

*Done 2026-11-25 (Claude, at Will's request; committed `b32e24b`):* the assembler ignores packets at or up to 1024 behind the newest. Every endpoint (vision, game controller, tracker, video) notes the interface each counted packet arrives on; a sender on several at once is logged once, listed in `Status.duplicates`, and raises `duplicate_packets` (warning, one per interface set, with a how-to). Checked in namespaces: a sender on another machine, with the host's static and link-local interfaces on one switch, was doubled and caught with `rp_filter=2`; with `rp_filter=1` the kernel dropped the copies.

**T2-10 | major | Will + Claude | interface selection; cause unknown, needs the lab machine**
Reported by Will 2026-11-25: on a lab machine with a field network NIC and a dual-port PoE NIC for FLIR cameras, the console prints warnings. They stopped when he unticked the PoE interfaces for the vision/GC network on the Network page. Not video related, though video may be affected too. The PoE ports are always static or link-local, on another subnet than the field network. The warning text isn't recorded and nothing has been reproduced.
Workaround: untick those interfaces on the Network page.
To collect there, with the PoE interfaces ticked: the exact warning lines and which program prints them, `ip -br addr`, `ip route`, `sysctl net.ipv4.conf.all.rp_filter`, and `GET /api/network` and `/api/alerts`. Then the same with them unticked.
Idea, pending the cause: automatic selection skips link-local (169.254/16) interfaces while another is usable. Static PoE ports look like any other NIC, so those would need a hint on the Network page instead.

*2026-11-25:* T2-9's alert may name the cause: check whether it appears there with the PoE interfaces ticked. It won't if the prints come from something other than one sender being heard twice.

### Track 3: API / OpenAPI contract

Overall: the REST half is well disciplined. Every route goes through `api.Route`/`Raw`, every one has an operation summary and tag, `TestMain` in `declared_test.go` fails on any undeclared error code, and the error envelope is uniform. The weak half is the WebSocket contract and the frontend's use of the spec.

**T3-1 | major | Will | `cmd/ssl-vision-processor-gui/docs.go:33-40`, `main.go:591-833`, `frontend/src/lib/wrapper-bus.ts:141`**
The WS topic contract is hand-maintained in three places and has already drifted. The spec's topic table lists 5 topics; the host serves 7 (`instances.state` at `main.go:790` and `live_field.frames` are missing). Topic names are string literals on both sides. Each frontend subscriber asserts its own payload type (`topic<ConfigState>("config.state")`), so a wrong name or type compiles.
Fix: one topic registry in Go (name → payload type → equivalent route) that drives `/api/topics`, the spec's topic table (or an `x-topics` extension with schema refs), and a test that every hub topic is registered. Generate a TS `TopicMap` from it so `topic("config.state")` infers its type and an unknown name fails to compile.

**T3-2 | major | Will | `frontend/src/lib/*.svelte.ts`, `api.ts`**
The frontend uses only `components["schemas"]` from the generated file, never `paths`. There are 30 hand-written `/api/...` URL strings and 12 `response.json()` results cast to a type, so a wrong path, method, query parameter or body shape still type-checks, which is exactly the drift the generated types exist to catch.
Fix: `openapi-fetch` (same author as `openapi-typescript`, ~6 kB, no runtime schema) typed against `paths`. It makes `client.GET("/api/config/cameras/{id}/effective", {params: {path: {id}}})` checked end to end and removes most casts. `requestJSON`/`errorMessage` can stay as its error middleware.

**T3-3 | major | Will | public route and topic names; `docs.go:19`**
`specVersion` is `1.0.0`, and its comment promises that changing or removing anything means `/api/v2`. Once the API ships in a release with docs at `/api/docs`, other teams will script against these names; it's cheap to rename now and costly later. Current inconsistencies:
- case: `/api/live_field` (snake) vs `/api/config/save-as` (kebab)
- number: `/api/camera/devices` vs `/api/config/cameras/{id}`; `/api/snapshot/{camID}/{view}` vs `/api/snapshots`
- path params: `{camID}` vs `{id}`
- topics: `wrapper_packet.out` vs `*.state` vs `live_field.frames`
- operationIds: `getConfig`/`getAlerts` vs bare nouns (`network`, `instances`, `load`, `system`). The snapshot route and both WebSockets have none, so generated clients and Redoc anchors fall back to unstable names.

Fix: pick one convention (kebab-case paths, plural collections, `{id}`, `verbNoun` operationIds, `<noun>.state` topics), apply it before the release, and consider `0.x` for the spec version until after the first event.

*Done 2026-11-25 (Claude, at Will's request; uncommitted):* `/api/live-field`, `/api/capture-devices`, `/api/snapshots/{id}/{view}`; topics `live-field.state` and `geometry.state`; verbNoun operationIds on every route, the WebSockets included; `ErrorBody`; spec version 0.1.0. No aliases. `TestNamesFollowTheRules` keeps new names to the rules.

**T3-4 | minor | Will | `cmd/ssl-vision-processor-gui/responses_test.go:17-33`**
`TestResponsesMatchTheSpec` checks a hand-kept list of 16 GET routes. A new GET route is silently unchecked, and `/api/geometry`, every write route's 200 body and the error envelope are never validated. `conforms` checks nullability, enums and required fields only.
Fix: derive the GET list from the spec and fail on any route not exercised. Add write-route and error-envelope checks. If the custom validator keeps growing, swap it for `kin-openapi`'s `openapi3filter` in tests only.
*Progress 2026-11-11 (Claude):* `TestEveryGETRouteIsChecked` now fails when a GET route is neither checked nor listed as non-JSON, and `/api/geometry` was added to the checked set. Still open: write-route 200 bodies and the error envelope.

*Done 2026-11-18 (Claude, at Will's request):* `TestWriteResponsesMatchTheSpec` runs every write route on one host (PUT, PATCH, lock/unlock, save, save-as, load, reload, restart), including stale, bad-body, missing If-Match, not-found and bad-name errors, and checks each status is in the spec and each body (success or error envelope) matches it. It fails when a write route isn't exercised. `checkResponseFor` generalizes the GET check.

**T3-5 | nit | Will | `cmd/ssl-vision-processor-gui/routes.go`**
Tags (`Status`, `Host-local`, `Configuration`, `Field`, `Live updates`) have no descriptions and no top-level `tags` order, so Redoc orders groups by first use. `/api/live_field` sits under "Field" while its topic sits under "Live updates". Add a top-level `tags` list with one-line descriptions.

*Done 2026-11-18 (Claude, at Will's request):* `api.Info.Tags` sets the spec's top-level `tags`: Status, Configuration, Field, Live updates, Host-local, each with a one-line description (`specTags`, `routes.go`). `/api/live_field` moved to Live updates. `TestEveryTagIsDescribed` fails on a route tag missing from `specTags`.

### Track 4: frontend architecture

Overall: the state modules (`*.svelte.ts`) are a clean, consistent pattern: one owner module per concern, `$effect.root` for module-level persistence, and `localStorage` wrapped in try/catch everywhere. Flowbite and token discipline holds (every raw `<button>` has a stated reason), `check-text.mjs` enforces centralized prose, and the WS bus reconnects with backoff and resubscribes. The findings are about what a user sees and edits that can be lost.

**T4-1 | blocker | Will | `frontend/src/App.svelte:91-146`**
Every page renders a "Temporary debug utilities -- see the note in this file's source" block below the content, for every user, with no expert-mode gate:
- a snapshot grid that re-lists every 5 s and **re-fetches every visible PNG every second** (`cacheBuster = Date.now()` on a 1 s interval)
- a "Subscribe to wrapper_packet.out" button that dumps raw JSON

The source comment calls it a stand-in "predating the two-column shell". It tells a new user the product isn't finished, and the image reloads cost bandwidth on the venue network for every open browser.
Fix: remove it for the MVP. If the snapshot grid or the raw packet is still useful, move it behind expert mode into the log/debug panel, with refresh on demand rather than 1 Hz.
*Dependency 2026-11-18 (Claude):* this grid is the only place the GUI shows `raw`/`flat`/`gradient`/`blob` (the corner picker shows only `geomcalib_input`). After T5-1, Advanced's "Snapshot refresh" note says the Geometry page shows these, which is true only because of this grid. `smoke.spec.ts` "debug snapshots reach the page" also asserts its `cam 0 / raw` caption. Options: (A) delete everything; reword or remove Snapshot refresh; drop the caption assert. (B, recommended) move the grid into a "Camera views" card on the Geometry page with on-demand refresh, Flowbite and tokens; drop the wrapper-packet dump (Live Field covers it); the banner and its hex CSS go too (T4-7). Awaiting Will's choice.
*Done 2026-11-18 (Claude, at Will's request; uncommitted):* Will chose a variant of B. The banner, the wrapper-packet button and the 1 Hz grid are gone from `App.svelte`, and so is Shell's `below` slot. A new `SnapshotViews.svelte` (Flowbite Card, Button and A; text in `text/snapshots.ts`) lists one camera's images when the page shows and on Refresh. There is no polling. Each image links to its full-size file. Placement:
- **Geometry:** "Calibration views" with `pixels`, `lines`, `pixels.corner` and `pixels.refined`.
- **Color:** "Detection views" with `flat` and `blob`.
`raw` and `gradient` aren't shown. The e2e fixture now writes `pixels` and `flat`, and the snapshot test checks each card on its page. `make check` passes. Checked by screenshot, light and dark; axe found nothing new.

**T4-2 | major | Will | `frontend/src/lib/config.svelte.ts:126-171`**
An edit made while the host is unreachable is silently lost. `sendPending` sets `lastSynced = json` *before* `putDoc`. If `fetch` rejects (host restarting, wifi drop), the error escapes as an unhandled rejection from `void sendPending()`, `config.error` is never set, and the edit is never retried. When the WS reconnects, the new `config.state.revision` triggers `loadConfig()`, which replaces the local document with the host's. The user typed a value, saw it in the field, and it's gone with no message.
Fix: set `lastSynced` only after a 2xx. On a network error, set `config.error` ("not saved: host unreachable, will retry") and retry when the connection returns. Hold off `loadConfig()` while a local edit is unsent, or merge it.
*Test status:* `config.test.ts` covers the edit and sync paths (97% statements). This bug is pinned as an `it.todo`, not an expected failure, because it surfaces as an unhandled rejection, which fails the whole vitest run. Turn it into a real test with the fix.

*Done 2026-11-18 (Claude, at Will's request):* `lastSynced` moves only when the host answers. A PUT the host never answers (fetch rejects) shows "Not sent: the host can't be reached…", keeps the edit, and retries every 2 s; `flushEdits` (Save, lock) sends it at once. The `config.state` refetch holds off while a retry is pending, so a reconnect doesn't drop the edit. If the host restarted at another revision, the retry gets a 409 and the usual "edited elsewhere" reload applies. Also fixed: two waiters could both send the same edit after an in-flight PUT (`while (inFlight)`). The `it.todo` is now three tests in `config.test.ts`.

**T4-3 | minor | Will | `config.svelte.ts:170`**
After a 400 (validation), the rejected value stays in the local document while the host keeps the old one, and `lastSynced` now equals the rejected JSON, so nothing reconciles them until the next edit or a reload. The banner shows the reason, but the form shows a value the host isn't using. The "a rejected edit shows the host's reason" e2e test checks the banner, not the field.
Fix: on 400, keep the message and restore the host's document. Better, map `issues` to the field (FormRow already has message slots) so the user sees which value was refused.

**T4-4 | minor | Will | `frontend/src/lib/wrapper-bus.ts`, `config.svelte.ts`**
No stale-frontend detection. After a `.deb` upgrade restarts the service, every open browser keeps the old bundle and talks to the new API. Mismatched shapes then surface as odd errors, not as "reload".
Fix: embed the build version in the page. On WS reconnect, compare it with `/api/health`'s `version`, and show a "the GUI was updated, reload" notice when they differ.

*Done 2026-11-18 (Claude, at Will's request):* `hostVersion.svelte.ts` remembers the version `/api/health` reports when the page first connects (the bundle comes from the same binary, so that's the bundle's) and checks again on every WebSocket reconnect. A different version shows `UpdateNotice.svelte`: "The GUI was updated (old → new). Reload this page…" with a Reload button. No build step needed. `hostVersion.test.ts` (3 tests).

**T4-5 | minor | Will | large components**
`config/YuvPositionPane.svelte` (1047 lines, 288 of them CSS), `config/CameraPanel.svelte` (650), `config/WeightTriangle.svelte` (529), `layout/Shell.svelte` (510).
Fix: pull the coordinate and colour math into `.ts` modules (which also closes most of T7-1) and split the panes into subcomponents.

**T4-6 | minor | Will | `wrapper-bus.ts:141`, `App.svelte:18-43`**
The bus is the last `svelte/store` user (`readable`). Consumers wrap it in `$effect` plus a manual `subscribe`, and App.svelte needs a 10-line comment explaining why a `$store` reference wouldn't have gated the subscription.
Fix: a runes-native `topicState(name)` built on `createSubscriber` (svelte/reactivity), which subscribes while something reads it. Do it together with T3-1's typed `TopicMap`.

**T4-7 | nit | Will | hex colours**
`App.svelte:151-165` hand-styles a warning box in hex, against the token rule (it goes away with T4-1). In canvas/SVG drawing, where hex is allowed, the field green `#0a5c2e` is repeated in three files (`FieldSketch`, `WizardFieldPreview`, `WizardDimensionsPreview`) and the danger stroke `#b00020` in `WeightTriangle`.
Fix: one CSS custom property each in `app.css`.
*Progress 2026-11-18 (Claude):* the `App.svelte` hex box went with T4-1. The canvas hex colours remain.

**T4-8 | nit | Claude | `frontend/src/app.css:129`**
`@source "../node_modules/flowbite-svelte/dist"` generates CSS for every class any Flowbite component uses: 294 kB raw, 40 kB gzipped. Fine on a LAN; noted only so it isn't mistaken for a leak.

*Dropped 2026-11-18:* noted, no action; it's a size note, not a defect.

### Track 5: new-user experience (hands-on)

How it was run: the release binary `267bae0`, each time in an empty scratch home. The bootstrapped `vision.yml` was moved to a random private multicast group with `first_run: true` kept, so nothing could reach the league groups. Driven in installed Chrome via Playwright, with axe-core 4.10 on every page: light and dark at 1440×900, light at 1024×768 and 390×844. Second scenario: `-runCameras` with a stand-in `vision_processor` on `PATH` that prints "could not open camera /dev/video0: Device or resource busy" and exits 1. The chrome-devtools MCP was unavailable, so Lighthouse wasn't run; axe covers the accessibility side.

What works well: the wizard opens by itself on a first run; Division B is one click to a review step; Apply leads straight into a "Save configuration" dialog with a clear "Save and start publishing" button and a yellow "Saving it starts publishing" note. Alerts carry "How to fix" and a link to the relevant page. Dark mode is coherent. The supervisor captured the stand-in's stderr and backed off correctly (`crashes: 4`, state `waiting`).

**T5-1 | blocker | Will | Debug page: `frontend/src/lib/config/ConfigCategoryPlaceholder.svelte`, `lib/text/placeholder.ts`, `lib/layout/configCategories.ts:117`**
The Debug page is a contributor placeholder shipped to users: "Not wired to a backend yet -- there is nowhere to read or write a specific instance's config.yml over the network. See `internal/config` in gui/CLAUDE.md's 'Not yet built'", followed by "Contributing this panel? Replace this file with a real form…" and a static table. Screenshot: `L-debug.png`. Same class as T4-1, which is also visible under every page in these screenshots.
Fix: hide the Debug page for the MVP (or show it only in expert mode, with user-facing wording), and remove the contributor note.
*Decision 2026-11-18 (Will):* drop the Debug page; move its settings into Advanced as shared `defaults.debug` (all cameras). `wait_for_geometry` and `debug_images` become toggles, `debug_stream_interval_ms` a number. `ground_truth` stays a hand-edited key: it's a file name that `ValidName` doesn't check, and only the benchmarks use it. Also: add `debug` to `advancedOverrides`. (An earlier note here warned about disk use from `debug_images`. That was wrong: it saves three PNGs once per start, not per frame. See the 2026-11-18 follow-up.) Delete `ConfigCategoryPlaceholder.svelte`, the `MainContent` fallback, `ConfigFieldList.svelte`, `placeholder.placeholder` and `CONFIG_FIELDS.debug`. Then `CONFIG_FIELDS`, `fields` and `yamlKey` look unused (optional tidy-up).
*Follow-up 2026-11-18 (Claude, from reading `src/`):* what each key does, for choosing which to expose.
- `debug_stream_interval_ms`: rewrites `img/<cam>.raw.jpg` every N ms, plus `flat`, `gradient` and `blob` JPEGs once calibrated. The files are overwritten, so they don't pile up. These are what the snapshot views show; with 0, an uncalibrated camera only saves a single `raw.jpg` at frame 100. This is the useful one.
- `debug_images`: saves `.flat.1.png`, `.gradDot.1.png` and `.blob.1.png` once, on the first detection frame. `CLImage::save` adds `img/`, so they do land in `img/`, but without the `<camera>.` prefix, so the snapshot listing skips them.
- `wait_for_geometry`: blocks startup until any geometry packet arrives. On a first run the host publishes nothing until the first save, so vision_processor hangs before its frame loop and never announces. Unsafe as a GUI toggle without a warning.
- `ground_truth`: benchmark binaries only.
The `configFields.ts` comments are wrong for two of them (`debug_images` "in img/"; `debug_stream_interval_ms` "img/.sample.<camId>.png while uncalibrated"). Moot once `CONFIG_FIELDS.debug` is deleted.
*Done 2026-11-18 (Claude, at Will's request; committed `64af243`):* the Debug page is gone. Advanced has a new "Diagnostics" card with one setting, "Snapshot refresh" (`defaults.debug.debug_stream_interval_ms`). The schema gives it a 0–5000 ms range in steps of 100 (`schema.go`). `setAdvanced` now rounds any `integer` setting, not just `blobs`. `advancedOverrides` now includes `debug`. Deleted: the `debug` category, `ConfigCategoryPlaceholder.svelte`, `ConfigFieldList.svelte`, `placeholder.placeholder`, `CONFIG_FIELDS.debug`, the MainContent fallback, and `0/debug` from the e2e page list. An old `#0/debug` link falls back to the default page. New `advanced.test.ts` (8 tests). `make check` passes. The other three keys are listed under Future features.

**T5-2 | major | Will | `internal/alerts/alerts.go:127-137`, `frontend/src/lib/config/CameraPanel.svelte:560-605`**
A crash-looping supervised vision_processor raises **no alert**. Only `StateConflict` becomes one. With the stand-in failing every start, `/api/alerts` still lists only `socket_silent`, two `team_default_height` and `loopback_multicast_off`. The sidebar shows `cam 0` with no status. The Camera and Stream pages show "Waiting for video on 224.5.23.100:10100. Is the vision_processor running?" even though the host knows it isn't, and why. The crash count and the captured stderr are only in a card at the bottom of the Camera page. This is the most likely failure in the lab setup `-runCameras` exists for (wrong driver, busy or missing `/dev/video0`).
Fix: an `Error` alert per camera when `crashes ≥ 2` (or state `waiting` after a crash), with the last output line as its message and "How to fix" pointing at the Camera page. A state dot on the sidebar camera row. The video placeholder should read process state and say "vision_processor keeps exiting: <last line>" when supervised.

*Done 2026-11-18 (Claude, at Will's request):* `process_crashing` (Error) once a supervised vision_processor fails twice in a row (`crashingAfter`), with the crash count, exit and last non-blank output line; the GUI words it, with a How to fix doc (`process-crashing`: driver, device, something else holding the camera) and a link to the Camera page. The sidebar row of a supervised camera has a status dot (green running, yellow starting, red failing or held, with screen-reader text). The video placeholder says "vision_processor keeps exiting: <last line>", "isn't started: <conflict>" or "is starting" instead of asking whether it runs. Tests: two alert cases in `alerts_test.go`, `processes.test.ts`.

**T5-3 | major | Will | `cmd/ssl-vision-processor-gui/main.go:436-467` (`bootstrapConfig`), wizard**
Every new camera defaults to the **Spinnaker (FLIR)** driver (the schema's `camera.driver` default `SPINNAKER`, vision_processor's own fallback). The planned CV package is FOSS-only (OpenCV/V4L, DC1394) and `-runCameras` targets student labs with webcams, so a new lab user's first `-runCameras` start crash-loops on a driver they don't have, and (T5-2) nothing says so. No C++ change is needed: the host owns the bootstrapped `vision.yml`.
Fix: write `defaults.camera.driver: OPENCV` in a new config, or better, a wizard step "What cameras?" (webcam / FLIR / FireWire / mvIMPACT) that sets it and offers the `/api/camera/devices` picker.

*Done 2026-11-25 (Claude, at Will's request; committed `34b48f2`):* not OPENCV. A new config writes `defaults.camera.driver: DUMMY`, the protocol's "no camera (initial state)" (proto `fdd7988`), so nobody gets a wrong driver. A file without a driver still means SPINNAKER. With `-runCameras` a DUMMY camera is held, not started (state `no_camera`; its restart route answers 409 `no_camera`), starts when a driver is chosen, and never counts as a device conflict. New alert `camera_not_selected` (warning, error on a competition field), except for a camera the host writes no config for that is already sending. The Camera page shows "No camera selected" and hides device, resolution and tuning until one is chosen; sidebar, video placeholder and checklist say so too. Today's vision_processor exits on DUMMY (`openCamera` has no case), so a camera run by hand still needs a driver first. Once upstream PR #61 lands, the hold can go. The wizard step stays a possible later feature. *2026-11-25:* it broke `make release-check` (a new config no longer starts a camera), and so CI's release build, unnoticed because `make check` doesn't run it; fixed with the API rename batch.

**T5-4 | major | Will | alert severities on a fresh install**
Before the user has done anything, a first run shows **4 warnings and 1 caution**: `first_run_not_publishing`, `socket_silent`, `team_default_height` ×2, `loopback_multicast_off`. In a lab without a game controller, the two height warnings never go away. Users learn to ignore the warning count on day one, and that is exactly the count that should matter before a match.
Fix:
- `team_default_height` should be a caution while no GC has been heard (a warning once one has). With `CompetitionField` on, keep it a warning.
- `socket_silent` on the vision group should be info/caution until a vision_processor has ever been heard.
- Check the loopback caution's wording. "vision_processor on this machine can't be heard here" is only true when no other interface carries the multicast; on a machine with a NIC, the local VP is normally heard via `IP_MULTICAST_LOOP` on that interface.

*Done 2026-11-25 (Claude, at Will's request; committed `b32e24b`):* vision silent is a caution until something is heard, except on a competition field or while a vision_processor this host runs is running. The default-height alert is a caution only with nothing set and no game controller heard (still an error on a competition field; a lab can clear it with a height per color). Loopback multicast off is hidden while a network is in use and a warning otherwise. The checklist's Teams step stays to do on that height caution. Automatic interface selection falls back to loopback when nothing else is usable and its multicast is on. The how-to was incomplete: a plain sender like vision_processor also needs a multicast route (`ip route add 224.0.0.0/4 dev lo src 127.0.0.1`), or its sends fail with ENETUNREACH. Checked in a network namespace.

**T5-5 | major | Will | accessibility (axe, every page)**
- **Critical `label`:** unlabeled form controls on Camera (3), Geometry (4) and Color (1), in light and dark, at every width.
- **Serious `color-contrast`:** on every page except Stream. Worst on Color in dark mode (29 nodes) and on Layout/Advanced in dark (10 each). Among the flagged nodes: the tab-bar section headers (`.after:content-['/']`) and the inactive tab text.
- **At ≤1024 px:** `scrollable-region-focusable` on Layout and Advanced.
- **At 390 px:** `skip-link` (moderate).

Fix: label the controls (likely the slider/number pairs; each needs `aria-label` or `aria-labelledby` to its FormRow label), raise the muted grays one step, and add the axe check to e2e (T7-4) so it stays fixed.

*Deferred to v2.1 (Will, 2026-11-18):* handled as one accessibility pass with color-blind safety and related items; see Future features.

**T5-6 | major | Will | `layout/Shell.svelte`, `layout/TabBar.svelte`**
There are two navigation systems with different contents. The sidebar lists Live Field, Teams, Network, Dimensions and Layout, then Camera, Geometry, Color, Stream and Debug. The top tab bar repeats them under "GLOBAL CONFIGS" and "PER-CAMERA CONFIGS - FIELD CAM #0", but has no Stream, Debug, Alerts or Advanced. A new user can't tell which is primary, or why some pages appear in only one.
Fix: pick one. Keep the sidebar (it scales to several cameras) and drop the tab bar, or keep the tabs only on narrow screens as a replacement for the sidebar.

*Done 2026-11-25 (Claude, at Will's request; uncommitted):* Will's choice: the tab bar is the navigation. The sidebar keeps Alerts and the camera list, unchanged; `ConfigNav.svelte` (the page links) is gone and Stream is now a tab. The sidebar's free space is for camera summaries later (Future features).

**T5-7 | minor | Will | first-run wording**
- The wizard's Finish step still says "Applying… doesn't save -- use Save in the settings menu (or Ctrl+S)", although Apply immediately opens the save dialog.
- On a first run, that dialog says "No unsaved changes." right next to "Save and start publishing", which reads as a contradiction.
- The wizard opens on "Field setup / Start from a regulation field" with no one-line welcome (what this tool is, that nothing is published until saved).
- The sidebar's italic developer note ("One row per camera role, not per machine -- a host running all 4 cameras of a quad setup appears here 4 times. From vision.yml's cameras list.") is permanent.
- The header badge "User Interface - OPEN (127.0.0.1:28200)" and the bare "3/11" checklist counter are unexplained.
- ASCII `--` is used as a dash throughout the UI text.

Fix: first-run-aware text in the wizard and save dialog, a welcome line, the sidebar note moved into a tooltip, "Checklist 3/11", typographic dashes.

**T5-8 | minor | Will | Geometry page, `CornerPicker.svelte`**
With no snapshot yet, the corner picker renders a broken-image icon with its alt text ("cam 0 calibration input") and a bare `line_corners:` code block, and the page requests `/api/snapshot/0/geomcalib_input` and logs a 404 in the console on every visit. "Lock current calibration" is styled like an enabled primary button when there's nothing to lock.
Fix: an empty state ("No calibration frame yet. vision_processor saves one when it starts calibrating"), fetch only what `/api/snapshots` lists, and a plainly disabled lock button with a reason.

*Done 2026-11-18 (Claude, at Will's request):* the corner picker lists `/api/snapshots` first (and on Refresh frame) and only requests `geomcalib_input` when it exists, so no broken image or 404; otherwise it says no calibration frame has been saved yet and that it's local-only. The `line_corners:` block shows only with an image. The Lock button was already disabled without a live calibration; a visible line under it now says why (the tooltip only showed on hover).

**T5-9 | minor | Will | Camera page, external cameras**
Without `-runCameras`, the Camera page edits settings the host can't deliver: "This camera has no `config_path`, so the host doesn't write its config file; copy these to it by hand." This is a known limitation (config push isn't built), but the page still looks like a working form.
Fix: in that mode, show the settings read-only with a "Copy as YAML" button, or link to the docs section on running vision_processor with a host-written config.

**T5-10 | dropped (Will, 2026-11-11: phone support is out of scope)**
At 390 px pages scroll horizontally; not a target. 1024 px is fine.

**T5-11 | nit | Will | Camera page**
The Gamma toggle shows **on** with the label "Off" next to it (`t-camera.png`), unlike Exposure and Gain, where on means "Auto". The Color page's YUV plane keeps a bright light-gray hatched background in dark mode.

*Done 2026-11-18 (Claude, at Will's request):* the Gamma toggle reads "Linear" (on means 1.0, no gamma), so on no longer sits next to "Off". The YUV plane's out-of-gamut checkerboard uses dark shades in dark mode and redraws when the theme changes.

### Track 6: documentation

Overall: `gui/README.md` is a genuinely good user guide: quick start, package questions, file locations, the three Ubuntu network pitfalls, a troubleshooting table, and upgrading. `DEVELOPMENT.md` covers both nix and apt. The man page is test-checked against the flags (`TestManPageListsEveryFlag`). The findings are drift between documents, links that break once the README ships, and the path a new user takes after the quick start.

**T6-1 | major | Claude (Will reviews) | `gui/ARCHITECTURE.md:351-395, 419-429`**
`ARCHITECTURE.md` contradicts the code and `CLAUDE.md`:
- "Not yet built: **Discovery** … The C++ side does not emit these yet." But `internal/discovery`, `GET /api/instances` and the `instances.state` topic exist, and `CLAUDE.md` says the announce ships (upstream PR #50).
- The WebSocket section documents 6 topics and omits `instances.state` (the same drift as T3-1).

Fix: correct both. Then decide which document is canonical for humans (below).

**T6-2 | minor | Claude (Will decides) | `gui/CLAUDE.md` (384 lines) vs `gui/ARCHITECTURE.md` (429)**
The design rationale is split between an agent-facing file and the human architecture doc, and some of it exists only in `CLAUDE.md`: the drop-stale hub channels, why `WriteTimeout` must not be set, `Pdeathsig` and process groups, the sslnet rationale. Human contributors don't read `CLAUDE.md`, and two copies are how T6-1 happened.
Fix: make `ARCHITECTURE.md` the single home for decisions and gotchas, and slim `CLAUDE.md` to commands, rules and links into it.

**T6-3 | minor | Claude | `gui/frontend/README.md:8-18`**
It describes a tab bar with "the shared Overview, Network, Field Dimensions, Camera Layout and Advanced". The real tab bar is Live Field, Teams, Network, Dimensions, Layout | Camera, Geometry, Color. There is no Overview, and Advanced isn't in it. It also defers to "the root README" for the snapshot same-filesystem limitation, but that's documented in `ARCHITECTURE.md`. Fix both.

*Done 2026-11-18 (Claude, at Will's request):* the page and tab lists match `configCategories.ts`, the snapshot limitation points at `ARCHITECTURE.md`'s "Not yet built", the App.svelte bullet describes what's left after T4-1, `SnapshotViews.svelte` is listed, the snapshot JSON field is `cameraId`, and `make run` points at `DEVELOPMENT.md`.

**T6-4 | minor | Claude | `gui/README.md` relative links**
The README ships in the tarball (beside `LICENSE.md`), in the `.deb` (`/usr/share/doc/…/README.md`) and through `-readme`. Its links `../README.md` (lines 3, 228), `DEVELOPMENT.md` (10) and `LICENSE.md` (296) resolve only inside the repo; in the tarball, `../README.md` points outside the folder.
Fix: absolute GitHub URLs for links out of `gui/`, keeping `#anchors` relative. Add a test, like the man page one, that fails on a relative link that leaves the file.

*Done 2026-11-18 (Claude, at Will's request):* the links to the root README and `DEVELOPMENT.md` are absolute GitHub URLs (`blob/master/gui/…`, which resolve once `gui/` is on master). `TestReadmeLinksWorkOutsideTheRepo` fails on any README link that isn't an `#anchor` or a full URL.

**T6-5 | major | Claude + Will | `gui/README.md` quick start, Cameras**
The quick start ends after the first save. A new user is then left with a GUI showing warnings (T5-4) and no cameras, and the guide's next steps are much further down. The webcam steps also document the T5-3 trap as an instruction ("ready to go… once its driver is set to OPENCV on the Camera Settings page"). The troubleshooting table has no row for the commonest local failure: vision_processor keeps restarting (wrong driver, busy device).
Fix:
- Add a quick-start step 5, "Connect cameras", that branches to "one webcam on this machine" (`-runCameras`) or "cameras on other machines", with the vision_processor build link.
- Drop the OPENCV caveat once T5-3 lands.
- Add troubleshooting rows: "vision_processor keeps restarting" (Camera page output, driver, `/dev/video*` busy) and "no video in Firefox/Chromium".

**T6-6 | minor | Claude | root `README.md:19-20`**
- It sends readers who want to "work on it" to `gui/CLAUDE.md` for the `buf` setup; that's `DEVELOPMENT.md`'s job.
- Its frontend line (`make proto && cd frontend && npm install && npm run dev`) bypasses the Makefile's own prerequisites (`npm ci`, generated API types) that `DEVELOPMENT.md` documents. Followed literally, `src/lib/generated/api.ts` is never created, so `npm run check` and the editor's type checking fail. The dev server itself probably runs, since Vite erases the type-only import (not verified).
- Its dependency section sends users to `setup.sh` (T1-8).

Fix: the root README gives one sentence plus links to `gui/README.md` (users) and `gui/DEVELOPMENT.md` (contributors), and nothing else about the GUI.

*Done 2026-11-18 (Claude, at Will's request):* the root README's GUI section is one sentence and two links (`gui/README.md`, `gui/DEVELOPMENT.md`). The `setup.sh` part stays with T1-8.

**T6-7 | nit | Claude | `gui/README.md:16, 52, 155-193`**
- The README promises a `.tar.gz` "for your machine", but there's no arm64 tarball yet (T1-4).
- Every port mention follows `:8085` (L-1).

Update both together with those fixes.

*Progress 2026-11-18:* the port mentions follow 8080 (L-1). The tarball wording waits on T1-4.

### Track 7: tests

Overall: the Go side is excellent. Integration-first tests, real sockets, a fake vision_processor, spec-checked responses, and three consecutive `-race` reruns with no failures. Cross-package coverage per package is 72-100%; the only unexercised code is `main.run()` wiring, which `release-check` and e2e run as a binary. The gaps are almost all on the frontend.

| Test type | Go host | Frontend |
|---|---|---|
| Unit | broad, every package | 8 files: alerts, cameraLayout, checklist, corners, live, teams, report ×2 |
| Integration (real sockets, fake VP) | `realsockets_test`, `supervised_test`, loopback suites (CI only) | none |
| API contract | `responses_test`, `declared_test`, `openapi_test` | none (see T3-2) |
| Component | n/a | **none** (62 components, 10.5k lines) |
| E2E (real host, Chrome) | | 13 tests, 1 known-bug marker |
| Packaging / release | `release-check.sh`, `deb-check.sh` on systemd | |
| Accessibility | | **none** |

**T7-1 | major | Will | `frontend/src/lib/**/*.svelte`**
There are no component tests. The interactive, math-heavy components (`config/YuvPositionPane.svelte` 1047 lines, `config/CameraPanel.svelte` 650, `config/WeightTriangle.svelte` 529, `CornerPicker.svelte` 450, `config/LayoutMap.svelte` 320) are covered only by e2e tests that mostly assert "it renders".
Fix: follow the repo's existing pattern of pulling logic out into `.ts` modules and unit-testing those (as `corners.ts` and `cameraLayout.ts` already are). For the remainder, add `@testing-library/svelte` on the existing `jsdom` dev dependency. Priority: corner picking, YUV plane and weight-triangle coordinate math, layout map drag and assignment.

**T7-2 | major | Will | untested logic modules**
None of these has a test:
- `wrapper-bus.ts` (147 lines): reconnect, resubscribe and latest-value cache. Every live view depends on it, and a regression here looks like "the GUI froze".
- `video/videoStream.ts` (292): MSE buffer and append management.
- `color.svelte.ts` (311)
- `cameraRanges.ts` (172)
- `cameraSettings.svelte.ts` (174)
- `wizard/wizard.svelte.ts` (145)
- `layout/nav.svelte.ts` (140): hash routing
- `logs/hostLog.svelte.ts` (126): log long-poll
- `preferences.svelte.ts` (81): localStorage, including the failure path

Fix: unit tests, with a fake `WebSocket` for `wrapper-bus` (close → reconnect → resubscribe → cached value delivered) and a fake `SourceBuffer` for `videoStream`.
*Progress 2026-11-11 (Claude):* tests added for `wrapper-bus.ts` (98% statements), `logs/hostLog.svelte.ts` (96%), `layout/nav.svelte.ts` (98%), `network.svelte.ts` (99%), `color.svelte.ts` (99%) and `live/draw.ts` (98%, against a recording canvas context). Then `fieldSplit.ts`, `cameraRanges.ts` and `util.ts` (100%). Committed in `1d081ba` (now `0395479` after the T1-1 rewrite). `advanced.svelte.ts` added with T5-1. Still untested: `videoStream.ts`, `cameraSettings`, `wizard`, `lineDetection`, `layoutView`, `preferences` failure paths.

**T7-3 | major | Will | `frontend/e2e/smoke.spec.ts:139-160`**
Known bug, marked `test.fail()` since `09cf1ee` (2026-10-24): leaving a page while an input is focused throws an uncaught page error (`derived_inert`, "L(...) is not a function") from flowbite-svelte 1.33.1's `Input` calling `onblur` after unmount. That is the most common editing path (type a value, click another tab). The edit itself saves; any other impact of the uncaught error is unverified. 1.33.1 is the newest release in range, so there is no upgrade fix.
Fix: work around it (blur `document.activeElement` before a route change in `nav.svelte.ts`, or wrap `Input`'s `onblur`), file it upstream, and remove `test.fail()`. Don't ship a known uncaught error on the main path.

*Done 2026-11-18 (Claude, with T1-5):* fixed upstream by svelte 5.55.5 → 5.58.0 (flowbite-svelte still 1.33.1). `test.fail()` is removed; the test passed 20 of 20 repeats and now guards it.

**T7-4 | major | Will + Claude | `frontend/e2e/`**
E2E doesn't cover the first-release paths most likely to go wrong. Missing scenarios:
- `-runCameras` with a stand-in vision_processor (the Go suite's fake VP and `release-check.sh`'s stand-in already exist)
- host restart while the page is open (WS reconnect, stale-revision handling)
- a supervised VP crash loop showing up in the UI
- load / save-as
- competition-mode alerts
- a narrow viewport
- an accessibility check

Fix: add `@axe-core/playwright` to "every page renders" (cheap, catches label and contrast regressions), then the scenarios above. Track 5 will refine the list.

**T7-5 | minor | Claude | `frontend/package.json`**
No way to measure frontend coverage (`@vitest/coverage-v8` isn't installed, so `vitest --coverage` fails). Add it and a `test:coverage` script; report only, don't gate.
*Done 2026-11-11 (Claude, `759871b`):* `@vitest/coverage-v8` added, `npm run test:coverage`, `coverage/` ignored. Frontend statements: 9.7% → 19.3% (`.ts` logic modules high; the remainder is Svelte components, see T7-1).

**T7-6 | nit | Claude | `gui/Makefile` e2e / release-check**
`main.run()` and the wiring around it show 0% because they're only exercised as a binary. `go build -cover` with `GOCOVERDIR` for the e2e and release-check binaries would measure that, and show which startup paths no test reaches.

*Done 2026-11-18 (Claude, at Will's request):* `make e2e-cover` builds the host with `-cover` and runs the browser tests with `GOCOVERDIR`, then prints per-package coverage and writes `bin/cover/e2e.out`. `run` goes from 0% to 70.4%. Needed two fixes: Playwright now stops the hosts with SIGTERM (`gracefulShutdown`), and `serve.mjs` starts the host in its own process group, since a second SIGTERM (the group's plus the forwarded one) made it quit without a clean exit. `release-check` isn't instrumented.

**T7-7 | nit | Will | 26 `time.Sleep` calls in 11 Go test files**
Three `-race` reruns were clean, but sleeps are where CI-load flakes start. Where a sleep waits for a condition rather than for time to pass, poll against a deadline instead (`mctest` already has helpers).

*Done 2026-11-18 (Claude, at Will's request):* condition sleeps are now waits: the hub's WebSocket tests wait for the subscriber count (`waitSubscribers`), and `TestAChangeReachesWebSocketSubscribers` waits for `hub.Watched` (`waitUntil`). The sleeps left assert that something does *not* happen (no restart after a debounce, no packet from another group, no suspend while awake) or space edits inside a debounce window, and need real time; the rest are inside poll loops.

**T7-8 | minor | Will | `frontend/src/lib/color.svelte.ts:59-77`**
Found while adding tests. `yuvInGamut` has no rounding tolerance. Because `rgbToYuv` rounds to integers, about 4% of real RGB colors (5,821 of 140,608 sampled) come back outside the gamut by up to 2.9. Every saturated canonical color and pure white is among them. The YUV plane on the Color page therefore shades part of the true gamut boundary as unreachable, and "least unreachable Y" picks can be off by one at the edge.
Fix: treat an overflow under ~3 as in gamut (or compare against the rounded decode). Pinned as `it.fails` in `color.test.ts`: remove the `.fails` when it's fixed.

*Done 2026-11-18 (Claude, at Will's request):* `yuvInGamut` accepts an overflow under `GAMUT_TOLERANCE` (3). The pinned test is a real one now and also checks every RGB color in steps of 5 (140,608) is in gamut. `yuvGamutOverflow` is unchanged, so the least-unreachable-Y search still ranks by the exact overflow.

### Track 8: cross-cutting consistency

**T8-1 | major | Will | product name**
The product has four names. The browser title and header say **"SSL Vision Processor"**, which reads as if this page *is* the vision processor. The README, man page, package description and systemd unit say **"vision_processor GUI"** (11 uses), with "vision-processor GUI", "vision processor GUI" and the log's "Init VP GUI" mixed in. The binary and package are `ssl-vision-processor-gui`. First-time users meet all of these in their first ten minutes.
Fix: pick one display name (for example "SSL Vision Processor GUI") for the title, header, README H1, man page NAME, package description, unit `Description=` and startup log. Keep `vision_processor` in code font only for the C++ binary.

*Done 2026-11-25 (Claude, at Will's request; uncommitted):* "SSL Vision Processor GUI" everywhere it's displayed: title, header, About, README, man page, package, units, account description, API docs, startup log, config file header. `vision_processor` is only the C++ program. Binary, package, folder and account names are unchanged.

**T8-2 | minor | Will + Claude | page names**
The same pages have different names in the sidebar ("Camera", "Layout", "Dimensions"), page headings ("Camera settings", "Field dimensions"), README ("Camera Settings page", "Camera Layout page") and the frontend README ("Field Dimensions", "Camera Layout"). Headings use sentence case and the README uses title case.
Fix: one name per page from `lib/text`, used by the nav, the heading and the docs. Pick a case convention.

**T8-3 | minor | Will | identifier naming across layers**
- **Camera id:** `camera_id` (vision.yml), `cameraId` (JSON), `{id}` vs `{camID}` (paths, T3-3), `cam`/`camera`/`id` (logs, T2-6), "cam 0" / "camera 0" / "FIELD CAM #0" / "CAM 0" (UI).
- **Topic and path case:** mixed (T3-3).

Fix: one rule per layer (snake in YAML, camelCase in JSON, `{id}` in paths, `camera` in logs, "Camera 0" in UI), recorded in `ARCHITECTURE.md`.

**T8-4 | minor | Will | text centralization ahead of i18n (v2.1)**
`check-text.mjs` enforces sentences of 6+ words, but short labels and button text stay inline in components by design (`MIN_WORDS = 6`): "Subscribe to wrapper_packet.out", "Refresh frame", "Lock current calibration", tab names. For v2.1, every one of these has to move. Moving them now, while the components are fresh and before others contribute, is cheaper.
Fix: lower the threshold for `.svelte` markup text to 1 word (with `text-ok` for symbols and units), or add that as a second, warn-only check now.

**T8-5 | nit | Will | punctuation**
ASCII `--` is used as an em dash in UI text, alerts, the README and the debconf templates. Use `—` (or a comma) in user-facing text; it costs nothing and reads as finished.

## League alignment (track 9)

Read-only fresh clones, 2026-11-11: ssl-go-tools master `ce5fa35` (latest tag v1.12.0 = `3254f60`), ssl-game-controller master `e267c58`, plus ssl-status-board and ssl-remote-control for conventions. Main thread spot-checked: PR #29 state, `multicast_server.go:53-59` and `:13`, GC compose ports, `main.go:326`.

### Summary

- **Nothing new has been merged to sslnet.** No sslnet change has landed on master since v1.12.0 (2026-07-02). Nicolai's work is **open PR #29** "feat: Add UdpServer for multicast and broadcast reception" (g3force, 2026-11-07, branch `improved-multicast`). CI is green and it has no human review. On master and v1.12.0, which GC and status-board pin, **all four deficiencies in `gui/CLAUDE.md` remain**. Two more bugs on master: a socket fd leak on a failed `JoinGroup` (`multicast_server.go:113-116`), and an unguarded write to `r.connection` racing with `Stop` (`:103` vs `:57`).
- **PR #29 fixes all four for receive, without breaking anything.** `MulticastServer` and `BroadcastServer` become deprecated wrappers with the same signatures. It joins every eligible interface on one socket and re-checks every 5 s, uses a 2 MiB SO_RCVBUF with a 64 KiB read buffer, and filters by destination via IP_PKTINFO on Linux. It has no send-side changes, no health or status for callers, a fixed buffer size, and no runtime address change.
- **We can adopt very little.** Upstream protobuf bindings are all under `internal/` (not importable). Only `pkg/sslnet` and `pkg/sourcefilter` are safe to import. **Never** import `pkg/persistence`, `player`, `stats` or `auto`: they register a second `ssl_vision_wrapper.proto` under the same name, a protobuf-go registry conflict (inferred, not reproduced). `sourcefilter` is the one cheap, worthwhile adoption. **Correction 2026-11-18:** ssl-go-tools is GPL-3.0 and this repo is Apache-2.0, so nothing from it can be imported or copied without making the binary GPL; L-5 was written independently.
- **Upstream contributions:** the receive-buffer option, a health accessor, interface listing, a resilient sender and the loopback test helper can all land as **purely additive** additions on top of #29. Anything that changes `MulticastServer`/`UdpClient` behaviour for existing users stays local.
- **Shared protos are in sync.** Referee, game event, common, GC geometry, tracked and vision geometry are identical to GC master once comments are stripped. Ours adds only `t_offsets` and `SSL_VPConfig config`, both wire-compatible. ssl-go-tools' own referee proto is stale against GC; that doesn't affect us.
- **For context only (protocol track, out of scope):** the `proto` submodule is at `fdd7988` on ssl-protocol-defs `dev/vision-processor`, two commits ahead of `main` and not merged.

### Adopt from upstream

| Item | Ours | Upstream | Verdict |
|---|---|---|---|
| Receive sockets (v1.12.0) | `internal/multicast/socket.go`, `endpoint.go` | `sslnet.MulticastServer` | **Reject**: all four deficiencies, plus the leak and race |
| Receive sockets (PR #29) | `socket.go:33-140`, `socket_linux.go` | `sslnet.UdpServer` | **Defer.** Unreleased. Would save ~150 lines, but we'd lose `Status.Problem` (Network page) and the per-endpoint buffer (video wants 4 MiB). Revisit when it's tagged *and* U-1/U-2 have landed. `Select`'s skip list maps straight onto `SkipInterfaces`. |
| Send sockets | `sender.go` | `sslnet.UdpClient` | **Reject**: it dials once and drops failed connections forever; no health, no `IsOwn` |
| Duplicate-GC filtering | `main.go:324-327` | `pkg/sourcefilter` (`ab1bf6e`), ~100 lines, stdlib only | ~~Adopt~~ GPL-3.0: own implementation, see L-5 |
| Protobuf bindings | `internal/vision`, `internal/gamecontroller` | upstream `internal/...` | **Keep**: not importable, lacks our VP fields |

### Upstream candidates (additive only, on top of PR #29)

| Item | Ours | Additive? | Verdict |
|---|---|---|---|
| U-1 receive-buffer size | `endpoint.go:40-42` | yes: `UdpServer.ReceiveBufferSize`, 0 keeps 2 MiB | **Propose** (on #29 or right after); a prerequisite for us adopting it |
| U-2 health / joined interfaces | `endpoint.go:52-60,93-97` | yes: new method or optional `OnProblem` callback | **Propose** |
| U-3 exported interface listing with reasons | `host.go:47-145` | yes, as a new exported func. Making our virtual-interface auto-exclusion the default would **not** be additive. | **Propose as opt-in**; keep the default local |
| U-4 resilient multi-interface sender | `sender.go` | yes: a new type; `UdpClient` untouched | **Propose.** GC's publisher has the same "dial once" weakness. |
| U-5 loopback multicast test helper | `internal/multicast/mctest` | yes: a new `sslnettest` package | **Offer with U-4**; needs decoupling from `multicast.Interface` |
| U-6 IP_MULTICAST_ALL=0 | `socket_linux.go` | no: it changes socket behaviour | Keep local (#29's destination filtering covers it) |
| U-7 reopen / SetAddress / suspend detection | `endpoint.go:125-195`, `suspend*.go` | would duplicate #29's lifecycle | Keep local |
| U-8 duplicate-sender watch | `internal/peers` | additive, but tied to our alerts | Keep local (`sourcefilter` covers the league need) |
| U-9 buildinfo `-version` | `internal/buildinfo` | additive, per repo | Keep local; maybe an issue: no league binary has `-version` |
| U-10..13 logging, typed API/OpenAPI, video, referee/discovery/origins | various | additive but heavy deps or VP-specific | Keep local |

### Convention divergences

**L-1 | major (sticky default; decide before release) | Will | `cmd/ssl-vision-processor-gui/main.go:42`, README, man page, debconf, ufw text**
Our default port `:8085` collides with the league's reference stack. GC's `docker-compose.yaml:67-74` runs the second ssl-remote-control on `:8085` (8081-8084 are GC, vision-client, status-board and remote-control). On a PC that also runs the league stack, our system service fails to bind at boot. Unverified: how often the vision PC and the GC PC are the same machine at events.
Fix: pick a port outside 8081-8085, and change every mention in one pass.

*Done 2026-11-18 (Will chose 8080; nothing in the league uses it):* the default is now `:8080` in the flag, `warnFirewall`, README, DEVELOPMENT.md, CLAUDE.md, the man page, debconf template, `configure-service`, the systemd unit, `deb-check.sh`, the Vite proxy and the tests. Older mentions of 8085 in this file are left as written. An installed service keeps whatever ADDRESS its `/etc/default` file already has.

**L-2 | minor | Will | `main.go:44-46`, README**
Every sibling tool takes `-visionAddress`, `-trackerAddress`, `-refereeAddress`/`-publishAddress` and `-skipInterfaces` (GC `main.go:21-25`, status-board, vision-client, remote-control). Ours has none by design (host-owned config). Users moving between league tools, or scripting a sim setup with moved groups, find no equivalent.
Fix (minimum): a README "Coming from other league tools" table mapping each flag to its vision.yml key or Network page control. Better: accept them as startup overrides written into the document and shown as such in the GUI.

**L-3**: merged into T1-4 (tarball and `-bin` are amd64 only). Also consider league-style `_` asset names (`<cmd>_<v>_linux_<arch>`) so league download scripts work unchanged.

**L-4 | minor | Claude | repo root**
No dependency bot. Every league Go repo runs Renovate (GC `.github/renovate.json`), and our `x/net` is already 7 minors behind go-tools. Fix: `renovate.json` for gomod, npm, github-actions. Pairs with T1-5.

**L-5 | major | Will | `cmd/ssl-vision-processor-gui/main.go:324-327`**
Every referee packet from any sender overwrites `match.Teams`. With a second GC on the network (common in labs and in team areas at events), team names, side assignment and the missing-height alert flap. Upstream fixed exactly this for its own tools (`pkg/sourcefilter`, `ab1bf6e`).
Fix: gate `trackReferee` with `sourcefilter.Accept(from.IP)` (or copy its ~30-line `Accept`), and keep the `peers` alert.

*Done 2026-11-18 (Claude, at Will's request):* `referee.Source` follows the first game controller heard and switches only after it's silent for 3 s (`SourceTimeout`); every sender still counts toward the `sender_conflict` alert. **Not** upstream's `pkg/sourcefilter`: ssl-go-tools is GPL-3.0 and this repo is Apache-2.0, so importing or copying it would make the binary GPL. Written from the idea, not the code. The same applies to the "Adopt from upstream" table and "safe to import" note below: sslnet is GPL too.

**L-6 | nit | Claude | `gui/CLAUDE.md` Stack (sslnet bullet)**
Add "re-evaluate when ssl-go-tools PR #29 is released; still needs U-1/U-2", plus the leak and race as further reasons.

*Done 2026-11-18 (Claude, at Will's request):* the sslnet bullet in `gui/CLAUDE.md` adds the leak and race, and says when to look again (PR #29 released, with U-1/U-2).

**L-7 | nit | Will | `main.go:58`**
GC, vision-client and status-board use `-verbose`; we use `-logLevel`. Accept `-verbose` as an alias for `-logLevel Debug`.

*Done 2026-11-18 (Claude, at Will's request):* `-verbose` sets the level to Debug, overriding `-logLevel`. In the man page and README flag table.

**L-8 | nit | Claude | packaging**
No docker image (`robocupssl/<cmd>` is the league pattern). A host-only image would join GC's compose stack. Defer past v1.

**Intentional divergences, no action:** XDG config with snake_case keys (GC: cwd-relative with kebab-case); REST+OpenAPI+`/ws` (GC: one protobuf WS); `//go:embed all:dist` plus SPA fallback; slog with a file (GC: `log`); GitHub Actions + nix (league: CircleCI); binding `:808x` on all interfaces (matches status-board and remote-control; GC alone binds localhost); the "UI is available" log line matches GC.

**Not verified:** PR #29's runtime behaviour (read only); whether CircleCI can enable loopback multicast; how events co-locate machines; the protobuf registry panic (inferred).

## Future features

These came up during the review. They're out of scope for the MVP and not findings.

- **Accessibility pass (v2.1, from T5-5).** Labels for every form control, contrast for the muted grays and tab headers, focusable scroll regions, and axe in e2e (T7-4) so it stays fixed. Also color-blind safety: team and blob colors, alert severities and status dots shouldn't rely on hue alone (add shape, text or pattern), checked with a color-vision simulator.

- **The rest of `debug:` in the GUI** (from T5-1). Each one is left out for a reason:
  - `wait_for_geometry` hangs vision_processor on a first run, because nothing is published before the first save. Before exposing it, either the host publishes field-only geometry during the first run, or the toggle carries a first-run warning.
  - `debug_images` writes `img/.flat.1.png`, `.gradDot.1.png` and `.blob.1.png` once per start. The names have no camera prefix, so the snapshot listing skips them. Worth exposing once vision_processor names them `img/<cam>.…` (a C++ change).
  - `ground_truth` is for the benchmarks only. If it's ever exposed, it's a file name and must go through `config.ValidName`.
- **Debug images over the network** (from T4-1, Will: needed long term). The Color views (`flat`, `blob`) and Geometry views (`pixels`, `lines`, `pixels.corner`, `pixels.refined`) are read from the host's own `img/`, so they only work for cameras on the same machine (gui/CLAUDE.md "Snapshots for remote instances"). Getting them from a vision_processor on another machine is a protocol change. `SnapshotViews.svelte` only needs a new source once that exists.
- **Cameras at a glance in the sidebar** (from T5-6, Will). Summary data on each camera in the list.
- **Per-camera overrides on the Advanced page.** Today the page only names the cameras that override a shared value. It could show and reset each override.
- **Tidy-up after T5-1:** `CONFIG_FIELDS`, `ConfigCategory.fields` and `yamlKey` have nothing left that reads them.

## Parked: needs C++ / protocol change

Not findings against this review. Recorded so they aren't lost.

- **vision_processor's `camera.driver` fallback is `SPINNAKER`** (mirrored by `/api/schema`). The host-side fix is T5-3. A FOSS-first default in C++ belongs with the CV `.deb` work.
- **The announce is sent from inside the frame loop and built once at startup** (`gui/CLAUDE.md` "Not yet built"). A vision_processor whose camera blocks never announces, so it is invisible to discovery exactly when it's broken. Related to T5-2, which covers the supervised case host-side.
- **SIGTERM is checked only between frames**, so a blocked `readImage()` needs the supervisor's SIGKILL fallback. That works today; a cleaner C++ shutdown would shorten restarts.
- **No way to ask a running instance to recalibrate.** Field changes restart supervised cameras to force it, and external cameras need a manual restart.
- **The proto submodule is pinned to ssl-protocol-defs `dev/vision-processor` (`fdd7988`), two commits ahead of `main` and unmerged.** Release builds embed protocol definitions that aren't on the league's main branch. Merge or tag them before the first release.

## Parked: future security plan

Titles only; not written up.

- No authentication or authorization on the API or WebSockets
- Origin and Host policy (`internal/origins`) scope and defaults; `-allowOrigin`/`-allowHost` semantics
- WebSocket origin checks (`/ws`, `/ws/video/{id}`)
- Request body size and rate limits; long-poll limits on `/api/logs?wait=`
- `-runCameras` process execution model (PATH lookup, working directories, symlinks)
- systemd unit hardening review (`ProtectSystem=strict` etc.) and the `rcssl` group's read access
- Dependency vulnerability policy (govulncheck / npm audit as gates, not just hygiene)
- Debconf "Any machine on the network" wording and default

## MVP gate

Counts: **3 blockers, 21 majors, 29 minors, 15 nits.** In fix order (related items grouped):

1. **Repo:** drop the tracked binary from the branch before it merges to master. *(T1-1, blocker)*
2. **Unfinished UI out of the product:** the "Temporary debug utilities" block under every page and the placeholder Debug page. *(T4-1, T5-1, blockers)*
3. **First start works, or says clearly why not:** bind the port before starting anything, with a plain error; choose the default port now. *(T2-1, L-1)*
4. **The lab path works:** webcam-friendly driver default or a wizard camera step; an alert and a visible state for a crash-looping vision_processor; first-run alert severities tuned so warnings mean something. *(T5-3, T5-2, T5-4)*
5. **No silent data loss or known errors on the main editing path:** retry and report edits sent while the host is unreachable; work around the flowbite `Input` unmount error and remove `test.fail()`. *(T4-2, T7-3)*
6. **Accessibility baseline:** label every control, fix the contrast tokens, and add axe to e2e so it stays fixed. *(T5-5, T7-4)*
7. **One navigation and one name:** a single nav model and a single product name. *(T5-6, T8-1)*
8. **Freeze the API you'll have to keep:** naming pass, a topic registry with typed topics, and the frontend on the generated `paths`. *(T3-3, T3-1, T3-2)*
9. **League behaviour:** filter duplicate game controllers with `sourcefilter`. *(L-5)*
10. **Release pipeline:** gate the tag on `make check` and e2e, publish as a draft or prerelease with real release notes, fix the package description, bump the toolchain and dependencies. *(T1-2, T1-3, T1-5)*
11. **Docs follow the product:** fix `ARCHITECTURE.md` drift, add the quick-start camera step and troubleshooting rows. *(T6-1, T6-5)*
12. **Test debt that protects the above:** unit tests for `wrapper-bus`, `videoStream` and the other logic modules, plus extracted component math. Can trail the tag slightly, but should land before outside contributors arrive. *(T7-1, T7-2)*

Upstream (ssl-go-tools) is not on the MVP path. Propose U-1/U-2 on PR #29 now, so adopting `UdpServer` is possible after the release.
