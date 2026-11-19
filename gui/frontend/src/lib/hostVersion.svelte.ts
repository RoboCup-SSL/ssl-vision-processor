// Notices the host being upgraded under an open page: the bundle comes from
// the host's own binary, so the version it reported when this page connected
// is the bundle's. A different one after a reconnect means the page is stale.
import { requestJSON } from "./api";
import type { HealthResponse } from "./apiTypes";
import { connectionState } from "./wrapper-bus";

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
    const response = await requestJSON("/api/health");
    const { version } = (await response.json()) as HealthResponse;
    hostVersion.page ??= version;
    hostVersion.host = version;
  } catch {
    // Unreachable again; the next reconnect checks.
  }
}

$effect.root(() => {
  $effect(() => {
    return connectionState.subscribe((state) => {
      if (state === "open") void checkHostVersion();
    });
  });
});
