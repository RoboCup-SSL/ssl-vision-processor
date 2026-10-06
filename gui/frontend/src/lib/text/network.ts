// Network page (config/NetworkPanel.svelte, config/HostInterfaces.svelte)
// and the address/port advice in network.svelte.ts. Advice warnings always
// show; notes show with "Show extra tooltips".

export const network = {
  heading: "Network",
  // `backticks` show as code.
  intro:
    "Shared by the host and every vision_processor (vision.yml's `defaults.network`). The host reopens its sockets as soon as a change applies. vision_processors read these only at startup, so restart them after a change.",
  reopening: (address: string): string => `Reopening on ${address}…`,

  interfaces: {
    autoHint: "(skips loopback, disconnected, and virtual interfaces)",
    waiting: "Waiting for the host…",
    scope:
      "Only this host's sockets; vision_processors use their system's default route. Broadcast receiving listens on every interface.",
    loopbackOff: (iface: string): string =>
      `Multicast is off on \`${iface}\`. Programs on this machine that multicast over loopback won't hear each other. To enable it until reboot:`,
    loopbackFix: (iface: string): string =>
      `sudo ip link set ${iface} multicast on`,
  },

  errors: {
    notIPv4: "Not an IPv4 address.",
    zeroAddress: "0.0.0.0 isn't a destination.",
    portRange: "Must be 1 to 65535.",
    sameAddress:
      "Vision and game controller can't use the same address and port.",
  },

  kind: { vision: "vision", gc: "game controller" },

  // Other well known SSL UDP ports, for "Normally ...'s port" notes.
  knownPorts: {
    10003: "the game controller",
    10013: "the backup game controller",
    10005: "legacy vision",
    10006: "vision",
    10016: "backup vision",
    10010: "the tracker",
  } as Record<number, string>,

  presets: {
    standardMulticast: "Standard multicast",
    broadcastAll: "Broadcast, all interfaces",
    broadcastOn: (iface: string, address: string): string =>
      `Broadcast, ${iface} (${address})`,
    standard: "Standard",
    backup: "Backup",
    legacy: "Legacy",
  },

  // A socket's state under its card, and the badge tooltips in the header.
  status: {
    waiting: "Waiting for the host…",
    notOpen: (problem: string): string => `Not open: ${problem}. Retrying…`,
    viaPort: " (listening on the port)",
    neverHeard: (what: string, via: string): string =>
      `No ${what} heard on this address yet${via}.`,
    receiving: (
      heard: number,
      what: string,
      from: string,
      via: string,
    ): string => `Receiving: ${String(heard)} ${what}, latest${from}${via}.`,
    silent: (seconds: string, from: string, via: string): string =>
      `Silent for ${seconds} s (last${from})${via}.`,
    from: (source: string): string => ` from ${source}`,
    visionWhat: "detection packets",
    gcWhat: "referee messages",
    overriding: (cameras: number[]): string =>
      `Camera${cameras.length === 1 ? "" : "s"} ${cameras.join(", ")} override${cameras.length === 1 ? "s" : ""} these in their own config, so their vision_processor uses different groups than the host.`,
  },

  address: {
    loopback: "Loopback: only reaches this machine.",
    reserved:
      "224.0.0.x is reserved for routing protocols. Pick another group.",
    sourceSpecific:
      "232.x is source-specific multicast: plain joins may receive nothing.",
    otherStandard: (kind: string): string =>
      `Standard ${kind} group. Works on a different port.`,
    nonStandard:
      "Non-standard group. Every SSL tool on the field must use it too.",
    sameMac: (group: string): string =>
      `Same Ethernet MAC as ${group}: switches can't filter them apart.`,
    broadcastAll:
      "Broadcast: floods the subnet. vision_processor sends it out its default route only.",
    broadcastOn: (iface: string): string =>
      `Subnet broadcast on ${iface}: floods the subnet.`,
    unknownBroadcast:
      "Not multicast. Looks like a subnet broadcast, but no interface on this host has it.",
    unicast:
      "Not multicast or broadcast: only the machine at this IP receives.",
  },

  port: {
    privileged: (start: number, root: boolean): string =>
      `Below ${String(start)}: binding needs root or CAP_NET_BIND_SERVICE, here and on every vision_processor${root ? " (this host runs as root)" : ""}.`,
    legacy:
      "Legacy 2014 format port: old clients get detections but not this geometry.",
    backup: "Backup (+10) port, for a redundant second system.",
    known: (user: string): string => `Normally ${user}'s port.`,
    nonStandard:
      "Non-standard port. Every SSL tool on the field must use it too.",
  },
};
