// The vision and game controller addresses. They live in
// vision.yml's defaults.network (the same block every vision_processor reads
// from its generated config.yml), so editing them goes through config.doc like
// any other setting; the host reopens its sockets as soon as the edit lands.
// Socket status arrives separately, once a second, on the network.state topic.
import { config } from "./config.svelte";
import { topic } from "./wrapper-bus";
import { network as text } from "./text/network";

// Mirrors gui/internal/multicast's Status.
export interface SocketStatus {
  address: string;
  heard: number;
  lastHeard?: string;
  source?: string;
  receiving: boolean;
  // Why the socket isn't fully open (e.g. "no usable network interface");
  // the host retries until it is.
  problem?: string;
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

// One camera_id heard on the vision socket from one address, from
// gui/internal/detections' Source.
export interface DetectionSource {
  cameraId: number;
  address: string;
  fps: number;
  lastHeard: string;
  receiving: boolean;
}

// The current match's teams and their robot heights, from
// gui/internal/referee's State.
export interface RefereeTeam {
  name: string;
  // From the height table; unset when the team isn't in it.
  height?: number;
}

export interface RefereeState {
  // When the last referee message arrived; unset if none has.
  heard?: string;
  yellow: RefereeTeam;
  blue: RefereeTeam;
  heightsFile: string;
  heightsError?: string;
  meanHeight?: number;
  maxHeight?: number;
  // The whole table, team name to height in mm.
  heights?: Record<string, number>;
}

export interface NetworkState {
  vision: SocketStatus;
  gc: SocketStatus;
  // Every camera_id heard in the last 30 s, by camera_id then address.
  cameras: DetectionSource[] | null;
  referee: RefereeState;
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
  if (!octets) return text.errors.notIPv4;
  if (octets.every((o) => o === 0)) return text.errors.zeroAddress;

  return null;
}

export function portError(port: number): string | null {
  return Number.isInteger(port) && port >= 1 && port <= 65535
    ? null
    : text.errors.portRange;
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

const KIND_LABEL: Record<Kind, string> = text.kind;

// The league's standard groups; a redundant second system adds 10 to the port.
export const STANDARD: Record<Kind, { ip: string; port: number }> = {
  vision: { ip: "224.5.23.2", port: 10006 },
  gc: { ip: "224.5.23.1", port: 10003 },
};

const BACKUP_OFFSET = 10;

// Carries RoboCup2014Legacy.Wrapper packets: same detection field, different
// geometry message.
const LEGACY_VISION_PORT = 10005;

const KNOWN_PORTS = text.knownPorts;

export interface Preset<T> {
  label: string;
  value: T;
}

export function addressPresets(
  kind: Kind,
  interfaces: HostInterface[],
): Preset<string>[] {
  return [
    { label: text.presets.standardMulticast, value: STANDARD[kind].ip },
    { label: text.presets.broadcastAll, value: "255.255.255.255" },
    ...interfaces.flatMap((i) =>
      i.used && i.broadcast
        ? [
            {
              label: text.presets.broadcastOn(i.name, i.address ?? ""),
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
    { label: text.presets.standard, value: standard },
    { label: text.presets.backup, value: standard + BACKUP_OFFSET },
  ];

  if (kind === "vision") {
    presets.push({ label: text.presets.legacy, value: LEGACY_VISION_PORT });
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
    advice.warnings.push(text.address.loopback);
  } else if (a >= 224 && a <= 239) {
    if (a === 224 && b === 0 && c === 0) {
      advice.warnings.push(text.address.reserved);
    } else if (a === 232) {
      advice.warnings.push(text.address.sourceSpecific);
    }

    if (ip === STANDARD[other].ip) {
      advice.notes.push(text.address.otherStandard(KIND_LABEL[other]));
    } else if (ip !== STANDARD[kind].ip) {
      advice.notes.push(text.address.nonStandard);
    }

    for (const std of [STANDARD.vision.ip, STANDARD.gc.ip]) {
      const stdOctets = parseIPv4(std);
      if (ip !== std && stdOctets && low23(octets) === low23(stdOctets)) {
        advice.notes.push(text.address.sameMac(std));
      }
    }
  } else {
    const iface = state?.interfaces?.find((i) => i.broadcast === ip);

    if (ip === "255.255.255.255") {
      advice.notes.push(text.address.broadcastAll);
    } else if (iface) {
      advice.notes.push(text.address.broadcastOn(iface.name));
    } else if (d === 255) {
      advice.warnings.push(text.address.unknownBroadcast);
    } else {
      advice.warnings.push(text.address.unicast);
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
    advice.notes.push(text.port.privileged(start, state?.root ?? false));
  }

  const standard = STANDARD[kind].port;
  const known = KNOWN_PORTS[port];

  if (port === standard) return advice;

  if (kind === "vision" && port === LEGACY_VISION_PORT) {
    advice.notes.push(text.port.legacy);
  } else if (port === standard + BACKUP_OFFSET) {
    advice.notes.push(text.port.backup);
  } else if (known) {
    advice.notes.push(text.port.known(known));
  } else {
    advice.notes.push(text.port.nonStandard);
  }

  return advice;
}
