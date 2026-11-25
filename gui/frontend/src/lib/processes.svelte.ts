// The vision_processors the host runs itself (started with -runCameras), from the
// processes.state topic. Empty when the host runs none: then each one is
// started and restarted by hand.
import { topic } from "./wrapper-bus";
import { requestJSON } from "./api";
import type { ProcessStatus } from "./apiTypes";

export type { ProcessStatus };

export const processes = $state<{ list: ProcessStatus[] }>({ list: [] });

$effect.root(() => {
  $effect(() => {
    return topic<ProcessStatus[]>("processes.state").subscribe((list) => {
      if (list) processes.list = list;
    });
  });
});

export function processFor(cameraId: number): ProcessStatus | undefined {
  return processes.list.find((p) => p.cameraId === cameraId);
}

// Failed runs in a row before a process counts as crash-looping; the host's
// process_crashing alert uses the same (gui/internal/alerts crashingAfter).
export const CRASHING_AFTER = 2;

export type ProcessHealth =
  "running" | "starting" | "failing" | "held" | "noCamera";

// One word for a supervised process, for a status dot.
export function processHealth(p: ProcessStatus): ProcessHealth {
  if (p.state === "no_camera") return "noCamera";
  if (p.state === "conflict") return "held";
  if (p.crashes >= CRASHING_AFTER) return "failing";
  if (p.state === "running") return "running";

  return "starting";
}

// The last non-blank line it printed: usually why it exited.
export function lastLine(p: ProcessStatus): string {
  for (let i = p.output.length - 1; i >= 0; i--) {
    const line = p.output[i]?.trim();
    if (line) return line;
  }

  return "";
}

export async function restartProcess(cameraId: number): Promise<void> {
  const response = await requestJSON(
    `/api/processes/${String(cameraId)}/restart`,
    { method: "POST" },
  );

  processes.list = (await response.json()) as ProcessStatus[];
}
