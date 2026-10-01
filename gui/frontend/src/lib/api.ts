// Shared fetch helpers, used by every module that owns request state
// (geometry.svelte.ts, lineCorners.svelte.ts, App.svelte's snapshot poll):
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

// fetch that throws the backend's own response body as the Error message on
// a non-2xx status, instead of every caller re-deriving that (or, worse,
// just the numeric status code) from scratch.
export async function requestJSON(
  url: string,
  init?: RequestInit,
): Promise<Response> {
  const response = await fetch(url, init);
  if (!response.ok) {
    throw new Error(await response.text());
  }

  return response;
}
