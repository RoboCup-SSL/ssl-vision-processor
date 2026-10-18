// Shared fetch helpers, used by every module that owns request state
// (geometry.svelte.ts, App.svelte's snapshot poll):
// the same "set a loading/saving flag, clear the error, run the request,
// catch and format any failure, always clear the flag" and "throw the
// backend's own error text on a non-2xx response" shapes were previously
// hand-repeated at every call site.

// Runs fn with setLoading(true) for its duration. Clears the error first;
// sets it (formatted the same way everywhere) if fn throws.
export async function withLoadingState(
  setLoading: (loading: boolean) => void,
  setError: (message: string | null) => void,
  fn: () => Promise<void>,
): Promise<void> {
  setLoading(true);
  setError(null);

  try {
    await fn();
  } catch (err) {
    setError(err instanceof Error ? err.message : String(err));
  } finally {
    setLoading(false);
  }
}

import type { ApiError } from "./apiTypes";

export type { ApiError };

// The host's error for a non-2xx response, as text for a banner: each issue
// on its own line when there are several.
export async function errorMessage(response: Response): Promise<string> {
  const text = await response.text();

  try {
    const body = JSON.parse(text) as ApiError;

    return body.issues?.join("\n") ?? body.message;
  } catch {
    return text || `${String(response.status)} ${response.statusText}`;
  }
}

// fetch that throws the host's error message on a non-2xx status.
export async function requestJSON(
  url: string,
  init?: RequestInit,
): Promise<Response> {
  const response = await fetch(url, init);
  if (!response.ok) {
    throw new Error(await errorMessage(response));
  }

  return response;
}
