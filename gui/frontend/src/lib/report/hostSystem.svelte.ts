// The host's GET /api/system, shared by the About dialog and the settings
// menu's issue link. Loaded again each time either opens: a restarted host
// may be a different build.

import { api, unwrap } from "../api";
import type { SystemInfo } from "../apiTypes";

export const hostSystem = $state<{
  info: SystemInfo | null;
  error: string | null;
}>({ info: null, error: null });

export async function loadHostSystem(): Promise<void> {
  try {
    hostSystem.info = await unwrap(api.GET("/api/system"));
    hostSystem.error = null;
  } catch (err) {
    hostSystem.error = err instanceof Error ? err.message : String(err);
  }
}
