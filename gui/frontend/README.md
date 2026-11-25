# vision-processor-gui-frontend

Browser UI for the SSL Vision Processor GUI. Svelte 5, TypeScript, and Vite. See
[`gui/DEVELOPMENT.md`](../DEVELOPMENT.md) for building it as part of the Go host, and
[`gui/ARCHITECTURE.md`](../ARCHITECTURE.md) for how the whole system fits
together. This document covers the frontend's own layout and scripts.

The UI is organized as a two column shell: the alert counts and the camera
list on the left, the selected page on the right. The tab bar above the page
holds every page but Alerts: the shared Live Field, Teams, Network, Dimensions,
Layout and Advanced (expert users only), and the selected camera's Camera,
Geometry, Color and Stream. Picking a camera on the left changes which one the
per-camera tabs edit. The settings pages edit the host's
`vision.yml` working copy, which applies live; Save in the settings menu
writes it to disk, and a tab shows an asterisk while it has unsaved changes.
Dimensions edits the shared field. Geometry holds the selected camera's
calibration lock and the corner picker, which marks calibration corners on a
debug snapshot. The snapshots (via `GET /api/snapshots`) exist only when the
host and the vision_processor share a filesystem; see "Not yet built" in
[`gui/ARCHITECTURE.md`](../ARCHITECTURE.md).

## Run

Two options, both from `gui/`:

```
# Through the Go host itself: builds everything, serves on :8080
make run
```

```
# Vite dev server with hot reload on :5173, proxying /api and /ws to :8080.
# Needs the host running (make run, in another terminal) and one `make
# frontend` first: the protobuf bindings (src/proto) and the API types
# (src/lib/generated) are generated, not committed.
make frontend
cd frontend && npm run dev
```

Use `make npm` rather than `npm install` to install packages: it runs
`npm ci`, which keeps `package-lock.json` as committed. The frontend needs
Node 22 (`.nvmrc`); the nix dev shell provides it.

## Layout

- `src/lib/layout/` holds the shell: `Shell.svelte` (a Flowbite Navbar with
  the status badges and gear menu, over a Sidebar and the main column),
  `InstanceList.svelte` (the sidebar's cameras),
  `TabBar.svelte`, and `MainContent.svelte`, which switches panels on
  `nav.selectedCategoryId`. `nav.svelte.ts` and `configCategories.ts` hold the
  shared navigation state and the category list itself.
- Navigation follows the URL hash, `#<camera>/<page>` (e.g. `#0/geometry`):
  the cameras, tabs, and header badges are links, so back/forward and
  bookmarks work. The tab bar is links styled with Flowbite's tab theme rather
  than its Tabs component, which can't leave every tab unselected on a page
  that isn't a tab.
- `src/lib/FieldDimensionsEditor.svelte` is the Field Dimensions editor. It edits
  `config.doc.field` in the working document; `src/lib/geometry.svelte.ts`
  holds what the host derives from it (the field markings) and the rulebook
  presets.
- `src/lib/live/` is the Live Field page. `LiveFieldPanel.svelte` subscribes
  to `live-field.state` and `geometry.state`; `FieldCanvas.svelte` draws
  on two canvases, the field (redrawn on resize or a geometry change) and
  what's on it (at most once per animation frame). `model.ts` reduces
  detection and tracked frames to one shape in mm, `sources.ts` lists and
  picks sources, `draw.ts` draws, `patterns.ts` is the standard robot
  pattern copied from `src/pattern.h`, and `liveView.svelte.ts` keeps the
  view settings in localStorage.
- `src/lib/config/GeometryPanel.svelte` is the Geometry category's panel. It
  embeds `src/lib/CornerPicker.svelte`, which edits the selected camera's
  `seed` in the working document (`config.svelte.ts`).
- `src/lib/apiTypes.ts` names the host's API types, generated from its
  OpenAPI spec into `src/lib/generated/` by `make`. Import API shapes from
  there; never copy a Go type by hand.
- `src/lib/alerts/` is the Alerts page and its sidebar summary.
  `alerts.ts` derives every known problem from state the browser already
  has and sorts it into errors, warnings, and cautions; `docs.ts` holds the
  resolution docs its "How to fix" buttons open. To add an alert, push it in
  `collect()` and point its actions at a page and/or a doc.
- `src/lib/api.ts` is the HTTP API client (`openapi-fetch`), typed by the
  host's OpenAPI spec: a wrong path, parameter or body doesn't compile. Call it
  as `unwrap(api.GET("/api/health"))`; `apiPath` builds the URLs that aren't
  fetched (an image's `src`, a WebSocket).
- `src/lib/bus.svelte.ts` is a single `WebSocket` client. It exposes
  `connection.state`, `topicState(name)` (the topic's latest message as
  `current`, for components) and `onTopic(name, fn)` (for state modules). Both
  are typed by the topic's name, from the host's topic list; an unknown name
  doesn't compile. A topic subscribes with its first reader and unsubscribes
  with its last. It reconnects on close
  with exponential backoff, from 1 second up to 30.
- `src/lib/SnapshotViews.svelte` is the debug image card on the Color and
  Geometry pages: the views a camera has written, listed on demand (not
  polled), with their labels in `src/lib/text/snapshots.ts`.
- `src/App.svelte` mounts the shell and the setup wizard, and loads the
  config.
- `src/main.ts` mounts `App` into `#app`.

The WS wire format (`gui/internal/hub`):

```jsonc
// client -> server
{ "action": "subscribe",   "topic": "geometry.state" }
{ "action": "unsubscribe", "topic": "geometry.state" }
// server -> client
{ "topic": "geometry.state", "data": { ... } }
```

`geometry.state` carries the current `SSL_WrapperPacket` as
canonical protojson, republished once per second.

Snapshot endpoints are plain HTTP: `GET /api/snapshots` returns the
list of available `{cameraId, view}` entries as JSON;
`GET /api/snapshots/<id>/<view>` returns the actual `image/jpeg` or
`image/png` (or 404 if missing).

## Scripts

```
npm run dev           # Vite dev server with HMR
npm run build         # production build to dist/
npm run preview       # serve the production build locally
npm run check         # svelte-check + tsc (type-check everything)
npm run lint          # eslint over src/
npm run check:text    # user-facing text only in src/lib/text/
npm test              # vitest unit tests (src/**/*.test.ts, scripts/)
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
`make run` in [`gui/DEVELOPMENT.md`](../DEVELOPMENT.md).
