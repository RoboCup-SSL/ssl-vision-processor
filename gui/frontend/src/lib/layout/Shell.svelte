<script lang="ts">
  import { connectionState } from "../wrapper-bus";
  import { preferences } from "../preferences.svelte";
  import InstanceList from "./InstanceList.svelte";
  import ConfigNav from "./ConfigNav.svelte";
  import MainContent from "./MainContent.svelte";
  import type { Snippet } from "svelte";
  import {
    Dropdown,
    DropdownGroup,
    DropdownHeader,
    DropdownItem,
    Checkbox,
  } from "flowbite-svelte";
  import { config, reloadFromDisk } from "../config.svelte";
  import { fileDialogs } from "./fileDialogs.svelte";
  import ConfigDialogs from "./ConfigDialogs.svelte";
  import { nav } from "./nav.svelte";
  import { network, networkFromDoc } from "../network.svelte";

  // `below` is where App.svelte puts the existing snapshot grid / WS dev
  // panel for now, until they become the mockup's real Video + Debug Console
  // (issue #18) -- see the note there. Keeping it a slot rather than baking
  // it into this component so Shell stays just the layout, not a dumping
  // ground for whatever hasn't found a permanent home yet.
  interface Props {
    below?: Snippet;
  }

  let { below }: Props = $props();

  // Global settings menu (gear, top-right): the vision.yml file actions, then
  // preferences (../preferences.svelte) -- both app-wide, not scoped to any
  // one tab, so they belong in the shell rather than wherever happened to
  // need them first. Dropdown (flowbite-svelte)
  // owns open/close and outside-click dismissal itself now -- previously
  // hand-rolled here with a window click listener.
  let settingsOpen = $state(false);

  let unsaved = $derived(config.state?.changes.length ?? 0);

  // Multicast badges: the address each host socket is open on, green while
  // packets arrive there. Until the first network.state lands, the configured
  // address stands in.
  let configured = $derived(networkFromDoc());
  let sockets = $derived([
    {
      label: "Vision",
      status: network.state?.vision,
      fallback: `${configured.vision_ip}:${String(configured.vision_port)}`,
    },
    {
      label: "GC",
      status: network.state?.gc,
      fallback: `${configured.gc_ip}:${String(configured.gc_port)}`,
    },
  ]);

  // Clears app.css's base-layer button chrome and matches the preference
  // checkboxes below: same text, same left inset as their p-3 group.
  const ITEM_CLASS =
    "flex items-center justify-between gap-4 rounded-none border-0 bg-transparent px-3 py-1.5 text-sm font-medium text-gray-900 dark:text-white";

  function openDialog(which: "save" | "saveAs" | "load"): void {
    settingsOpen = false;
    fileDialogs[which] = true;
  }

  function revert(): void {
    settingsOpen = false;

    if (
      unsaved === 0 ||
      confirm(
        `Discard ${String(unsaved)} unsaved change(s) and reload from disk?`,
      )
    ) {
      void reloadFromDisk();
    }
  }
</script>

<ConfigDialogs />

