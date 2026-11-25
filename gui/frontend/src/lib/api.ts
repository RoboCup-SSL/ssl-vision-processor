// The host's HTTP API, typed by its OpenAPI spec (generated/api.ts): a wrong
// path, method, parameter or body doesn't compile.
import createClient from "openapi-fetch";
import type { ApiError } from "./apiTypes";
import type { paths } from "./generated/api";

// Call it as api.GET("/api/config"), api.POST("/api/config/load", { body }).
// It rejects only when the host doesn't answer; unwrap also throws for an
// error answer.
export const api = createClient<paths>({
  baseUrl: location.origin,
  // Looked up per call, so tests can replace it.
  fetch: (request) => globalThis.fetch(request),
});

// What an error answer carries: the host's error body, or the text of
// something in between, such as a proxy.
type ErrorAnswer = ApiError | string | undefined;

// The host's error, as text for a banner: each issue on its own line when
// there are several.
export function errorMessage(error: ErrorAnswer, response: Response): string {
  if (typeof error === "object") {
    return error.issues?.join("\n") ?? error.message;
  }

  if (error) return error;

  return `${String(response.status)} ${response.statusText}`;
}

// The data of a call's answer, throwing the host's error message if it
// answered with one.
export async function unwrap<T>(
  call: Promise<{ data?: T; error?: ErrorAnswer; response: Response }>,
): Promise<T> {
  const { data, error, response } = await call;
  if (!response.ok) throw new Error(errorMessage(error, response));

  return data as T;
}

// One of the spec's paths with its {parameters} filled in, for what isn't
// fetched here: an image's src, a WebSocket.
export function apiPath(
  path: keyof paths,
  params: Record<string, string | number> = {},
): string {
  return path.replace(/\{(\w+)\}/g, (_, name: string) => {
    const value = params[name];
    // text-ok: an internal invariant, not shown to users.
    if (value === undefined) throw new Error(`${path}: no ${name}`);

    return encodeURIComponent(String(value));
  });
}
