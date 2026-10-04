<script lang="ts">
  import { untrack } from "svelte";
  import { config } from "../config.svelte";
  import NoteTip from "../NoteTip.svelte";
  import HostInterfaces from "./HostInterfaces.svelte";
  import {
    network,
    networkFromDoc,
    writeNetwork,
    ipv4Error,
    portError,
    addressPresets,
    portPresets,
    addressAdvice,
    portAdvice,
    type Kind,
    type NetworkConfig,
    type SocketStatus,
  } from "../network.svelte";

  let current = $derived(networkFromDoc());

  // What's in the inputs. Committed to the document on change (blur, Enter,
  // or picking a preset) only once valid, so a half-typed address never
  // reaches the host, which would reopen its sockets on it.
  let draft = $state<NetworkConfig>(untrack(() => ({ ...current })));

  // An edit from elsewhere (another tab, the file on disk) replaces the draft.
  // Compared by value: a refetched document with the same addresses mustn't
  // wipe what's being typed.
  let synced = untrack(() => JSON.stringify(current));

  $effect(() => {
    const json = JSON.stringify(current);
    if (json === synced) return;

    synced = json;
    draft = JSON.parse(json) as NetworkConfig;
  });

  let errors = $derived({
    vision_ip: ipv4Error(draft.vision_ip),
    vision_port: portError(draft.vision_port),
    gc_ip: ipv4Error(draft.gc_ip),
    gc_port: portError(draft.gc_port),
  });

  let sharedError = $derived(
    !errors.vision_ip &&
      !errors.gc_ip &&
      draft.vision_ip === draft.gc_ip &&
      draft.vision_port === draft.gc_port
      ? "Vision and game controller can't use the same address and port."
      : null,
  );

  function commit(key: keyof NetworkConfig): void {
    if (errors[key] || sharedError || draft[key] === current[key]) return;

    writeNetwork(key, draft[key]);
  }

  // Cameras whose own config overrides the shared block: their
  // vision_processor uses different groups than the host.
  let overriding = $derived(
    (config.doc?.cameras ?? [])
      .filter((c) => c.config?.["network"] !== undefined)
      .map((c) => c.cameraId),
  );

  const GROUPS = [
    {
      id: "vision",
      title: "Vision",
      ipKey: "vision_ip",
      portKey: "vision_port",
      what: "detection packets",
    },
    {
      id: "gc",
      title: "Game controller",
      ipKey: "gc_ip",
      portKey: "gc_port",
      what: "referee messages",
    },
  ] as const;

  let groups = $derived(
    GROUPS.map((g) => {
      const kind: Kind = g.id;
      const ip = draft[g.ipKey];
      const port = draft[g.portKey];
      const addresses = addressPresets(kind, network.state?.interfaces ?? []);
      const ports = portPresets(kind);

      return {
        ...g,
        status: kind === "vision" ? network.state?.vision : network.state?.gc,
        addresses,
        ports,
        addressIndex: addresses.findIndex((p) => p.value === ip),
        portIndex: ports.findIndex((p) => p.value === port),
        addressAdvice: addressAdvice(ip, kind, network.state),
        portAdvice: portAdvice(port, kind, network.state),
      };
    }),
  );

  function pickAddress(g: (typeof groups)[number], index: number): void {
    const preset = g.addresses[index];
    if (!preset) return;

    draft[g.ipKey] = preset.value;
    commit(g.ipKey);
  }

  function pickPort(g: (typeof groups)[number], index: number): void {
    const preset = g.ports[index];
    if (!preset) return;

    draft[g.portKey] = preset.value;
    commit(g.portKey);
  }

  function describe(status: SocketStatus | undefined, what: string): string {
    if (!status) return "Waiting for the host…";

    if (status.problem) return `Not open: ${status.problem}. Retrying…`;

    const via = status.mode === "port" ? " (listening on the port)" : "";

    if (!status.lastHeard) return `No ${what} heard on this address yet${via}.`;

    const age = Math.max(
      0,
      (Date.now() - new Date(status.lastHeard).getTime()) / 1000,
    );
    const from = status.source ? ` from ${status.source}` : "";

    return status.receiving
      ? `Receiving: ${String(status.heard)} ${what}, latest${from}${via}.`
      : `Silent for ${age.toFixed(0)} s (last${from})${via}.`;
  }
</script>

