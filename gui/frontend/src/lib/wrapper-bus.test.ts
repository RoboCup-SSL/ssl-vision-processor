import { get } from "svelte/store";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// test-setup.ts mocks this module for everyone else; here it's the real one,
// against a WebSocket stand-in the test drives by hand.
type Bus = typeof import("./wrapper-bus");

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

async function loadBus(): Promise<Bus> {
  return vi.importActual<Bus>("./wrapper-bus");
}

beforeEach(() => {
  FakeSocket.all = [];
  vi.useFakeTimers();
  vi.stubGlobal("WebSocket", FakeSocket);
  vi.resetModules();
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("wrapper-bus", () => {
  it("connects to /ws on the page's own host", async () => {
    await loadBus();

    expect(latest().url).toBe(`ws://${location.host}/ws`);
  });

  it("subscribes on open to topics asked for before the socket opened", async () => {
    const bus = await loadBus();
    const unsubscribe = bus.topic("config.state").subscribe(noop);

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

    const a = bus.topic("alerts.state").subscribe(noop);
    const b = bus.topic("alerts.state").subscribe(noop);

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
    const stopFirst = bus
      .topic("network.state")
      .subscribe((v) => first.push(v));
    latest().receive({ topic: "network.state", data: { n: 1 } });
    latest().receive({ topic: "other.state", data: "ignored" });

    const late = bus.topic("network.state");
    expect(get(late)).toEqual({ n: 1 });
    expect(first).toEqual([null, { n: 1 }]);

    stopFirst();
  });

  it("forgets a topic's value once nobody listens", async () => {
    const bus = await loadBus();
    latest().open();

    const stop = bus.topic("processes.state").subscribe(noop);
    latest().receive({ topic: "processes.state", data: [1] });
    stop();

    expect(get(bus.topic("processes.state"))).toBeNull();
  });

  it("ignores malformed frames, error answers and binary data", async () => {
    const bus = await loadBus();
    latest().open();
    const warn = vi.spyOn(console, "warn").mockImplementation(noop);

    const seen: unknown[] = [];
    const stop = bus.topic("config.state").subscribe((v) => seen.push(v));
    latest().receive("{not json");
    latest().receive({
      error: "unknown_topic",
      message: "no such topic",
      topic: "x",
    });
    latest().emit("message", { data: new ArrayBuffer(4) });

    expect(seen).toEqual([null]);
    expect(warn).toHaveBeenCalledTimes(2);

    stop();
    warn.mockRestore();
  });

  it("reports the connection state", async () => {
    const bus = await loadBus();
    const states: string[] = [];
    const stop = bus.connectionState.subscribe((s) => states.push(s));

    latest().open();
    latest().close();

    expect(states).toEqual(["connecting", "open", "closed"]);
    stop();
  });

  it("reconnects with backoff and resubscribes every topic", async () => {
    const bus = await loadBus();
    const stopA = bus.topic("a.state").subscribe(noop);
    const stopB = bus.topic("b.state").subscribe(noop);
    latest().open();

    latest().close();
    expect(FakeSocket.all).toHaveLength(1);

    vi.advanceTimersByTime(999);
    expect(FakeSocket.all).toHaveLength(1);
    vi.advanceTimersByTime(1);
    expect(FakeSocket.all).toHaveLength(2);

    latest().open();
    expect(latest().sent).toEqual([
      { action: "subscribe", topic: "a.state" },
      { action: "subscribe", topic: "b.state" },
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
