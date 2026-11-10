<script lang="ts">
  import ConfirmModal from "../ConfirmModal.svelte";
  import { connectionState } from "../wrapper-bus";
  import { preferences } from "../preferences.svelte";
  import InstanceList from "./InstanceList.svelte";
  import AlertSummary from "../alerts/AlertSummary.svelte";
  import ChecklistPill from "../checklist/ChecklistPill.svelte";
  import NetworkStatus from "./NetworkStatus.svelte";
  import ChecklistPanel from "../checklist/ChecklistPanel.svelte";
  import { checklist, openChecklist } from "../checklist/checklist.svelte";
  import { checklist as checklistText } from "../text/checklist";
  import { app as text } from "../text/app";
  import { links } from "../text/links";
  import { setCompetitionField } from "../teams.svelte";
  import ConfigNav from "./ConfigNav.svelte";
  import MainContent from "./MainContent.svelte";
  import { untrack, type Snippet } from "svelte";
  import { CogOutline } from "flowbite-svelte-icons";
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
  import AboutDialog from "../report/AboutDialog.svelte";
  import SystemReportDialog from "../report/SystemReportDialog.svelte";
  import { hostSystem, loadHostSystem } from "../report/hostSystem.svelte";
  import { issueUrl } from "../report/issue";
  import { headerSockets } from "../network.svelte";
  import LogPanel from "../logs/LogPanel.svelte";
  import { logs as logsText } from "../text/logs";

  // `below` is where App.svelte puts the existing snapshot grid / WS dev
  // panel for now, until they become the mockup's real Video panel (issue
  // #18) -- see the note there. The Debug Console is the log panel below. Keeping it a slot rather than baking
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

  // The setup checklist is for new users: expert mode hides it, its pill and
  // its menu entries. Its open state and ticks are kept for when it's off.
  let checklistOpen = $derived(!preferences.expertUser && checklist.open);

  let unsaved = $derived(config.state?.changes.length ?? 0);

  // The log panel is fixed to the bottom of the window; the page leaves room
  // for it at its current height, so the end of every page can still be
  // scrolled into view.
  let logHeight = $state(0);

  let sockets = $derived(headerSockets());

  // The socket badges show their addresses while the header has room for
  // them, since teams read where to connect from there, and fold into one
  // Network dropdown when it hasn't. Measured rather than a breakpoint, as
  // addresses vary in length: the width the full badges needed is
  // remembered, and they come back once the header is that wide again.
  let headerLeft = $state<HTMLElement>();
  let socketsCollapsed = $state(false);
  let expandedWidth = 0;

  function fitSockets(): void {
    if (!headerLeft) return;

    if (!socketsCollapsed && headerLeft.scrollWidth > headerLeft.clientWidth) {
      expandedWidth = headerLeft.scrollWidth;
      socketsCollapsed = true;
    } else if (socketsCollapsed && headerLeft.clientWidth >= expandedWidth) {
      socketsCollapsed = false;
    }
  }

  $effect(() => {
    if (!headerLeft) return;

    const observer = new ResizeObserver(fitSockets);
    observer.observe(headerLeft);

    return () => {
      observer.disconnect();
    };
  });

  // The group's own size doesn't change when its contents do, so measure
  // again after expanding, and when an address changes length.
  $effect(() => {
    void socketsCollapsed;
    void sockets.map((s) => s.status?.address ?? s.fallback).join();
    untrack(fitSockets);
  });

  // Matches the preference checkboxes below: same text, same left inset as
  // their p-3 group.
  const ITEM_CLASS =
    "flex items-center justify-between gap-4 px-3 py-1.5 text-sm font-medium text-gray-900 dark:text-white";

  function openDialog(which: "save" | "saveAs" | "load"): void {
    settingsOpen = false;
    fileDialogs[which] = true;
  }

  let confirmingRevert = $state(false);

  let aboutOpen = $state(false);
  let reportOpen = $state(false);

  // Report An Issue starts an issue with the build filled in, so the build is
  // asked for when the menu opens, before the link can be clicked.
  $effect(() => {
    if (settingsOpen) void loadHostSystem();
  });
  let newIssue = $derived(issueUrl(hostSystem.info?.build ?? null));

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

<AboutDialog
  bind:open={aboutOpen}
  onreport={() => {
    reportOpen = true;
  }}
/>
<SystemReportDialog bind:open={reportOpen} />

<div
  class="shell"
  class:with-checklist={checklistOpen}
  style:--log-height={preferences.logPanel ? `${String(logHeight)}px` : null}