<div class="network-panel">
  <h2>Network</h2>
  <p class="hint">
    Shared by the host and every vision_processor (vision.yml's
    <code>defaults.network</code>). The host reopens its sockets as soon as a
    change applies. vision_processors read these only at startup, so restart
    them after a change.
  </p>

  {#if overriding.length > 0}
    <p class="warning">
      Camera{overriding.length === 1 ? "" : "s"}
      {overriding.join(", ")} override{overriding.length === 1 ? "s" : ""} these in
      their own config, so their vision_processor uses different groups than the host.
    </p>
  {/if}

  {#if sharedError}
    <p class="error">{sharedError}</p>
  {/if}

  {#each groups as g (g.id)}
    <fieldset>
      <legend>
        <span
          class="dot"
          class:receiving={g.status?.receiving}
          title={g.status?.receiving ? "Receiving" : "Not receiving"}
        ></span>
        {g.title}
      </legend>

      <div class="row">
        <label for={`${g.id}-ip`}>Address</label>
        <select
          aria-label={`${g.title} address preset`}
          value={g.addressIndex}
          onchange={(e) => {
            pickAddress(g, Number(e.currentTarget.value));
          }}
        >
          {#each g.addresses as preset, i (preset.value)}
            <option value={i}>{preset.label}</option>
          {/each}
          <option value={-1} disabled>Custom</option>
        </select>
        <input
          id={`${g.id}-ip`}
          type="text"
          spellcheck="false"
          class:invalid={errors[g.ipKey]}
          bind:value={draft[g.ipKey]}
          onchange={() => {
            commit(g.ipKey);
          }}
        />
        <NoteTip id={`${g.id}-ip-notes`} notes={g.addressAdvice.notes} />
      </div>
      {#if errors[g.ipKey]}<p class="error">{errors[g.ipKey]}</p>{/if}
      {#each g.addressAdvice.warnings as warning (warning)}
        <p class="field-warning">{warning}</p>
      {/each}

      <div class="row">
        <label for={`${g.id}-port`}>Port</label>
        <select
          aria-label={`${g.title} port preset`}
          value={g.portIndex}
          onchange={(e) => {
            pickPort(g, Number(e.currentTarget.value));
          }}
        >
          {#each g.ports as preset, i (preset.value)}
            <option value={i}>{preset.label} ({preset.value})</option>
          {/each}
          <option value={-1} disabled>Custom</option>
        </select>
        <input
          id={`${g.id}-port`}
          type="number"
          min="1"
          max="65535"
          class:invalid={errors[g.portKey]}
          bind:value={draft[g.portKey]}
          onchange={() => {
            commit(g.portKey);
          }}
        />
        <NoteTip id={`${g.id}-port-notes`} notes={g.portAdvice.notes} />
      </div>
      {#if errors[g.portKey]}<p class="error">{errors[g.portKey]}</p>{/if}
      {#each g.portAdvice.warnings as warning (warning)}
        <p class="field-warning">{warning}</p>
      {/each}

      <p class="status">
        {#if g.status && g.status.address !== `${current[g.ipKey]}:${String(current[g.portKey])}`}
          Reopening on {current[g.ipKey]}:{current[g.portKey]}…
        {:else}
          {describe(g.status, g.what)}
        {/if}
      </p>
    </fieldset>
  {/each}

  <HostInterfaces />
</div>

<style>
  .network-panel {
    max-width: 560px;
  }

  h2 {
    margin: 0 0 0.5rem;
  }

  fieldset {
    border: 1px solid #ddd;
    border-radius: 4px;
    margin-bottom: 1rem;
  }

  legend {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }

  .row {
    display: grid;
    grid-template-columns: 4rem 1fr 9.5rem 1.25rem;
    align-items: center;
    gap: 0.5rem;
    margin: 0.4rem 0;
    font-size: 0.85rem;
  }

  select,
  input {
    font-size: 0.85rem;
  }

  input {
    font-family: monospace;
  }

  input.invalid {
    border-color: #c81e1e;
  }

  .dot {
    width: 0.55rem;
    height: 0.55rem;
    border-radius: 50%;
    background: #bbb;
  }

  .dot.receiving {
    background: #2e7d32;
  }

  .hint,
  .status {
    color: #666;
    font-size: 0.8rem;
  }

  .status {
    margin: 0.5rem 0 0;
  }

  .error,
  .field-warning {
    margin: 0 0 0.4rem;
    font-size: 0.8rem;
  }

  .error {
    color: #c81e1e;
  }

  .field-warning {
    color: #7a4a00;
  }

  .warning {
    padding: 0.4rem 0.6rem;
    border-radius: 4px;
    background: #fff6e5;
    color: #7a4a00;
    font-size: 0.85rem;
  }
</style>
