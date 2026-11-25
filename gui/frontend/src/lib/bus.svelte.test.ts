import { flushSync } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// test-setup.ts mocks this module for everyone else; here it's the real one,
// against a WebSocket stand-in the test drives by hand.
type Bus = typeof import("./bus.svelte");

class FakeSocket {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  static all: FakeSocket[] = [];

  readyState = FakeSocket.CONNECTING;
  sent: unknown[] = [];
  private listeners = new Map<string, ((event: unknown) => void)[]>();

  constructor(readonly url: string) {
    FakeSocket.all.push(this);
  }

  addEventListener(type: string, fn: (event: unknown) => void): void {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), fn]);
  }

  send(data: string): void {
    this.sent.push(JSON.parse(data));
  }

  close(): void {
    if (this.readyState === FakeSocket.CLOSED) return;
    this.readyState = FakeSocket.CLOSED;
    this.emit("close", {});
  }

  open(): void {
    this.readyState = FakeSocket.OPEN;
    this.emit("open", {});
  }

  receive(data: unknown): void {
    this.emit("message", {
      data: typeof data === "string" ? data : JSON.stringify(data),
    });
  }

  emit(type: string, event: unknown): void {
    for (const fn of this.listeners.get(type) ?? []) fn(event);
  }
}

function latest(): FakeSocket {
  const socket = FakeSocket.all.at(-1);
  if (!socket) throw new Error("no socket opened");
  return socket;
}

const noop = (): void => undefined;

let copies = 0;

async function loadBus(): Promise<Bus> {
  // A new query is a fresh copy of this module alone. vi.resetModules would
  // give it a Svelte of its own, whose signals this file's effects can't see.
  return vi.importActual<Bus>(`./bus.svelte.ts?copy=${String(++copies)}`);
}

