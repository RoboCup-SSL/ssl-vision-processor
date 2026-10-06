<script lang="ts">
  import RichText from "../RichText.svelte";
  import SettingsCard from "../SettingsCard.svelte";
  import { Input, Select, Indicator, Alert, Heading, P } from "flowbite-svelte";
  import { untrack } from "svelte";
  import { network as text } from "../text/network";
  import { config } from "../config.svelte";
  import FormRow from "../FormRow.svelte";
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
      ? text.errors.sameAddress
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
      what: text.status.visionWhat,
    },
    {
      id: "gc",
      title: "Game controller",
      ipKey: "gc_ip",
      portKey: "gc_port",
      what: text.status.gcWhat,
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
    if (!status) return text.status.waiting;

    if (status.problem) return text.status.notOpen(status.problem);

    const via = status.mode === "port" ? text.status.viaPort : "";

    if (!status.lastHeard) return text.status.neverHeard(what, via);

    const age = Math.max(
      0,
      (Date.now() - new Date(status.lastHeard).getTime()) / 1000,
    );
    const from = status.source ? text.status.from(status.source) : "";

    return status.receiving
      ? text.status.receiving(status.heard, what, from, via)
      : text.status.silent(age.toFixed(0), from, via);
  }
</script>

<div class="network-panel">
  <Heading tag="h2" class="mb-2 text-xl font-semibold">{text.heading}</Heading>
  <P size="sm" class="mb-2 text-gray-600 dark:text-gray-400"
    ><RichText text={text.intro} /></P
  >

  {#if overriding.length > 0}
    <Alert color="yellow" class="mb-3 p-2 text-sm">
      {text.status.overriding(overriding)}
    </Alert>
  {/if}

  {#if sharedError}
    <Alert color="red" class="mb-3 p-2 text-sm">{sharedError}</Alert>
  {/if}

  {#each groups as g (g.id)}
    <SettingsCard title={g.title}>
      {#snippet status()}
        <Indicator
          size="sm"
          color={g.status?.receiving ? "green" : "gray"}
          title={g.status?.receiving ? "Receiving" : "Not receiving"}
        />
      {/snippet}

      <FormRow
        label="Address"
        for={`${g.id}-ip`}
        labelWidth="4rem"
        notes={g.addressAdvice.notes}
        warnings={g.addressAdvice.warnings}
        errors={errors[g.ipKey] ? [errors[g.ipKey] ?? ""] : []}
      >
        <div class="pair">
          <Select
            aria-label={`${g.title} address preset`}
            size="sm"
            placeholder=""
            value={g.addressIndex}
            onchange={(e: Event) => {
              pickAddress(
                g,
                Number((e.currentTarget as HTMLSelectElement).value),
              );
            }}
          >
            {#each g.addresses as preset, i (preset.value)}
              <option value={i}>{preset.label}</option>
            {/each}
            <option value={-1} disabled>Custom</option>
          </Select>
          <Input
            id={`${g.id}-ip`}
            type="text"
            size="sm"
            class="font-mono"
            spellcheck="false"
            color={errors[g.ipKey] ? "red" : "default"}
            bind:value={draft[g.ipKey]}
            onchange={() => {
              commit(g.ipKey);
            }}
          />
        </div>
      </FormRow>

      <FormRow
        label="Port"
        for={`${g.id}-port`}
        labelWidth="4rem"
        notes={g.portAdvice.notes}
        warnings={g.portAdvice.warnings}
        errors={errors[g.portKey] ? [errors[g.portKey] ?? ""] : []}
      >
        <div class="pair">
          <Select
            aria-label={`${g.title} port preset`}
            size="sm"
            placeholder=""
            value={g.portIndex}
            onchange={(e: Event) => {
              pickPort(g, Number((e.currentTarget as HTMLSelectElement).value));
            }}
          >
            {#each g.ports as preset, i (preset.value)}
              <option value={i}>{preset.label} ({preset.value})</option>
            {/each}
            <option value={-1} disabled>Custom</option>
          </Select>
          <Input
            id={`${g.id}-port`}
            type="number"
            size="sm"
            class="font-mono"
            min="1"
            max="65535"
            color={errors[g.portKey] ? "red" : "default"}
            bind:value={draft[g.portKey]}
            onchange={() => {
              commit(g.portKey);
            }}
          />
        </div>
      </FormRow>

      <p class="status">
        {#if g.status && g.status.address !== `${current[g.ipKey]}:${String(current[g.portKey])}`}
          {text.reopening(`${current[g.ipKey]}:${String(current[g.portKey])}`)}
        {:else}
          {describe(g.status, g.what)}
        {/if}
      </p>
    </SettingsCard>
  {/each}

  <HostInterfaces />
</div>

<style>
  .network-panel {
    max-width: 560px;
  }

  .pair {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 9.5rem;
    gap: 0.5rem;
  }

  .status {
    color: var(--text-muted);
    font-size: 0.8rem;
  }

  .status {
    margin: 0.5rem 0 0;
  }
</style>
