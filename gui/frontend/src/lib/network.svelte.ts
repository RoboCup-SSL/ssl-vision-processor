// The vision and game controller addresses. They live in
// vision.yml's defaults.network (the same block every vision_processor reads
// from its generated config.yml), so editing them goes through config.doc like
// any other setting; the host reopens its sockets as soon as the edit lands.
// Socket status arrives separately, once a second, on the network.state topic.
import { config } from "./config.svelte";
import { topic } from "./wrapper-bus";

// Mirrors gui/internal/multicast's Status.
export interface SocketStatus {
  address: string;
  heard: number;
  lastHeard?: string;
  source?: string;
  receiving: boolean;
  // "port": no group to join, listening on the port (broadcast or unicast).
  mode: "multicast" | "port";
}

// A local network interface, from gui/internal/multicast's Interface.
export interface HostInterface {
  name: string;
  address?: string;
  broadcast?: string;
  // Up, multicast capable, with an IPv4 address.
  usable: boolean;
  // Automatic selection's verdict, and why not.
  autoUse: boolean;
  autoReason?: string;
  // Used by the host's sockets right now.
  used: boolean;
}

export interface NetworkState {
  vision: SocketStatus;
  gc: SocketStatus;
  autoInterfaces: boolean;
  interfaces: HostInterface[] | null;
  // The loopback interface, and whether multicast is enabled on it.
  loopback: string;
  loopbackMulticast: boolean;
  // Lowest port an unprivileged process may bind on the host.
  unprivilegedPortStart: number;
  root: boolean;
}

export interface NetworkConfig {
  vision_ip: string;
  vision_port: number;
  gc_ip: string;
  gc_port: number;
}

export type NetworkKey = keyof NetworkConfig;

// vision_processor's fallbacks (Resources.cpp) for keys the block leaves out.
export const NETWORK_DEFAULTS: NetworkConfig = {
  vision_ip: "224.5.23.2",
  vision_port: 10006,
  gc_ip: "224.5.23.1",
  gc_port: 10003,
};

export const network = $state<{ state: NetworkState | null }>({
  state: null,
});

$effect.root(() => {
  $effect(() => {
    return topic<NetworkState>("network.state").subscribe((state) => {
      if (state) network.state = state;
    });
  });
});

function block(): Record<string, unknown> | undefined {
  return config.doc?.defaults?.["network"] as
    | Record<string, unknown>
    | undefined;
}

// The shared network block with fallbacks applied.
export function networkFromDoc(): NetworkConfig {
  const b = block() ?? {};
  const str = (key: NetworkKey): string =>
    typeof b[key] === "string" ? b[key] : String(NETWORK_DEFAULTS[key]);
  const num = (key: NetworkKey): number =>
    typeof b[key] === "number" ? b[key] : Number(NETWORK_DEFAULTS[key]);

  return {
    vision_ip: str("vision_ip"),
    vision_port: num("vision_port"),
    gc_ip: str("gc_ip"),
    gc_port: num("gc_port"),
  };
}

export function writeNetwork<K extends NetworkKey>(
  key: K,
  value: NetworkConfig[K],
): void {
  const doc = config.doc;
  if (!doc) return;

  doc.defaults ??= {};
  doc.defaults["network"] = { ...(block() ?? {}), [key]: value };
}

// host.interfaces: automatic (the default) or a skip list.
export function interfaceSelection(): { auto: boolean; skip: string[] } {
  const i = config.doc?.host?.interfaces;

  return { auto: i?.auto ?? true, skip: i?.skip ?? [] };
}

export function writeInterfaces(auto: boolean, skip: string[]): void {
  const doc = config.doc;
  if (!doc) return;

  doc.host = {
    ...doc.host,
    interfaces: { auto, ...(skip.length > 0 ? { skip } : {}) },
  };
}

// Format checks only, the same as gui/internal/config's Network.Validate.
// Whether the address is a sensible choice is advice's job, below.
export function ipv4Error(ip: string): string | null {
  const octets = parseIPv4(ip);
  if (!octets) return "Not an IPv4 address.";
  if (octets.every((o) => o === 0)) return "0.0.0.0 isn't a destination.";

  return null;
}

export function portError(port: number): string | null {
  return Number.isInteger(port) && port >= 1 && port <= 65535
    ? null
    : "Must be 1 to 65535.";
}

function parseIPv4(ip: string): number[] | null {
  const parts = ip.split(".");
  if (parts.length !== 4 || parts.some((p) => !/^\d{1,3}$/.test(p))) {
    return null;
  }

  const octets = parts.map(Number);

  return octets.some((o) => o > 255) ? null : octets;
}

export type Kind = "vision" | "gc";

const KIND_LABEL: Record<Kind, string> = {
  vision: "vision",
  gc: "game controller",
};

// The league's standard groups; a redundant second system adds 10 to the port.
export const STANDARD: Record<Kind, { ip: string; port: number }> = {
  vision: { ip: "224.5.23.2", port: 10006 },
  gc: { ip: "224.5.23.1", port: 10003 },
};

const BACKUP_OFFSET = 10;

