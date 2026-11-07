// Every known problem the host sees right now (gui/internal/alerts, via
// hostAlerts), sorted into errors (broken: something isn't working), warnings
// (likely wrong: fix before a match), and cautions (worth knowing, often
// fine). The host decides which alerts exist and how serious; this module
// words them and points each at a page (where it's fixed) and/or a doc in
// docs.ts (how).
//
// To add one: add it in gui/internal/alerts with a stable code, then word it
// in present() below.
import { config } from "../config.svelte";
import { navHref, selectedInstance } from "../layout/nav.svelte";
import { regionLabel } from "../cameraLayout";
import type { DocId } from "../text/docs";
import { alerts as text } from "../text/alerts";
import { overview as overviewText } from "../text/overview";
import { hostAlerts, type HostAlert, type Severity } from "./hostAlerts.svelte";

export type { Severity };

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

// Resolution docs for the host's camera warning codes (gui/internal/config's
// status.go). Codes not listed get no doc.
const CAMERA_WARNING_DOCS: Record<string, DocId> = {
  field_changed: "calibration-field-changed",
  calibration_aspect_mismatch: "calibration-resolution",
  seed_stale: "seed-stale",
  seed_aspect_mismatch: "seed-resolution",
  seed_rescalable: "seed-resolution",
  seed_resolution_unknown: "seed-resolution-unknown",
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

// present words one host alert. Params are the host's facts, named per code
// in gui/internal/alerts.
function present(a: HostAlert): Alert {
  const p = a.params ?? {};
  const str = (key: string): string => {
    const v = p[key];

    return typeof v === "string" ? v : "";
  };
  const base = {
    id: a.id,
    severity: a.severity,
    cameraId: a.cameraId,
  };

  switch (a.code) {
    case "render_failed":
      return {
        ...base,
        title: text.renderFailed(String(a.cameraId)),
        detail: str("error"),
        actions: [howTo("config-render-failed")],
      };
    case "camera_id_conflict":
      return {
        ...base,
        title: text.conflict(a.cameraId ?? 0),
        detail: text.heardFrom((p["addresses"] as string[] | undefined) ?? []),
        actions: [
          howTo("camera-id-conflict"),
          page(text.pages.layout, a.cameraId, "layout"),
        ],
      };
    case "region_uncovered": {
      const slot = Number(p["slot"]);

      return {
        ...base,
        title: text.uncovered(
          slot,
          config.doc ? regionLabel(config.doc, slot) : "",
        ),
        detail: text.uncoveredDetail,
        actions: [
          howTo("region-uncovered"),
          page(text.pages.layout, undefined, "layout"),
        ],
      };
    }
    case "camera_unplaced":
      return {
        ...base,
        title: text.unplaced(Number(p["slot"]), str("address")),
        detail: text.unplacedDetail,
        actions: [
          howTo("camera-unplaced"),
          page(text.pages.layout, undefined, "layout"),
        ],
      };
    case "file_changed":
      return {
        ...base,
        title: text.fileChanged(str("path")),
        detail: str("error") || text.fileChangedDetail(Number(p["changes"])),
        actions: [howTo("file-changed")],
      };
    case "socket_problem":
      return {
        ...base,
        title: text.socket(socketLabel(str("socket")), str("problem")),
        detail: text.socketDetail(str("address")),
        actions: [
          howTo("socket-problem"),
          page(text.pages.network, undefined, "network"),
        ],
      };
    case "socket_silent": {
      const vision = p["socket"] === "vision";

      return {
        ...base,
        title: vision ? text.visionSilent : text.gcSilent,
        detail: text.silentDetail(str("address"), p["everHeard"] === true),
        actions: [
          howTo(vision ? "vision-silent" : "gc-silent"),
          page(text.pages.network, undefined, "network"),
        ],
      };
    }
    case "heights_unreadable":
      return {
        ...base,
        title: text.heightsUnreadable(str("file")),
        detail: str("error"),
        actions: [howTo("heights-unreadable")],
      };
    case "team_default_height": {
      const color =
        p["color"] === "yellow"
          ? overviewText.teams.yellow
          : overviewText.teams.blue;
      const team = str("team");
      const issue = p["issue"] as keyof typeof overviewText.teams.issue;

      return {
        ...base,
        title: team
          ? text.teamDefaultHeight(color, team)
          : text.colorDefaultHeight(color),
        detail: overviewText.teams.issue[issue],
        actions: [
          howTo("team-height-missing"),
          page(text.pages.overview, undefined, "overview"),
        ],
      };
    }
    case "loopback_multicast_off":
      return {
        ...base,
        title: text.loopback(str("interface")),
        detail: text.loopbackDetail,
        actions: [howTo("loopback-multicast-off")],
      };
  }

  // A camera warning from the host's camera status, or a code this GUI
  // doesn't know yet: the host's own message carries it.
  const doc = CAMERA_WARNING_DOCS[a.code];

  return {
    ...base,
    title:
      a.cameraId === undefined
        ? a.code
        : text.cameraWarning(a.cameraId, a.code),
    detail: a.message,
    actions: [
      ...(doc ? [howTo(doc)] : []),
      ...(a.cameraId === undefined
        ? []
        : [page(text.pages.geometry, a.cameraId, "geometry")]),
    ],
  };
}

function socketLabel(kind: string): string {
  return kind === "vision" ? text.socketLabels.vision : text.socketLabels.gc;
}

export function alerts(): Alert[] {
  return hostAlerts.list.map(present);
}

export function alertsOf(severity: Severity): Alert[] {
  return alerts().filter((a) => a.severity === severity);
}
