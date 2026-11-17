// The host's log for the log panel (LogPanel.svelte), from GET /api/logs:
// long-polled while the panel is open, at the panel's own level, so the
// host's console level doesn't matter.

import { requestJSON } from "../api";
import type { LogLevel, LogRecord, LogsResponse } from "../apiTypes";

// Records kept in this tab, and asked for when the panel opens.
const KEEP = 1000;
// Seconds each request waits for a record before answering empty.
const WAIT = 25;
// After a failed request, before trying again.
const RETRY_MS = 2000;

export const hostLog = $state<{
  // Records were logged that this tab never got.
  missed: boolean;
  error: string | null;
  paused: boolean;
  // Records logged while paused, not shown yet.
  waiting: number;
}>({ missed: false, error: null, paused: false, waiting: 0 });

// Raw, not deep state: records are only ever replaced, never edited, and a
// thousand proxied records would cost for nothing.
let records = $state.raw<LogRecord[]>([]);
let pending: LogRecord[] = [];

// The records shown, oldest first.
export function logRecords(): LogRecord[] {
  return records;
}

function keep(list: LogRecord[]): LogRecord[] {
  return list.length > KEEP ? list.slice(list.length - KEEP) : list;
}

function add(page: LogRecord[]): void {
  if (page.length === 0) return;

  if (hostLog.paused) {
    pending = keep([...pending, ...page]);
    hostLog.waiting = pending.length;
  } else {
    records = keep([...records, ...page]);
  }
}

export function setPaused(paused: boolean): void {
  hostLog.paused = paused;

  if (!paused) {
    records = keep([...records, ...pending]);
    pending = [];
    hostLog.waiting = 0;
  }
}

export function clearLog(): void {
  records = [];
  pending = [];
  hostLog.waiting = 0;
  hostLog.missed = false;
}

function sleep(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    const id = setTimeout(resolve, ms);
    signal.addEventListener(
      "abort",
      () => {
        clearTimeout(id);
        resolve();
      },
      { once: true },
    );
  });
}

// Follows the log at level and up, starting with the last KEEP records, until
// the returned function is called. Calling it again (a new level) starts over.
export function followLog(level: LogLevel): () => void {
  const controller = new AbortController();
  const { signal } = controller;
  // A function, as TypeScript keeps signal.aborted narrowed to false across
  // the awaits below.
  const stopped = (): boolean => signal.aborted;

  clearLog();
  hostLog.error = null;

  void (async () => {
    let after = 0;
    let first = true;

    while (!stopped()) {
      // The first request answers at once, with what's already logged.
      const wait = first ? "" : `&wait=${String(WAIT)}`;
      const url = `/api/logs?after=${String(after)}&level=${level}&limit=${String(KEEP)}${wait}`;

      try {
        const page = (await (
          await requestJSON(url, { signal })
        ).json()) as LogsResponse;
        if (stopped()) return;

        // The first page leaving older records out is expected.
        if (page.missed && !first) hostLog.missed = true;

        add(page.records);
        after = page.next;
        first = false;
        hostLog.error = null;
      } catch (err) {
        if (stopped()) return;

        hostLog.error = err instanceof Error ? err.message : String(err);
        await sleep(RETRY_MS, signal);
      }
    }
  })();

  return () => {
    controller.abort();
  };
}