>
  <header>
    <!-- Never wraps: a second row would push the page down and split the
         right-hand controls from each other. -->
    <Navbar
      fluid
      class="px-6 py-2"
      navContainerClass="flex-nowrap gap-4"
      closeOnClickOutside={false}
    >
      <div
        bind:this={headerLeft}
        class="flex min-w-0 flex-1 items-center gap-3 overflow-hidden"
      >
        <NavBrand>
          <h1 class="text-lg font-semibold whitespace-nowrap">
            SSL Vision Processor
          </h1>
        </NavBrand>
        <NetworkStatus {sockets} collapsed={socketsCollapsed} />
        <Badge
          rounded
          color={CONNECTION_COLOR[$connectionState]}
          class="shrink-0 whitespace-nowrap"
        >
          User Interface - {$connectionState.toUpperCase()} ({location.host})
        </Badge>
      </div>

      <!-- One flex item, so the Navbar's spacing keeps the two together on
           the right. -->
      <div class="flex shrink-0 items-center gap-2">
        {#if config.doc}
          <!-- A pill like its neighbours: class lands on Toggle's <label>, so
               the whole pill is the click target. Colors are Button's
               "alternative" ones. -->
          <Toggle
            size="small"
            class="h-8 shrink-0 cursor-pointer rounded-full border border-gray-200 px-3 text-gray-900 hover:bg-gray-100 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-400 dark:hover:bg-gray-700 dark:hover:text-white"
            checked={config.doc.competitionField === true}
            title={text.competitionField.title}
            onchange={(e: Event) => {
              setCompetitionField(
                (e.currentTarget as HTMLInputElement).checked,
              );
            }}
            ><span class="text-xs font-medium whitespace-nowrap"
              >{text.competitionField.label}</span
            ></Toggle
          >
        {/if}
        {#if !preferences.expertUser && (preferences.showChecklist || checklist.open)}
          <ChecklistPill />
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
            <!-- An SVG, not the ⚙ character: a font's glyph doesn't sit at
                 the center of its box, so it never quite centered here. -->
            <CogOutline
              class="text-gray-700 dark:text-gray-300"
              size="md"
            />{#if unsaved > 0}<Indicator
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
            {#if !preferences.expertUser}
              <DropdownGroup>
                <DropdownItem
                  class={ITEM_CLASS}
                  onclick={() => {
                    settingsOpen = false;
                    openChecklist();
                  }}>{checklistText.menuOpen}</DropdownItem
                >
              </DropdownGroup>
            {/if}
            <DropdownGroup class="flex flex-col gap-2 p-3">
              {#if !preferences.expertUser}
                <Checkbox bind:checked={preferences.showChecklist}
                  >{checklistText.menuShow}</Checkbox
                >
              {/if}
              <Checkbox bind:checked={preferences.logPanel}
                >{logsText.menuShow}</Checkbox
              >
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
                href={links.apiDocs}
                target="_blank"
                rel="noopener noreferrer">API Reference ↗︎</DropdownItem
              >
              <DropdownItem
                class={ITEM_CLASS}
                href={links.repository}
                target="_blank"
                rel="noopener noreferrer">GitHub Repository ↗︎</DropdownItem
              >
              <DropdownItem
                class={ITEM_CLASS}
                href={newIssue}
                target="_blank"
                rel="noopener noreferrer">Report An Issue ↗︎</DropdownItem
              >
            </DropdownGroup>
            <DropdownGroup>
              <DropdownItem
                class={ITEM_CLASS}
                onclick={() => {
                  settingsOpen = false;
                  reportOpen = true;
                }}>System report…</DropdownItem
              >
              <DropdownItem
                class={ITEM_CLASS}
                onclick={() => {
                  settingsOpen = false;
                  aboutOpen = true;
                }}>About…</DropdownItem
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

  {#if checklistOpen}
    <ChecklistPanel />
  {/if}
</div>

{#if preferences.logPanel}
  <div class="log-dock" bind:clientHeight={logHeight}>
    <LogPanel />
  </div>
{/if}

<style>
  .shell {
    padding-bottom: var(--log-height, 0px);
    display: grid;
    grid-template-columns: 280px 1fr;
    grid-template-rows: auto 1fr;
    min-height: 100vh;
  }

  /* The setup checklist docks as a third column, only while it's open. */
  .shell.with-checklist {
    grid-template-columns: 280px 1fr 360px;
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

  .settings {
    position: relative;
  }

  /* Over the page, under Flowbite's dropdowns and modals (z-50). */
  .log-dock {
    position: fixed;
    right: 0;
    bottom: 0;
    left: 0;
    z-index: 40;
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
