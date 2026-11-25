// The WebSocket to the host: what it publishes, by topic (the host's
// cmd/.../topics.go lists them), and whether it's connected.
import { createSubscriber } from "svelte/reactivity";
import { apiPath } from "./api";
import type { Topic, TopicMap } from "./apiTypes";

export type ConnectionState = "connecting" | "open" | "closed";

// Same-origin: the Go host serves /ws itself, alongside /api and the frontend.
// wss:// when the page itself was loaded over https.
const DEFAULT_URL = `${location.protocol === "https:" ? "wss" : "ws"}://${location.host}${apiPath("/ws")}`;
const BACKOFF_INITIAL_MS = 1_000;
// Low, since the host is on the local network: a retry every few seconds
// costs nothing, and the page comes back soon after a host restart.
const BACKOFF_MAX_MS = 5_000;

// The connection, reactive.
export const connection = $state<{ state: ConnectionState }>({
  state: "connecting",
});

interface ClientMessage {
  action: "subscribe" | "unsubscribe";
  topic: string;
}

// An error answers a message the host couldn't act on (an unknown topic, for
// one), in the same shape as the HTTP API's errors.
type ServerMessage =
  | { topic: string; data: unknown }
  | { error: string; message: string; topic?: string };

class Bus {
  private socket: WebSocket | null = null;
  private topicListeners = new Map<string, Set<(value: unknown) => void>>();
  private latest = new Map<string, unknown>();
  private backoffMs = BACKOFF_INITIAL_MS;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  constructor(private readonly url: string) {
    this.connect();
  }

  subscribeTopic(
    topic: string,
    listener: (value: unknown) => void,
  ): () => void {
    let listeners = this.topicListeners.get(topic);
    if (!listeners) {
      // Not reactive: nothing renders from the listeners themselves.
      // eslint-disable-next-line svelte/prefer-svelte-reactivity
      listeners = new Set();
      this.topicListeners.set(topic, listeners);
      this.send({ action: "subscribe", topic });
    }
    listeners.add(listener);
    const cached = this.latest.get(topic);
    if (cached !== undefined) listener(cached);
    return () => {
      const set = this.topicListeners.get(topic);
      if (!set) return;
      set.delete(listener);
      if (set.size === 0) {
        this.topicListeners.delete(topic);
        this.latest.delete(topic);
        this.send({ action: "unsubscribe", topic });
      }
    };
  }

  private connect(): void {
    connection.state = "connecting";
    const socket = new WebSocket(this.url);
    this.socket = socket;

    socket.addEventListener("open", () => {
      connection.state = "open";
      this.backoffMs = BACKOFF_INITIAL_MS;
      for (const topic of this.topicListeners.keys()) {
        this.send({ action: "subscribe", topic });
      }
    });

    socket.addEventListener("message", (event) => {
      this.onMessage(event.data);
    });

    socket.addEventListener("close", () => {
      connection.state = "closed";
      this.socket = null;
      this.scheduleReconnect();
    });

    socket.addEventListener("error", () => {
      socket.close();
    });
  }

  private scheduleReconnect(): void {
    if (this.reconnectTimer !== null) return;
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.backoffMs = Math.min(this.backoffMs * 2, BACKOFF_MAX_MS);
      this.connect();
    }, this.backoffMs);
  }

  private send(message: ClientMessage): void {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify(message));
    }
  }

  private onMessage(raw: unknown): void {
    if (typeof raw !== "string") return;
    let parsed: ServerMessage;
    try {
      parsed = JSON.parse(raw) as ServerMessage;
    } catch {
      console.warn("bus: malformed frame", raw);
      return;
    }
    if ("error" in parsed) {
      console.warn("bus: server error", parsed);
      return;
    }
    const listeners = this.topicListeners.get(parsed.topic);
    if (!listeners) return;
    this.latest.set(parsed.topic, parsed.data);
    for (const listener of listeners) listener(parsed.data);
  }
}

const bus = new Bus(DEFAULT_URL);

// Calls fn with each message on a topic, starting with the latest one if it's
// known, until the returned function is called.
export function onTopic<K extends Topic>(
  name: K,
  fn: (data: TopicMap[K]) => void,
): () => void {
  return bus.subscribeTopic(name, (data) => {
    fn(data as TopicMap[K]);
  });
}

// A topic's latest message as current, null before the first. The host sends
// the topic only while an effect or template reads it.
export function topicState<K extends Topic>(
  name: K,
): { readonly current: TopicMap[K] | null } {
  let value: TopicMap[K] | null = null;

  const track = createSubscriber((update) => {
    // A message known already arrives within onTopic, before anyone has read
    // the value: nothing to update.
    let started = false;

    const stop = onTopic(name, (data) => {
      value = data;
      if (started) update();
    });
    started = true;

    return () => {
      stop();
      value = null;
    };
  });

  return {
    get current() {
      track();

      return value;
    },
  };
}
