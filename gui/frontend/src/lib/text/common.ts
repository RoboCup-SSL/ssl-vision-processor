// Phrases more than one page uses.

export const PLACEHOLDER_RANGE =
  "Placeholder range until vision_processor reports the camera's.";

export const NO_HOST = "(no host)";

// Shown on a per-camera page when no camera is selected.
export const SELECT_INSTANCE = "Select a vision processor on the left first.";

// A live number padded on the left to width characters with figure spaces,
// each as wide as a digit. Shown with tabular-nums (every digit the same
// width too), a value that changes many times a second keeps its width as its
// digit count changes, so the text after it holds still and stays readable.
export function steady(value: string, width: number): string {
  return value.padStart(width, "\u2007");
}

// "1 camera", "2 cameras": the word as is for 1, with an s otherwise.
export function plural(n: number, word: string): string {
  return n === 1 ? word : `${word}s`;
}
