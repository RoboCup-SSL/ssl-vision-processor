// The Live Field's sources: the vision_processors' raw detections (one
// source, made of every camera heard) and each autoref's tracker, found by
// what the host hears on the tracker group.
import type { SSL_DetectionFrameJson } from "../../proto/vision/ssl_vision_detection_pb";
import type {
  LiveFieldCamera,
  LiveFieldSnapshot,
  LiveFieldTracker,
} from "../apiTypes";
import { fromDetection, fromTracked, type Layer } from "./model";

export const VISION = "vision";

export type SourceKind = "vision" | "tracker";

export interface Source {
  key: string;
  kind: SourceKind;
  // The autoref's own name, or its uuid if it sends none. Empty for vision.
  name: string;
  receiving: boolean;
  fps: number;
  // Since the host last heard it, by the host's clock.
  ageMs: number | null;
}

// Keyed by name rather than uuid, which an autoref picks anew each start, so
// a view setting for "TIGERs AutoRef" survives its restart.
export function trackerKey(t: LiveFieldTracker): string {
  return `tracker:${t.sourceName || t.uuid}`;
}

function age(snapshot: LiveFieldSnapshot, receivedAt: string): number {
  return Math.max(
    0,
    new Date(snapshot.at).getTime() - new Date(receivedAt).getTime(),
  );
}

// Vision first, always, then every autoref heard.
export function sourcesOf(snapshot: LiveFieldSnapshot | null): Source[] {
  const cameras = snapshot?.cameras ?? [];
  const ages = cameras.map((c) => (snapshot ? age(snapshot, c.receivedAt) : 0));

  const vision: Source = {
    key: VISION,
    kind: "vision",
    name: "",
    receiving: cameras.some((c) => c.receiving),
    fps: Math.max(0, ...cameras.map((c) => c.fps)),
    ageMs: ages.length > 0 ? Math.min(...ages) : null,
  };

  const trackers = (snapshot?.trackers ?? []).map(
    (t): Source => ({
      key: trackerKey(t),
      kind: "tracker",
      name: t.sourceName || t.uuid,
      receiving: t.receiving,
      fps: t.fps,
      ageMs: snapshot ? age(snapshot, t.receivedAt) : null,
    }),
  );

  // Two autorefs sending the same name share a key; keep the first.
  const unique = trackers.filter(
    (t, i) => trackers.findIndex((u) => u.key === t.key) === i,
  );

  return [vision, ...unique];
}

// One drawable layer; camera is set for vision's, one per camera.
export interface SourceLayer {
  layer: Layer;
  camera?: number;
}

export function layersOf(
  snapshot: LiveFieldSnapshot | null,
  key: string,
  hiddenCameras: readonly number[],
): SourceLayer[] {
  if (!snapshot) return [];

  if (key === VISION) {
    return snapshot.cameras.flatMap((c) =>
      c.frame && !hiddenCameras.includes(c.cameraId)
        ? [
            {
              layer: fromDetection(c.frame as SSL_DetectionFrameJson),
              camera: c.cameraId,
            },
          ]
        : [],
    );
  }

  const tracker = snapshot.trackers.find((t) => trackerKey(t) === key);

  return tracker?.frame ? [{ layer: fromTracked(tracker.frame) }] : [];
}

// A camera's processing time: capture to send, both by the
// vision_processor's own clock, in ms.
export function processingMs(camera: LiveFieldCamera): number | null {
  const frame: SSL_DetectionFrameJson | undefined = camera.frame;
  const sent = Number(frame?.tSent);
  const captured = Number(frame?.tCapture);

  return Number.isFinite(sent) && Number.isFinite(captured)
    ? (sent - captured) * 1000
    : null;
}
