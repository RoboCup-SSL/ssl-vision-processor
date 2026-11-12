import { beforeEach, describe, expect, it } from "vitest";
import { config, type ConfigDocument } from "./config.svelte";
import type { HostInterface, NetworkState } from "./apiTypes";
import {
  addressAdvice,
  addressPresets,
  headerSockets,
  interfaceSelection,
  ipv4Error,
  networkFromDoc,
  portAdvice,
  portError,
  portPresets,
  STANDARD,
  writeInterfaces,
  writeNetwork,
} from "./network.svelte";
import { schema, type Schema } from "./schema.svelte";
import { network as text } from "./text/network";

function docOf(): ConfigDocument {
  if (!config.doc) throw new Error("no document loaded");
  return config.doc;
}

const FALLBACKS = {
  vision_ip: "224.5.23.2",
  vision_port: 10006,
  gc_ip: "224.5.23.1",
  gc_port: 10003,
};

function iface(name: string, broadcast: string, used = true): HostInterface {
  return {
    name,
    address: "192.168.1.5",
    broadcast,
    used,
  } as unknown as HostInterface;
}

function netState(extra: Partial<NetworkState> = {}): NetworkState {
  return { interfaces: [], ...extra } as unknown as NetworkState;
}

beforeEach(() => {
  schema.value = {
    settings: Object.fromEntries(
      Object.entries(FALLBACKS).map(([k, v]) => [
        `network.${k}`,
        { default: v },
      ]),
    ),
    drivers: [],
  } as unknown as Schema;
  config.doc = { cameras: [] } as unknown as ConfigDocument;
});

describe("the network block", () => {
  it("falls back to vision_processor's defaults and the standard tracker group", () => {
    expect(networkFromDoc()).toEqual({
      ...FALLBACKS,
      tracker_ip: "224.5.23.2",
      tracker_port: 10010,
    });
  });

  it("reads what vision.yml sets, ignoring values of the wrong type", () => {
    docOf().defaults = {
      network: { vision_ip: "239.1.2.3", vision_port: "10020", gc_port: 11003 },
    };
    docOf().tracker = { ip: "239.9.9.9", port: 11010 };

    expect(networkFromDoc()).toEqual({
      vision_ip: "239.1.2.3",
      vision_port: 10006,
      gc_ip: "224.5.23.1",
      gc_port: 11003,
      tracker_ip: "239.9.9.9",
      tracker_port: 11010,
    });
  });

  it("writes vision and game controller keys into defaults.network, keeping the rest", () => {
    docOf().defaults = { network: { gc_ip: "239.0.0.1" }, other: 1 };

    writeNetwork("vision_port", 10016);

    expect(docOf().defaults).toEqual({
      network: { gc_ip: "239.0.0.1", vision_port: 10016 },
      other: 1,
    });
  });

  it("creates defaults when vision.yml has none", () => {
    writeNetwork("gc_ip", "239.0.0.2");

    expect(docOf().defaults).toEqual({ network: { gc_ip: "239.0.0.2" } });
  });

  it("writes the tracker into its own block", () => {
    docOf().tracker = { port: 10010 };

    writeNetwork("tracker_ip", "239.0.0.3");

    expect(docOf().tracker).toEqual({ ip: "239.0.0.3", port: 10010 });
    expect(docOf().defaults).toBeUndefined();
  });

  it("writes nothing before the document has loaded", () => {
    config.doc = null;

    writeNetwork("vision_ip", "239.0.0.4");
    writeInterfaces(false, ["eth0"]);

    expect(config.doc).toBeNull();
  });
});

describe("interfaces", () => {
  it("are automatic with no skip list by default", () => {
    expect(interfaceSelection()).toEqual({ auto: true, skip: [] });
  });

  it("write a skip list only when there is one", () => {
    writeInterfaces(false, ["docker0"]);
    expect(interfaceSelection()).toEqual({ auto: false, skip: ["docker0"] });

    writeInterfaces(true, []);
    expect(docOf().host?.interfaces).toEqual({ auto: true });
  });
});

describe("headerSockets", () => {
  it("shows the configured addresses until the host reports", () => {
    const sockets = headerSockets();

    expect(sockets.map((s) => s.fallback)).toEqual([
      "224.5.23.2:10006",
      "224.5.23.1:10003",
      "224.5.23.2:10010",
    ]);
    expect(sockets.every((s) => s.status === undefined)).toBe(true);
  });

  it("has no fallback before the document loads", () => {
    config.doc = null;

    expect(headerSockets().map((s) => s.fallback)).toEqual([
      undefined,
      undefined,
      undefined,
    ]);
  });
});

