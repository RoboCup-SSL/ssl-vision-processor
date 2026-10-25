// The vision_processors the host runs itself (started with -vp), from the
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

export async function restartProcess(cameraId: number): Promise<void> {
  const response = await requestJSON(
    `/api/processes/${String(cameraId)}/restart`,
    { method: "POST" },
  );

  processes.list = (await response.json()) as ProcessStatus[];
}