// Carries RoboCup2014Legacy.Wrapper packets: same detection field, different
// geometry message.
const LEGACY_VISION_PORT = 10005;

// Other well known SSL UDP ports, for "normally used by" notes.
const KNOWN_PORTS: Record<number, string> = {
  10003: "the game controller",
  10013: "the backup game controller",
  10005: "legacy vision",
  10006: "vision",
  10016: "backup vision",
  10010: "the tracker",
};

export interface Preset<T> {
  label: string;
  value: T;
}

export function addressPresets(
  kind: Kind,
  interfaces: HostInterface[],
): Preset<string>[] {
  return [
    { label: "Standard multicast", value: STANDARD[kind].ip },
    { label: "Broadcast, all interfaces", value: "255.255.255.255" },
    ...interfaces.flatMap((i) =>
      i.used && i.broadcast
        ? [
            {
              label: `Broadcast, ${i.name} (${i.address ?? ""})`,
              value: i.broadcast,
            },
          ]
        : [],
    ),
  ].filter((p, i, all) => all.findIndex((q) => q.value === p.value) === i);
}

export function portPresets(kind: Kind): Preset<number>[] {
  const standard = STANDARD[kind].port;
  const presets = [
    { label: "Standard", value: standard },
    { label: "Backup", value: standard + BACKUP_OFFSET },
  ];

  if (kind === "vision") {
    presets.push({ label: "Legacy", value: LEGACY_VISION_PORT });
  }

  return presets;
}

// Warnings are shown always; notes only with extra tooltips turned on.
export interface Advice {
  warnings: string[];
  notes: string[];
}

// Ethernet multicast MACs carry only the low 23 bits of the group, so groups
// that differ only above them share a MAC and switches can't separate them.
function low23(octets: number[]): number {
  const [, b = 0, c = 0, d = 0] = octets;

  return ((b & 0x7f) << 16) | (c << 8) | d;
}

export function addressAdvice(
  ip: string,
  kind: Kind,
  state: NetworkState | null,
): Advice {
  const advice: Advice = { warnings: [], notes: [] };
  const octets = parseIPv4(ip);
  if (!octets) return advice;

  const [a = 0, b = 0, c = 0, d = 0] = octets;
  const other: Kind = kind === "vision" ? "gc" : "vision";

  if (a === 127) {
    advice.warnings.push("Loopback: only reaches this machine.");
  } else if (a >= 224 && a <= 239) {
    if (a === 224 && b === 0 && c === 0) {
      advice.warnings.push(
        "224.0.0.x is reserved for routing protocols. Pick another group.",
      );
    } else if (a === 232) {
      advice.warnings.push(
        "232.x is source-specific multicast: plain joins may receive nothing.",
      );
    }

    if (ip === STANDARD[other].ip) {
      advice.notes.push(
        `Standard ${KIND_LABEL[other]} group. Works on a different port.`,
      );
    } else if (ip !== STANDARD[kind].ip) {
      advice.notes.push(
        "Non-standard group. Every SSL tool on the field must use it too.",
      );
    }

    for (const std of [STANDARD.vision.ip, STANDARD.gc.ip]) {
      const stdOctets = parseIPv4(std);
      if (ip !== std && stdOctets && low23(octets) === low23(stdOctets)) {
        advice.notes.push(
          `Same Ethernet MAC as ${std}: switches can't filter them apart.`,
        );
      }
    }
  } else {
    const iface = state?.interfaces?.find((i) => i.broadcast === ip);

    if (ip === "255.255.255.255") {
      advice.notes.push(
        "Broadcast: floods the subnet. vision_processor sends it out its default route only.",
      );
    } else if (iface) {
      advice.notes.push(
        `Subnet broadcast on ${iface.name}: floods the subnet.`,
      );
    } else if (d === 255) {
      advice.warnings.push(
        "Not multicast. Looks like a subnet broadcast, but no interface on this host has it.",
      );
    } else {
      advice.warnings.push(
        "Not multicast or broadcast: only the machine at this IP receives.",
      );
    }
  }

  return advice;
}

export function portAdvice(
  port: number,
  kind: Kind,
  state: NetworkState | null,
): Advice {
  const advice: Advice = { warnings: [], notes: [] };
  if (portError(port)) return advice;

  const start = state?.unprivilegedPortStart ?? 1024;
  if (port < start) {
    advice.notes.push(
      `Below ${String(start)}: binding needs root or CAP_NET_BIND_SERVICE, here and on every vision_processor${state?.root ? " (this host runs as root)" : ""}.`,
    );
  }

  const standard = STANDARD[kind].port;
  const known = KNOWN_PORTS[port];

  if (port === standard) return advice;

  if (kind === "vision" && port === LEGACY_VISION_PORT) {
    advice.notes.push(
      "Legacy 2014 format port: old clients get detections but not this geometry.",
    );
  } else if (port === standard + BACKUP_OFFSET) {
    advice.notes.push("Backup (+10) port, for a redundant second system.");
  } else if (known) {
    advice.notes.push(`Normally ${known}'s port.`);
  } else {
    advice.notes.push(
      "Non-standard port. Every SSL tool on the field must use it too.",
    );
  }

  return advice;
}
