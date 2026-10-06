# vision-processor-gui-frontend

Browser UI for the vision-processor GUI. Svelte 5, TypeScript, and Vite. See
[`gui/README.md`](../README.md) for building it as part of the Go host, and
[`gui/ARCHITECTURE.md`](../ARCHITECTURE.md) for how the whole system fits
together. This document covers the frontend's own layout and scripts.

The UI is organized as a two column shell: an instance list and config
category nav on the left, the selected category's panel on the right. A tab
bar above the panel gives quick access to the categories used most, Virtual
Field, Geometry, and Color. All three edit the host's `vision.yml` working
copy, which applies live; Save in the settings menu writes it to disk, and a
tab shows an asterisk while it has unsaved changes. Field Dimensions edits the
shared field. Geometry holds the selected camera's calibration lock and the
corner picker, which marks calibration corners on a debug snapshot. Debug
snapshots (via `GET /api/snapshots`) are only
meaningful when the browser and the vision processor share a filesystem, see
the root [README.md](../../README.md).

## Run

Two options:

```
# Standalone dev server with HMR (proxies /api and /ws to the Go host on :8085)
cd gui && make proto   # regenerates src/proto -- not committed to git, see gui/CLAUDE.md
cd frontend
npm install
npm run dev
```

```
# Through the Go host itself, on its own port, no proxy involved
cd gui
make run
```

Either way the Go host in `gui/` must be running for the connection
badge and snapshot grid to show anything.

## Layout

- `src/lib/layout/` holds the shell: `Shell.svelte` (a Flowbite Navbar with
  the status badges and gear menu, over a Sidebar and the main column),
  `InstanceList.svelte` and `ConfigNav.svelte` (the sidebar's groups),
  `TabBar.svelte`, and `MainContent.svelte`, which switches panels on
  `nav.selectedCategoryId`. `nav.svelte.ts` and `configCategories.ts` hold the
  shared navigation state and the category list itself.
- Navigation follows the URL hash, `#<camera>/<page>` (e.g. `#0/geometry`):
  sidebar items, tabs, and the header badges are links, so back/forward and
  bookmarks work. The tab bar is links styled with Flowbite's tab theme rather
  than its Tabs component, which can't leave every tab unselected on a page
  that isn't a tab.
- `src/lib/FieldEditor.svelte` is the Field Dimensions editor, backed by
  `src/lib/geometry.svelte.ts`, a module level `$state` object shared with
  anything else that needs the field config.
- `src/lib/config/GeometryPanel.svelte` is the Geometry category's panel. It
  embeds `src/lib/CornerPicker.svelte`, backed by
  `src/lib/lineCorners.svelte.ts` in the same way.
- `src/lib/alerts/` is the Alerts page and its sidebar summary.
  `alerts.ts` derives every known problem from state the browser already
  has and sorts it into errors, warnings, and cautions; `docs.ts` holds the
  resolution docs its "How to fix" buttons open. To add an alert, push it in
  `collect()` and point its actions at a page and/or a doc.
- `src/lib/api.ts` holds the fetch helpers (`requestJSON`, `withLoadingState`)
  every panel's load and save functions are built on.
- `src/lib/wrapper-bus.ts` is a single `WebSocket` client. It exposes
  `connectionState` (a Svelte store) and `topic<T>(name)` (a store of the
  latest message on that topic). A topic subscribes lazily on first reader
  and unsubscribes when the last reader goes away. It reconnects on close
  with exponential backoff, from 1 second up to 30.
- `src/App.svelte` mounts the shell and still carries a couple of temporary
  debug utilities predating it (a snapshot grid and a raw `wrapper_packet.out`
  dump), kept until the mockup's real Video and Debug Console panels exist.
- `src/main.ts` mounts `App` into `#app`.

The WS wire format (`gui/internal/hub`):

```jsonc
// client -> server
{ "action": "subscribe",   "topic": "wrapper_packet.out" }
{ "action": "unsubscribe", "topic": "wrapper_packet.out" }
// server -> client
{ "topic": "wrapper_packet.out", "data": { ... } }
```

`wrapper_packet.out` carries the current `SSL_WrapperPacket` as
canonical protojson, republished once per second.

Snapshot endpoints are plain HTTP: `GET /api/snapshots` returns the
list of available `{cam_id, view}` entries as JSON;
`GET /api/snapshot/<cam_id>/<view>` returns the actual `image/jpeg` or
`image/png` (or 404 if missing).

## Scripts

```
npm run dev           # Vite dev server with HMR
npm run build         # production build to dist/
npm run preview       # serve the production build locally
npm run check         # svelte-check + tsc (type-check everything)
npm run lint          # eslint over src/
npm run format        # prettier --write .
npm run format:check  # prettier --check . (CI-style)
```

TypeScript is configured strict (`strict`,
`noUncheckedIndexedAccess`, `noImplicitOverride`,
`noPropertyAccessFromIndexSignature`, `noImplicitReturns`,
`noFallthroughCasesInSwitch`).

## Production serving

`npm run build` produces `dist/`, which the Go host in `gui/` embeds
directly into its binary (`gui/frontend/embed.go`) and serves on its
own port, with a fallback to `index.html` for client-side routes. See
`make run` in the root [README.md](../../README.md).
