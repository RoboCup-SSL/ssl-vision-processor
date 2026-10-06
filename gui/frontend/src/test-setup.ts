// Unit test setup: no host to talk to, so the WebSocket client is replaced
// by topics that never publish.
import { readable } from "svelte/store";
import { vi } from "vitest";

vi.mock("./lib/wrapper-bus", () => ({
  connectionState: readable("closed"),
  topic: () => readable(null),
}));
