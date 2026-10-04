<script lang="ts">
  import SettingsCard from "../SettingsCard.svelte";
  import { Checkbox, Toggle, Alert, P } from "flowbite-svelte";
  import {
    network,
    interfaceSelection,
    writeInterfaces,
    type HostInterface,
  } from "../network.svelte";

  // Which interfaces the GUI host's own sockets use. Host only: interface
  // names mean nothing on another machine, and vision_processor always uses
  // the system's default route.
  let selection = $derived(interfaceSelection());
  let ifaces = $derived(network.state?.interfaces ?? []);

  interface Row {
    name: string;
    detail: string;
    checked: boolean;
    disabled: boolean;
  }

  let rows = $derived.by((): Row[] => {
    const present = ifaces.map((i): Row => {
      if (selection.auto) {
        return {
          name: i.name,
          detail: detail(i.address, i.autoReason),
          checked: i.autoUse,
          disabled: true,
        };
      }

      return {
        name: i.name,
        detail: detail(i.address, i.usable ? undefined : unusableReason(i)),
        checked: i.usable && !selection.skip.includes(i.name),
        disabled: !i.usable,
      };
    });

    // A skipped interface that isn't here right now (unplugged adapter, a
    // stopped container's bridge) stays listed so it can be un-skipped.
    const missing = selection.auto
      ? []
      : selection.skip
          .filter((name) => !ifaces.some((i) => i.name === name))
          .map(
            (name): Row => ({
              name,
              detail: detail(undefined, "not present"),
              checked: false,
              disabled: false,
            }),
          );

    return [...present, ...missing];
  });

  // "(192.168.1.5, no carrier)", "(down)", or nothing.
  function detail(address?: string, reason?: string): string {
    const parts = [address, reason].filter(Boolean);

    return parts.length > 0 ? ` (${parts.join(", ")})` : "";
  }

  function unusableReason(i: HostInterface): string {
    return i.autoReason === "no carrier" || i.autoReason === "virtual"
      ? "unusable"
      : (i.autoReason ?? "unusable");
  }

  function setAuto(auto: boolean): void {
    // Going manual for the first time starts from what auto had picked, so
    // the switch itself changes nothing.
    const skip =
      !auto && selection.skip.length === 0
        ? ifaces.filter((i) => i.usable && !i.autoUse).map((i) => i.name)
        : selection.skip;

    writeInterfaces(auto, skip);
  }

  function toggle(name: string, use: boolean): void {
    const skip = selection.skip.filter((n) => n !== name);
    if (!use) skip.push(name);

    writeInterfaces(false, skip);
  }

  let loopbackOff = $derived(
    network.state !== null && !network.state.loopbackMulticast,
  );
</script>

<SettingsCard title="Host interfaces">
  <Toggle
    size="small"
    checked={selection.auto}
    onchange={(e: Event) => {
      setAuto((e.currentTarget as HTMLInputElement).checked);
    }}
  >
    Auto
    <span class="hint ms-1"
      >(skips loopback, disconnected, and virtual interfaces)</span
    >
  </Toggle>

  <ul class:muted={selection.auto}>
    {#each rows as row (row.name)}
      <li>
        <Checkbox
          checked={row.checked}
          disabled={row.disabled}
          onchange={(e: Event) => {
            toggle(row.name, (e.currentTarget as HTMLInputElement).checked);
          }}
        >
          <span
            ><span class="name">{row.name}</span><span class="detail"
              >{row.detail}</span
            ></span
          >
        </Checkbox>
      </li>
    {:else}
      <li class="hint">Waiting for the host…</li>
    {/each}
  </ul>

  <P size="sm" class="mb-2 text-gray-600 dark:text-gray-400">
    Only this host's sockets; vision_processors use their system's default
    route. Broadcast receiving listens on every interface.
  </P>

  {#if loopbackOff && network.state}
    <Alert color="yellow" class="mt-2 p-3 text-sm">
      Multicast is off on <code>{network.state.loopback}</code>. Programs on
      this machine that multicast over loopback won't hear each other. To enable
      it until reboot:
      <pre>sudo ip link set {network.state.loopback} multicast on</pre>
    </Alert>
  {/if}
</SettingsCard>

<style>
  ul {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    margin: 0.5rem 0 0.5rem 1.2rem;
    padding: 0;
    list-style: none;
  }

  ul.muted {
    color: var(--color-gray-400);
  }

  .name {
    font-family: monospace;
  }

  .detail,
  .hint {
    color: var(--color-gray-600);
    font-size: 0.8rem;
  }

  ul.muted .detail {
    color: var(--color-gray-400);
  }

  pre {
    margin: 0.4rem 0 0;
    padding: 0.3rem 0.5rem;
    border-radius: 4px;
    background: var(--color-white);
    font-size: 0.8rem;
    user-select: all;
  }
</style>
