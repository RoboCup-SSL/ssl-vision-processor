// Notices the host being upgraded under an open page: the bundle comes from
// the host's own binary, so the version it reported when this page connected
// is the bundle's. A different one after a reconnect means the page is stale.
import { untrack } from "svelte";
import { api, unwrap } from "./api";
import { connection } from "./bus.svelte";

export const hostVersion = $state<{
  // The version this page was loaded from.
  page: string | null;
  // What the host reports now.
  host: string | null;
}>({ page: null, host: null });

export function hostUpdated(): boolean {
  return (
    hostVersion.page !== null &&
    hostVersion.host !== null &&
    hostVersion.page !== hostVersion.host
  );
}

export async function checkHostVersion(): Promise<void> {
  try {
    const { version } = await unwrap(api.GET("/api/health"));
    hostVersion.page ??= version;
    hostVersion.host = version;
  } catch {
    // Unreachable again; the next reconnect checks.
  }
}

$effect.root(() => {
  $effect(() => {
    if (connection.state === "open") void untrack(checkHostVersion);
  });
});
