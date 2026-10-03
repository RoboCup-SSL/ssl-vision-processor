<script lang="ts">
  import { connectionState } from "../wrapper-bus";
  import { preferences } from "../preferences.svelte";
  import InstanceList from "./InstanceList.svelte";
  import ConfigNav from "./ConfigNav.svelte";
  import MainContent from "./MainContent.svelte";
  import type { Snippet } from "svelte";
  import { Dropdown, DropdownGroup, Checkbox } from "flowbite-svelte";

  // `below` is where App.svelte puts the existing snapshot grid / WS dev
  // panel for now, until they become the mockup's real Video + Debug Console
  // (issue #18) -- see the note there. Keeping it a slot rather than baking
  // it into this component so Shell stays just the layout, not a dumping
  // ground for whatever hasn't found a permanent home yet.
  interface Props {
    below?: Snippet;
  }

  let { below }: Props = $props();

  // Global settings menu (gear, top-right) -- preferences (../preferences.svelte)
  // is app-wide state, not scoped to any one tab, so it belongs in the shell
  // rather than wherever happened to need it first. Dropdown (flowbite-svelte)
  // owns open/close and outside-click dismissal itself now -- previously
  // hand-rolled here with a window click listener.
  let settingsOpen = $state(false);
</script>

<div class="shell">
  <header>
    <h1>vision-processor</h1>
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
        ⚙
      </button>

      <Dropdown
        placement="bottom-end"
        triggeredBy="#settings-trigger"
        bind:isOpen={settingsOpen}
      >
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