describe("format checks", () => {
  it.each([
    ["224.5.23.2", null],
    ["0.0.0.0", text.errors.zeroAddress],
    ["256.1.1.1", text.errors.notIPv4],
    ["1.2.3", text.errors.notIPv4],
    ["1.2.3.4.5", text.errors.notIPv4],
    ["a.b.c.d", text.errors.notIPv4],
    ["1.2.3.1234", text.errors.notIPv4],
    ["", text.errors.notIPv4],
  ])("ipv4Error(%j)", (ip, want) => {
    expect(ipv4Error(ip)).toBe(want);
  });

  it.each([
    [1, null],
    [65535, null],
    [0, text.errors.portRange],
    [65536, text.errors.portRange],
    [10006.5, text.errors.portRange],
    [Number.NaN, text.errors.portRange],
  ])("portError(%j)", (port, want) => {
    expect(portError(port)).toBe(want);
  });
});

describe("presets", () => {
  it("offer the standard group, broadcast, and each used interface's broadcast once", () => {
    const presets = addressPresets("vision", [
      iface("eth0", "192.168.1.255"),
      iface("eth1", "192.168.1.255"),
      iface("wlan0", "10.0.0.255", false),
      iface("tun0", ""),
    ]);

    expect(presets.map((p) => p.value)).toEqual([
      STANDARD.vision.ip,
      "255.255.255.255",
      "192.168.1.255",
    ]);
  });

  it("offer the standard, backup and, for vision only, legacy ports", () => {
    expect(portPresets("vision").map((p) => p.value)).toEqual([
      10006, 10016, 10005,
    ]);
    expect(portPresets("gc").map((p) => p.value)).toEqual([10003, 10013]);
  });
});

describe("addressAdvice", () => {
  const advise = (
    ip: string,
    kind: "vision" | "gc" | "tracker" = "vision",
    state: NetworkState | null = null,
  ) => addressAdvice(ip, kind, state);

  it("is silent for the standard group and for malformed input", () => {
    expect(advise("224.5.23.2")).toEqual({ warnings: [], notes: [] });
    expect(advise("not an ip")).toEqual({ warnings: [], notes: [] });
  });

  it("warns about loopback, reserved and source-specific groups", () => {
    expect(advise("127.0.0.1").warnings).toEqual([text.address.loopback]);
    expect(advise("224.0.0.5").warnings).toEqual([text.address.reserved]);
    expect(advise("232.1.1.1").warnings).toEqual([text.address.sourceSpecific]);
  });

  it("notes another kind's standard group, but not one this kind shares", () => {
    expect(advise("224.5.23.1", "vision").notes).toContain(
      text.address.otherStandard(text.kind.gc),
    );
    expect(advise("224.5.23.2", "tracker").notes).toEqual([]);
  });

  it("notes a non-standard group, and one sharing a MAC with a standard group", () => {
    expect(advise("239.5.23.2").notes).toEqual([
      text.address.nonStandard,
      text.address.sameMac("224.5.23.2"),
    ]);
    expect(advise("239.255.1.1").notes).toEqual([text.address.nonStandard]);
  });

  it("notes broadcast to all, or on a known interface", () => {
    expect(advise("255.255.255.255").notes).toEqual([
      text.address.broadcastAll,
    ]);

    const state = netState({ interfaces: [iface("eth0", "192.168.1.255")] });
    expect(advise("192.168.1.255", "vision", state).notes).toEqual([
      text.address.broadcastOn("eth0"),
    ]);
  });

  it("warns about an unknown broadcast address and about unicast", () => {
    expect(advise("10.1.1.255").warnings).toEqual([
      text.address.unknownBroadcast,
    ]);
    expect(advise("10.1.1.7").warnings).toEqual([text.address.unicast]);
  });
});

describe("portAdvice", () => {
  it("is silent for the standard port, and for an invalid one", () => {
    expect(portAdvice(10006, "vision", null)).toEqual({
      warnings: [],
      notes: [],
    });
    expect(portAdvice(0, "vision", null)).toEqual({ warnings: [], notes: [] });
  });

  it("names the legacy, backup and other known ports", () => {
    expect(portAdvice(10005, "vision", null).notes).toEqual([text.port.legacy]);
    expect(portAdvice(10016, "vision", null).notes).toEqual([text.port.backup]);
    expect(portAdvice(10003, "vision", null).notes).toEqual([
      text.port.known("the game controller"),
    ]);
    expect(portAdvice(10005, "gc", null).notes).toEqual([
      text.port.known("legacy vision"),
    ]);
    expect(portAdvice(12345, "gc", null).notes).toEqual([
      text.port.nonStandard,
    ]);
  });

  it("notes a privileged port, using the host's own limit and root state", () => {
    expect(portAdvice(80, "gc", null).notes[0]).toBe(
      text.port.privileged(1024, false),
    );

    const state = netState({
      unprivilegedPortStart: 0,
      root: true,
    });
    expect(portAdvice(80, "gc", state).notes).toEqual([text.port.nonStandard]);

    const rootState = netState({
      unprivilegedPortStart: 2000,
      root: true,
    });
    expect(portAdvice(1500, "gc", rootState).notes[0]).toBe(
      text.port.privileged(2000, true),
    );
  });
});