beforeEach(() => {
  FakeSocket.all = [];
  vi.useFakeTimers();
  vi.stubGlobal("WebSocket", FakeSocket);
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("bus", () => {
  it("connects to /ws on the page's own host", async () => {
    await loadBus();

    expect(latest().url).toBe(`ws://${location.host}/ws`);
  });

  it("subscribes on open to topics asked for before the socket opened", async () => {
    const bus = await loadBus();
    const unsubscribe = bus.onTopic("config.state", noop);

    expect(latest().sent).toEqual([]);
    latest().open();
    expect(latest().sent).toEqual([
      { action: "subscribe", topic: "config.state" },
    ]);

    unsubscribe();
  });

  it("sends one subscribe per topic, however many listen", async () => {
    const bus = await loadBus();
    latest().open();

    const a = bus.onTopic("alerts.state", noop);
    const b = bus.onTopic("alerts.state", noop);

    expect(latest().sent).toEqual([
      { action: "subscribe", topic: "alerts.state" },
    ]);
    a();
    expect(latest().sent).toHaveLength(1);
    b();
    expect(latest().sent).toEqual([
      { action: "subscribe", topic: "alerts.state" },
      { action: "unsubscribe", topic: "alerts.state" },
    ]);
  });

  it("delivers a topic's data, and the latest value to a late subscriber", async () => {
    const bus = await loadBus();
    latest().open();

    const first: unknown[] = [];
    const stopFirst = bus.onTopic("network.state", (v) => first.push(v));
    latest().receive({ topic: "network.state", data: { n: 1 } });
    latest().receive({ topic: "other.state", data: "ignored" });

    const late: unknown[] = [];
    const stopLate = bus.onTopic("network.state", (v) => late.push(v));
    expect(late).toEqual([{ n: 1 }]);
    expect(first).toEqual([{ n: 1 }]);

    stopFirst();
    stopLate();
  });

  it("forgets a topic's value once nobody listens", async () => {
    const bus = await loadBus();
    latest().open();

    const stop = bus.onTopic("processes.state", noop);
    latest().receive({ topic: "processes.state", data: [1] });
    stop();

    const seen: unknown[] = [];
    bus.onTopic("processes.state", (v) => seen.push(v))();
    expect(seen).toEqual([]);
  });

  it("ignores malformed frames, error answers and binary data", async () => {
    const bus = await loadBus();
    latest().open();
    const warn = vi.spyOn(console, "warn").mockImplementation(noop);

    const seen: unknown[] = [];
    const stop = bus.onTopic("config.state", (v) => seen.push(v));
    latest().receive("{not json");
    latest().receive({
      error: "unknown_topic",
      message: "no such topic",
      topic: "x",
    });
    latest().emit("message", { data: new ArrayBuffer(4) });

    expect(seen).toEqual([]);
    expect(warn).toHaveBeenCalledTimes(2);

    stop();
    warn.mockRestore();
  });

  it("reports the connection state", async () => {
    const bus = await loadBus();
    const states: string[] = [];
    const stop = $effect.root(() => {
      $effect(() => {
        states.push(bus.connection.state);
      });
    });

    flushSync();
    latest().open();
    flushSync();
    latest().close();
    flushSync();

    expect(states).toEqual(["connecting", "open", "closed"]);
    stop();
  });

  it("topicState follows a topic while something reads it", async () => {
    const bus = await loadBus();
    latest().open();

    const state = bus.topicState("network.state");
    expect(state.current).toBeNull();
    expect(latest().sent).toEqual([]);

    const seen: unknown[] = [];
    const read = () =>
      $effect.root(() => {
        const value = $derived(state.current);
        $effect(() => {
          seen.push(value);
        });
      });

    const stop = read();
    flushSync();
    latest().receive({ topic: "network.state", data: { n: 1 } });
    flushSync();
    latest().receive({ topic: "network.state", data: { n: 2 } });
    flushSync();
    expect(seen).toEqual([null, { n: 1 }, { n: 2 }]);

    // Another reader of the topic gets the latest at once.
    const other = bus.topicState("network.state");
    const stopOther = $effect.root(() => {
      $effect(() => {
        seen.push(other.current);
      });
    });
    flushSync();
    expect(seen.at(-1)).toEqual({ n: 2 });
    expect(seen).toHaveLength(4);

    // Nobody reads it any more: unsubscribed, and forgotten.
    stop();
    stopOther();
    await vi.advanceTimersByTimeAsync(0);
    expect(latest().sent).toEqual([
      { action: "subscribe", topic: "network.state" },
      { action: "unsubscribe", topic: "network.state" },
    ]);
    expect(state.current).toBeNull();
  });

  it("reconnects with backoff and resubscribes every topic", async () => {
    const bus = await loadBus();
    const stopA = bus.onTopic("alerts.state", noop);
    const stopB = bus.onTopic("network.state", noop);
    latest().open();

    latest().close();
    expect(FakeSocket.all).toHaveLength(1);

    vi.advanceTimersByTime(999);
    expect(FakeSocket.all).toHaveLength(1);
    vi.advanceTimersByTime(1);
    expect(FakeSocket.all).toHaveLength(2);

    latest().open();
    expect(latest().sent).toEqual([
      { action: "subscribe", topic: "alerts.state" },
      { action: "subscribe", topic: "network.state" },
    ]);

    stopA();
    stopB();
  });

  it("doubles the retry delay up to 5 s, and resets it after a connection", async () => {
    await loadBus();
    const delays: number[] = [];

    for (let i = 0; i < 5; i++) {
      const count = FakeSocket.all.length;
      latest().close();
      let waited = 0;
      while (FakeSocket.all.length === count) {
        vi.advanceTimersByTime(100);
        waited += 100;
      }
      delays.push(waited);
    }
    expect(delays).toEqual([1000, 2000, 4000, 5000, 5000]);

    latest().open();
    const count = FakeSocket.all.length;
    latest().close();
    vi.advanceTimersByTime(1000);
    expect(FakeSocket.all).toHaveLength(count + 1);
  });

  it("closes the socket on an error, which schedules a reconnect", async () => {
    await loadBus();
    latest().open();

    latest().emit("error", {});

    expect(latest().readyState).toBe(FakeSocket.CLOSED);
    vi.advanceTimersByTime(1000);
    expect(FakeSocket.all).toHaveLength(2);
  });
});
