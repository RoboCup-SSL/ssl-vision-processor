<script lang="ts">
  import {
    Badge,
    Dropdown,
    DropdownGroup,
    DropdownItem,
  } from "flowbite-svelte";
  import { ChevronDownOutline } from "flowbite-svelte-icons";
  import { app as text } from "../text/app";
  import { categoryHref } from "./nav.svelte";
  import type { Socket } from "../network.svelte";

  // The header's multicast sockets: a badge each, with its address, so teams
  // can read where to connect at a glance. When the header is too narrow for
  // them (Shell.svelte measures), they fold into one Network dropdown with
  // the same addresses inside.
  interface Props {
    sockets: Socket[];
    collapsed: boolean;
  }

  let { sockets, collapsed }: Props = $props();

  type Color = "yellow" | "green" | "gray";

  function color(socket: Socket): Color {
    if (socket.status?.problem) return "yellow";

    return socket.status?.receiving ? "green" : "gray";
  }

  const DOT: Record<Color, string> = {
    yellow: "bg-yellow-400",
    green: "bg-green-500",
    gray: "bg-gray-400 dark:bg-gray-500",
  };

  function address(socket: Socket): string | undefined {
    return socket.status?.address ?? socket.fallback;
  }

  function badgeTitle(socket: Socket): string {
    if (socket.status?.problem) {
      return text.socketBadge.notOpen(socket.status.problem);
    }

    return socket.status?.receiving
      ? text.socketBadge.receiving(socket.status.address)
      : text.socketBadge.silent;
  }

  function menuStatus(socket: Socket): string {
    if (socket.status?.problem) {
      return text.socketMenu.notOpen(socket.status.problem);
    }

    return socket.status?.receiving
      ? text.socketMenu.receiving
      : text.socketMenu.silent;
  }

  // Folded, the badge takes the most pressing color: a problem anywhere,
  // else green only once every socket is receiving.
  let overall = $derived<Color>(
    sockets.some((s) => color(s) === "yellow")
      ? "yellow"
      : sockets.every((s) => color(s) === "green")
        ? "green"
        : "gray",
  );
</script>

{#if collapsed}
  <button
    id="network-status-trigger"
    type="button"
    class="flex shrink-0 hover:brightness-95"
    title={text.socketMenu.title}
  >
    <Badge rounded color={overall} class="gap-1.5">
      {#each sockets as socket (socket.label)}
        <span class={["h-2 w-2 rounded-full", DOT[color(socket)]]}></span>
      {/each}
      {text.socketMenu.label}
      <ChevronDownOutline size="xs" />
    </Badge>
  </button>
  <Dropdown triggeredBy="#network-status-trigger" placement="bottom-start">
    <DropdownGroup>
      {#each sockets as socket (socket.label)}
        <DropdownItem
          href={categoryHref("network")}
          class="flex items-start gap-2.5 px-3 py-2 text-sm"
        >
          <span
            class={["mt-1.5 h-2 w-2 shrink-0 rounded-full", DOT[color(socket)]]}
          ></span>
          <span class="flex flex-col">
            <span class="font-medium text-gray-900 dark:text-white"
              >{socket.label}</span
            >
            {#if address(socket)}
              <span class="font-mono text-xs">{address(socket)}</span>
            {/if}
            <span class="text-xs text-gray-500 dark:text-gray-400"
              >{menuStatus(socket)}</span
            >
          </span>
        </DropdownItem>
      {/each}
    </DropdownGroup>
  </Dropdown>
{:else}
  {#each sockets as socket (socket.label)}
    {@const addr = address(socket)}
    <a
      href={categoryHref("network")}
      class="flex shrink-0 hover:brightness-95"
      title={badgeTitle(socket)}
    >
      <Badge rounded color={color(socket)}>
        {socket.label}{#if addr}
          ({addr}){/if}
      </Badge>
    </a>
  {/each}
{/if}
