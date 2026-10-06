<script lang="ts">
  import ConfirmModal from "../ConfirmModal.svelte";
  import { connectionState } from "../wrapper-bus";
  import { preferences } from "../preferences.svelte";
  import InstanceList from "./InstanceList.svelte";
  import AlertSummary from "../alerts/AlertSummary.svelte";
  import { app as text } from "../text/app";
  import { links } from "../text/links";
  import { setCompetitionField } from "../teams.svelte";
  import ConfigNav from "./ConfigNav.svelte";
  import MainContent from "./MainContent.svelte";
  import type { Snippet } from "svelte";
  import {
    Dropdown,
    DropdownGroup,
    DropdownHeader,
    DropdownItem,
    Checkbox,
    Badge,
    Button,
    Indicator,
    DarkMode,
    Toggle,
    Kbd,
    Navbar,
    NavBrand,
    Sidebar,
  } from "flowbite-svelte";
  import { config, reloadFromDisk } from "../config.svelte";
  import { fileDialogs } from "./fileDialogs.svelte";
  import ConfigDialogs from "./ConfigDialogs.svelte";
  import { categoryHref } from "./nav.svelte";
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

  // The WebSocket to this host: green when open.
  const CONNECTION_COLOR = {
    open: "green",
    connecting: "yellow",
    closed: "red",
  } as const;

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
      label: "Game Controller",
      status: network.state?.gc,
      fallback: `${configured.gc_ip}:${String(configured.gc_port)}`,
    },
  ]);

  // Matches the preference checkboxes below: same text, same left inset as
  // their p-3 group.
  const ITEM_CLASS =
    "flex items-center justify-between gap-4 px-3 py-1.5 text-sm font-medium text-gray-900 dark:text-white";

  function openDialog(which: "save" | "saveAs" | "load"): void {
    settingsOpen = false;
    fileDialogs[which] = true;
  }

  let confirmingRevert = $state(false);

  function revert(): void {
    settingsOpen = false;

    if (unsaved === 0) void reloadFromDisk();
    else confirmingRevert = true;
  }
</script>

<ConfirmModal
  bind:open={confirmingRevert}
  title={text.discardTitle}
  message={text.discardAndReload(unsaved)}
  confirmLabel={text.discardConfirm}
  danger
  onconfirm={() => void reloadFromDisk()}
/>

<ConfigDialogs />

<div class="shell">
  <header>
    <Navbar fluid class="px-6 py-2" closeOnClickOutside={false}>
      <div class="flex flex-wrap items-center gap-3">
        <NavBrand>
          <h1 class="text-lg font-semibold whitespace-nowrap">
            SSL Vision Processor
          </h1>
        </NavBrand>
        {#each sockets as socket (socket.label)}
          <a
            href={categoryHref("network")}
            class="badge-link"
            title={socket.status?.problem
              ? text.socketBadge.notOpen(socket.status.problem)
              : socket.status?.receiving
                ? text.socketBadge.receiving(socket.status.address)
                : text.socketBadge.silent}
          >
            <Badge
              rounded
              color={socket.status?.problem
                ? "yellow"
                : socket.status?.receiving
                  ? "green"
                  : "gray"}
            >
              {socket.label} ({socket.status?.address ?? socket.fallback})
            </Badge>
          </a>
        {/each}
        <Badge rounded color={CONNECTION_COLOR[$connectionState]}>
          User Interface - {$connectionState.toUpperCase()} ({location.host})
        </Badge>
      </div>

      <!-- One flex item, so the Navbar's spacing keeps the two together on
           the right. -->
      <div class="flex items-center gap-2">
        {#if config.doc}
          <Toggle
            size="small"
            checked={config.doc.competitionField === true}
            title={text.competitionField.title}
            onchange={(e: Event) => {
              setCompetitionField(
                (e.currentTarget as HTMLInputElement).checked,
              );
            }}
            ><span class="text-sm whitespace-nowrap"
              >{text.competitionField.label}</span
            ></Toggle
          >
        {/if}
        <DarkMode
          class="flex h-8 w-8 items-center justify-center rounded-full border border-gray-200 p-0 dark:border-gray-600"
          size="sm"
          ariaLabel="Toggle dark mode"
        />

        <div class="settings">
          <Button
            id="settings-trigger"
            color="alternative"
            size="xs"
            pill
            class="relative h-8 w-8 p-0 text-base"
            aria-label="Settings"
          >
            <!-- U+FE0E asks for the plain text glyph; without it the button's
             font stack picks the color emoji. -->
            <span class="text-lg leading-none text-gray-700 dark:text-gray-300"
              >⚙︎</span
            >{#if unsaved > 0}<Indicator
                color="primary"
                size="sm"
                placement="top-right"
                title="Unsaved changes"
              />{/if}
          </Button>

          <Dropdown
            placement="bottom-end"
            triggeredBy="#settings-trigger"
            bind:isOpen={settingsOpen}
          >
            <DropdownHeader class="px-3">
              <span class="block text-xs text-gray-500 dark:text-gray-400"
                >Configuration</span
              >
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
                <Kbd class="px-1.5 py-0.5 text-xs font-normal">Ctrl+S</Kbd>
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
              <Checkbox bind:checked={preferences.expertUser}
                >Expert mode</Checkbox
              >
            </DropdownGroup>
            <DropdownGroup>
              <DropdownItem
                class={ITEM_CLASS}
                href={links.repository}
                target="_blank"
                rel="noopener noreferrer">GitHub Repository ↗︎</DropdownItem
              >
              <DropdownItem
                class={ITEM_CLASS}
                href={links.issues}
                target="_blank"
                rel="noopener noreferrer">Report An Issue ↗︎</DropdownItem
              >
            </DropdownGroup>
          </Dropdown>
        </div>
      </div>
    </Navbar>
  </header>

  <Sidebar
    position="static"
    alwaysOpen
    disableBreakpoints
    backdrop={false}
    activateClickOutside={false}
    ariaLabel="Cameras and settings"
    class="sidebar w-full"
    classes={{
      div: "h-full overflow-y-auto bg-white px-3 py-4 dark:bg-gray-800",
      active:
        "rounded-md bg-primary-100 dark:bg-primary-900 px-2 py-1.5 text-sm font-semibold hover:bg-primary-100 dark:hover:bg-primary-900",
      nonactive: "rounded-md px-2 py-1.5 text-sm",
    }}
  >
    <AlertSummary />
    <InstanceList />
    <ConfigNav />
  </Sidebar>

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
  .shell {
    display: grid;
    grid-template-columns: 280px 1fr;
    grid-template-rows: auto 1fr;
    min-height: 100vh;
  }

  header {
    grid-column: 1 / -1;
    border-bottom: 1px solid var(--color-gray-200);
  }

  :global(.dark) header,
  :global(.dark .sidebar),
  :global(.dark) .below {
    border-color: var(--color-gray-700);
  }

  /* Only a link around the badge inside it. A flex box rather than an
     inline <a>, which would sit the badge on a text baseline, lower than the
     connection badge beside it. */
  .badge-link {
    display: flex;
  }

  .badge-link:hover {
    filter: brightness(0.95);
  }

  .settings {
    position: relative;
  }

  /* The Sidebar's <aside>, passed down as a class. */
  :global(.sidebar) {
    border-right: 1px solid var(--color-gray-200);
  }

  main {
    overflow-y: auto;
  }

  .below {
    margin-top: 2rem;
    padding: 1rem 1.5rem;
    border-top: 1px solid var(--color-gray-200);
  }
</style>