<div class="shell">
  <header>
    <h1>vision-processor</h1>
    {#each sockets as socket (socket.label)}
      <button
        type="button"
        class="badge socket"
        data-state={socket.status?.receiving ? "open" : "silent"}
        title={socket.status?.problem
          ? `Not open: ${socket.status.problem}. Retrying. Click to configure.`
          : socket.status?.receiving
            ? `Receiving on ${socket.status.address}. Click to configure.`
            : "Nothing heard on this address. Click to configure."}
        onclick={() => {
          nav.selectedCategoryId = "network";
        }}
      >
        {socket.label}
        {socket.status?.address ?? socket.fallback}
      </button>
    {/each}
    <span class="badge" data-state={$connectionState}>
      {$connectionState.toUpperCase()} ({location.host})
    </span>

    <div class="settings">
      <button
        id="settings-trigger"
        type="button"
        class="settings-button"
        aria-label="Settings"
      >
        ⚙{#if unsaved > 0}<span class="unsaved-dot" title="Unsaved changes"
          ></span>{/if}
      </button>

      <Dropdown
        placement="bottom-end"
        triggeredBy="#settings-trigger"
        bind:isOpen={settingsOpen}
      >
        <DropdownHeader class="px-3">
          <span class="block text-xs text-gray-500">Configuration</span>
          <span class="block truncate font-mono text-xs"
            >{config.state?.path ?? ""}</span
          >
        </DropdownHeader>
        <DropdownGroup>
          <DropdownItem
            class={ITEM_CLASS}
            onclick={() => {
              openDialog("save");
            }}
          >
            <span>Save{unsaved > 0 ? ` (${String(unsaved)})` : ""}</span>
            <span class="text-xs font-normal text-gray-400">Ctrl+S</span>
          </DropdownItem>
          <DropdownItem
            class={ITEM_CLASS}
            onclick={() => {
              openDialog("saveAs");
            }}>Save as…</DropdownItem
          >
          <DropdownItem
            class={ITEM_CLASS}
            onclick={() => {
              openDialog("load");
            }}>Load…</DropdownItem
          >
          <DropdownItem class={ITEM_CLASS} onclick={revert}
            >Revert to disk</DropdownItem
          >
        </DropdownGroup>
        <DropdownGroup class="flex flex-col gap-2 p-3">
          <Checkbox bind:checked={preferences.tooltipsEnabled}>
            Show extra tooltips
          </Checkbox>
          <Checkbox bind:checked={preferences.expertUser}>Expert mode</Checkbox>
        </DropdownGroup>
      </Dropdown>
    </div>
  </header>

  <aside class="sidebar">
    <InstanceList />
    <ConfigNav />
  </aside>

  <main>
    <MainContent />
    {#if below}
      <div class="below">
        {@render below()}
      </div>
    {/if}
  </main>
</div>

<style>
  .unsaved-dot {
    position: absolute;
    top: 0.15rem;
    right: 0.15rem;
    width: 0.45rem;
    height: 0.45rem;
    border-radius: 50%;
    background: #1a56db;
  }

  .shell {
    display: grid;
    grid-template-columns: 280px 1fr;
    grid-template-rows: auto 1fr;
    min-height: 100vh;
    font-family: ui-sans-serif, system-ui, sans-serif;
  }

  header {
    grid-column: 1 / -1;
    display: flex;
    align-items: center;
    gap: 1rem;
    padding: 0.75rem 1.5rem;
    border-bottom: 1px solid #ddd;
  }

  h1 {
    font-size: 1.1rem;
    margin: 0;
  }

  .badge {
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    padding: 0.2rem 0.5rem;
    border-radius: 999px;
    background: #ddd;
    color: #333;
  }

  .socket {
    border: none;
    cursor: pointer;
  }

  .socket:hover {
    filter: brightness(0.95);
  }

  .badge[data-state="silent"] {
    background: #eee;
    color: #666;
  }

  .badge[data-state="open"] {
    background: #c8e6c9;
    color: #1b5e20;
  }

  .badge[data-state="connecting"] {
    background: #fff3cd;
    color: #856404;
  }

  .badge[data-state="closed"] {
    background: #f8d7da;
    color: #721c24;
  }

  .settings {
    position: relative;
    margin-left: auto;
  }

  .settings-button {
    position: relative;
    width: 2rem;
    height: 2rem;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border: 1px solid #ccc;
    border-radius: 50%;
    background: white;
    font-size: 1.1rem;
    line-height: 1;
    cursor: pointer;
  }

  .settings-button:hover {
    background: #f5f5f5;
  }

  .sidebar {
    display: flex;
    flex-direction: column;
    border-right: 1px solid #ddd;
    overflow-y: auto;
  }

  main {
    overflow-y: auto;
  }

  .below {
    margin-top: 2rem;
    padding: 1rem 1.5rem;
    border-top: 1px solid #ddd;
  }
</style>
