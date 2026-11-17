// The host's alerts (gui/internal/alerts), from the alerts.state topic: which
// problems exist, how serious, and the facts behind each. alerts.ts words and
// links them.
import { topic } from "../wrapper-bus";
import type { HostAlert } from "../apiTypes";

export type { HostAlert, Severity } from "../apiTypes";

export const hostAlerts = $state<{ list: HostAlert[] }>({ list: [] });

$effect.root(() => {
  $effect(() => {
    return topic<HostAlert[]>("alerts.state").subscribe((list) => {
      if (list) hostAlerts.list = list;
    });
  });
});
