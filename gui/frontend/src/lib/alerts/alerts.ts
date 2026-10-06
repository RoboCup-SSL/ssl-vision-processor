// Every known problem the GUI can see right now, sorted into errors (broken:
// something isn't working), warnings (likely wrong: fix before a match), and
// cautions (worth knowing, often fine). Derived on demand from state the
// browser already has (config state, network state, the camera layout), so
// nothing here is stored or sent to the host.
//
// To add one: push it in collect() below with a stable id, and point its
// actions at a page (where it's fixed) and/or a doc in docs.ts (how).
import { config } from "../config.svelte";
import { network } from "../network.svelte";
import { navHref, selectedInstance } from "../layout/nav.svelte";
import { slots, unplacedSources } from "../cameraLayout";
import type { DocId } from "../text/docs";
import { alerts as text } from "../text/alerts";
import { overview as overviewText } from "../text/overview";
import {
  COLORS,
  competitionField,
  heightIssue,
  teamName,
} from "../teams.svelte";

export type Severity = "error" | "warning" | "caution";

export const SEVERITIES: { id: Severity; label: string }[] = [
  { id: "error", label: text.severities.error },
  { id: "warning", label: text.severities.warning },
  { id: "caution", label: text.severities.caution },
];

export type AlertAction =
  // A page in this GUI, e.g. the camera's Geometry tab.
  | { kind: "page"; label: string; href: string }
  // A resolution doc from docs.ts, shown in a dialog.
  | { kind: "doc"; label: string; doc: DocId }
  // Documentation elsewhere.
  | { kind: "external"; label: string; url: string };

export interface Alert {
  // Stable across updates, for keyed lists.
  id: string;
  severity: Severity;
  title: string;
  detail: string;
  cameraId?: number;
  actions: AlertAction[];
}

// Host camera warning codes (gui/internal/config's status.go) by severity
// and resolution doc. Codes not listed show as cautions without a doc.
const CAMERA_WARNINGS: Record<string, { severity: Severity; doc: DocId }> = {
  field_changed: { severity: "warning", doc: "calibration-field-changed" },
  calibration_aspect_mismatch: {
    severity: "warning",
    doc: "calibration-resolution",
  },
  seed_stale: { severity: "warning", doc: "seed-stale" },
  seed_aspect_mismatch: { severity: "warning", doc: "seed-resolution" },
  seed_rescalable: { severity: "caution", doc: "seed-resolution" },
  seed_resolution_unknown: {
    severity: "caution",
    doc: "seed-resolution-unknown",
  },
};

function howTo(doc: DocId): AlertAction {
  return { kind: "doc", label: text.howToFix, doc };
}

function page(label: string, cameraId: number | undefined, category: string) {
  return {
    kind: "page" as const,
    label,
    href: navHref(cameraId ?? selectedInstance()?.cameraId, category),
  };
}

