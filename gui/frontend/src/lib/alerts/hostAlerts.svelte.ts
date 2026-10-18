// The host's alerts (gui/internal/alerts), from the alerts.state topic: which
// problems exist, how serious, and the facts behind each. alerts.ts words and
// links them.
import { topic } from "../wrapper-bus";

export type Severity = "error" | "warning" | "caution";

export interface HostAlert {
  id: string;
  code: string;
  severity: Severity;
  cameraId?: number;
  // The facts the alert is about, named per code.
  params?: Record<string, unknown>;
  // The host's plain English summary.
  message: string;
}

export const hostAlerts = $state<{ list: HostAlert[] }>({ list: [] });

$effect.root(() => {
  $effect(() => {
    return topic<HostAlert[]>("alerts.state").subscribe((list) => {
      if (list) hostAlerts.list = list;
    });
  });
});
