<script lang="ts">
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

<fieldset>
  <legend>Host interfaces</legend>

  <label class="auto">
    <input
      type="checkbox"
      checked={selection.auto}
      onchange={(e) => {
        setAuto(e.currentTarget.checked);
      }}
    />
    Auto
    <span class="hint"
      >(skips loopback, disconnected, and virtual interfaces)</span
    >
  </label>

  <ul class:muted={selection.auto}>
    {#each rows as row (row.name)}
      <li>
        <label>
          <input
            type="checkbox"
            checked={row.checked}
            disabled={row.disabled}
            onchange={(e) => {
              toggle(row.name, e.currentTarget.checked);
            }}
          />
          <span
            ><span class="name">{row.name}</span><span class="detail"
              >{row.detail}</span
            ></span
          >
        </label>
      </li>
    {:else}
      <li class="hint">Waiting for the host…</li>
    {/each}
  </ul>

  <p class="hint">
    Only this host's sockets; vision_processors use their system's default
    route. Broadcast receiving listens on every interface.
  </p>

  {#if loopbackOff && network.state}
    <div class="warning">
      Multicast is off on <code>{network.state.loopback}</code>. Programs on
      this machine that multicast over loopback won't hear each other. To enable
      it until reboot:
      <pre>sudo ip link set {network.state.loopback} multicast on</pre>
    </div>
  {/if}
</fieldset>

<style>
  fieldset {
    border: 1px solid #ddd;
    border-radius: 4px;
    margin-bottom: 1rem;
  }

  label {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    margin: 0.3rem 0;
    font-size: 0.85rem;
  }

  ul {
    margin: 0.25rem 0 0.5rem 1.2rem;
    padding: 0;
    list-style: none;
  }

  ul.muted {
    color: #999;
  }

  .name {
    font-family: monospace;
  }

  .detail,
  .hint {
    color: #666;
    font-size: 0.8rem;
  }

  ul.muted .detail {
    color: #aaa;
  }

  .warning {
    padding: 0.4rem 0.6rem;
    border-radius: 4px;
    background: #fff6e5;
    color: #7a4a00;
    font-size: 0.85rem;
  }

  pre {
    margin: 0.4rem 0 0;
    padding: 0.3rem 0.5rem;
    border-radius: 4px;
    background: #fff;
    font-size: 0.8rem;
    user-select: all;
  }
</style>
