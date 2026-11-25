// The host's alerts (gui/internal/alerts), from the alerts.state topic: which
// problems exist, how serious, and the facts behind each. alerts.ts words and
// links them.
import { onTopic } from "../bus.svelte";
import type { HostAlert } from "../apiTypes";

export type { HostAlert, Severity } from "../apiTypes";

export const hostAlerts = $state<{ list: HostAlert[] }>({ list: [] });

$effect.root(() => {
  $effect(() => {
    return onTopic("alerts.state", (list) => {
      hostAlerts.list = list;
    });
  });
});