function collect(): Alert[] {
  const out: Alert[] = [];
  const doc = config.doc;
  const state = config.state;
  const net = network.state;

  for (const [camera, message] of Object.entries(state?.renderErrors ?? {})) {
    const cameraId = Number(camera);
    out.push({
      id: `render-${camera}`,
      severity: "error",
      title: text.renderFailed(camera),
      detail: message,
      cameraId,
      actions: [howTo("config-render-failed")],
    });
  }

  if (doc) {
    for (const slot of slots(doc)) {
      if (slot.conflict) {
        out.push({
          id: `conflict-${String(slot.cameraId)}`,
          severity: "error",
          title: text.conflict(slot.cameraId),
          detail: text.heardFrom(
            slot.sources.filter((s) => s.receiving).map((s) => s.address),
          ),
          cameraId: slot.cameraId,
          actions: [
            howTo("camera-id-conflict"),
            page(text.pages.layout, slot.cameraId, "layout"),
          ],
        });
      }

      if (!slot.camera) {
        out.push({
          id: `uncovered-${String(slot.cameraId)}`,
          severity: "warning",
          title: text.uncovered(slot.cameraId, slot.region),
          detail: text.uncoveredDetail,
          actions: [
            howTo("region-uncovered"),
            page(text.pages.layout, undefined, "layout"),
          ],
        });
      }
    }

    for (const source of unplacedSources(doc)) {
      out.push({
        id: `unplaced-${String(source.cameraId)}-${source.address}`,
        severity: "warning",
        title: text.unplaced(source.cameraId, source.address),
        detail: text.unplacedDetail,
        actions: [
          howTo("camera-unplaced"),
          page(text.pages.layout, undefined, "layout"),
        ],
      });
    }
  }

  for (const status of state?.cameras ?? []) {
    for (const warning of status.warnings) {
      const known = CAMERA_WARNINGS[warning.code];
      out.push({
        id: `camera-${String(status.cameraId)}-${warning.code}`,
        severity: known?.severity ?? "caution",
        title: text.cameraWarning(status.cameraId, warning.code),
        detail: warning.message,
        cameraId: status.cameraId,
        actions: [
          ...(known ? [howTo(known.doc)] : []),
          page(text.pages.geometry, status.cameraId, "geometry"),
        ],
      });
    }
  }

  if (state?.external) {
    out.push({
      id: "file-changed",
      severity: "warning",
      title: text.fileChanged(state.path),
      detail:
        state.external.error ??
        text.fileChangedDetail(state.external.changes.length),
      actions: [howTo("file-changed")],
    });
  }

  if (net) {
    for (const [kind, status, label] of [
      ["vision", net.vision, text.socketLabels.vision],
      ["gc", net.gc, text.socketLabels.gc],
    ] as const) {
      if (status.problem) {
        out.push({
          id: `socket-${kind}`,
          severity: "error",
          title: text.socket(label, status.problem),
          detail: text.socketDetail(status.address),
          actions: [
            howTo("socket-problem"),
            page(text.pages.network, undefined, "network"),
          ],
        });
      } else if (!status.receiving) {
        // Off a competition field a game controller is optional: lab use
        // often runs without one. On one, a match can't run without it.
        if (kind === "gc" && !competitionField()) continue;

        out.push({
          id: `silent-${kind}`,
          // The game controller only gets here on a competition field.
          severity: kind === "vision" ? "warning" : "error",
          title: kind === "vision" ? text.visionSilent : text.gcSilent,
          detail: text.silentDetail(status.address, Boolean(status.lastHeard)),
          actions: [
            howTo(kind === "vision" ? "vision-silent" : "gc-silent"),
            page(text.pages.network, undefined, "network"),
          ],
        });
      }
    }

    const ref = net.referee;
    if (ref.heightsError) {
      out.push({
        id: "heights-unreadable",
        severity: "warning",
        title: text.heightsUnreadable(ref.heightsFile),
        detail: ref.heightsError,
        actions: [howTo("heights-unreadable")],
      });
    } else {
      // A playing team that would get vision_processor's default height.
      for (const color of COLORS) {
        const team = teamName(color);
        const issue = heightIssue(color);
        if (!issue) continue;

        const colorLabel =
          color === "yellow"
            ? overviewText.teams.yellow
            : overviewText.teams.blue;

        out.push({
          id: `team-height-${color}`,
          severity: competitionField() ? "error" : "warning",
          title: team
            ? text.teamDefaultHeight(colorLabel, team)
            : text.colorDefaultHeight(colorLabel),
          detail: overviewText.teams.issue[issue],
          actions: [
            howTo("team-height-missing"),
            page(text.pages.overview, undefined, "overview"),
          ],
        });
      }
    }

    if (!net.loopbackMulticast) {
      out.push({
        id: "loopback-multicast",
        severity: "caution",
        title: text.loopback(net.loopback),
        detail: text.loopbackDetail,
        actions: [howTo("loopback-multicast-off")],
      });
    }
  }

  return out;
}

export function alerts(): Alert[] {
  return collect();
}

export function alertsOf(severity: Severity): Alert[] {
  return collect().filter((a) => a.severity === severity);
}
