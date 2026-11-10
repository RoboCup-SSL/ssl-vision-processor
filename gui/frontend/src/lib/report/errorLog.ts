// The page's last few uncaught errors and rejected promises, for the system
// report. main.ts installs it before the app mounts: a report is usually
// made after something has already gone wrong, so it has to be listening
// from the start.

const LIMIT = 20;
// A stack is useful for its first frames; a long one would crowd out the rest
// of the report.
const STACK_LIMIT = 2000;

export interface LoggedError {
  time: string;
  message: string;
  stack?: string;
}

export const recentErrors: LoggedError[] = [];

export function recordError(reason: unknown, fallback?: string): void {
  const entry: LoggedError = {
    time: new Date().toISOString(),
    message: reason instanceof Error ? reason.message : String(reason),
  };

  const stack = reason instanceof Error ? reason.stack : fallback;
  if (stack) entry.stack = stack.slice(0, STACK_LIMIT);

  recentErrors.push(entry);
  if (recentErrors.length > LIMIT) recentErrors.shift();
}

export function installErrorLog(target: Window = window): void {
  target.addEventListener("error", (e) => {
    recordError(
      e.error ?? e.message,
      `${e.filename}:${String(e.lineno)}:${String(e.colno)}`,
    );
  });
  target.addEventListener("unhandledrejection", (e) => {
    recordError(e.reason);
  });
}
