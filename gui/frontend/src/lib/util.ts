// Small helpers shared by the config modules.

// o without key, as a new object (vision.yml edits replace, never mutate).
export function without<T>(
  o: Record<string, T> | undefined,
  key: string,
): Record<string, T> {
  return Object.fromEntries(Object.entries(o ?? {}).filter(([k]) => k !== key));
}
