// Unit test setup: no host to talk to, so the WebSocket client is replaced
// by a closed connection whose topics never publish.
import { vi } from "vitest";

vi.mock("./lib/bus.svelte", () => ({
  connection: { state: "closed" },
  onTopic: () => () => undefined,
  topicState: () => ({ current: null }),
}));
