<script lang="ts">
  import { connectionState } from "../wrapper-bus";
  import { preferences } from "../preferences.svelte";
  import InstanceList from "./InstanceList.svelte";
  import ConfigNav from "./ConfigNav.svelte";
  import MainContent from "./MainContent.svelte";
  import type { Snippet } from "svelte";

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
  // rather than wherever happened to need it first.
  let settingsOpen = $state(false);
  let settingsEl: HTMLDivElement | undefined = $state();

  function toggleSettings(): void {
    settingsOpen = !settingsOpen;
  }

  // Closes on any click outside the button+menu -- a click on the gear
  // itself is inside settingsEl too, so this never fights toggleSettings's
  // own open/close.
  function handleWindowClick(event: MouseEvent): void {
    if (settingsOpen && !settingsEl?.contains(event.target as Node)) {
      settingsOpen = false;
    }
  }
</script>

<svelte:window onclick={handleWindowClick} />

<div class="shell">
  <header>
    <h1>vision-processor</h1>
    <span class="badge" data-state={$connectionState}>
      {$connectionState.toUpperCase()} ({location.host})
    </span>

    <div class="settings" bind:this={settingsEl}>
      <button
        type="button"
        class="settings-button"
        aria-label="Settings"
        onclick={toggleSettings}
      >
        ⚙
      </button>

      {#if settingsOpen}
        <div class="settings-menu">
          <label>
            <input type="checkbox" bind:checked={preferences.tooltipsEnabled} />
            Show extra tooltips
          </label>
          <label>
            <input type="checkbox" bind:checked={preferences.expertUser} />
            Expert mode
          </label>
        </div>
      {/if}
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

  .settings-menu {
    position: absolute;
    top: calc(100% + 0.5rem);
    right: 0;
    z-index: 20;
    min-width: 180px;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    padding: 0.6rem 0.75rem;
    background: white;
    border: 1px solid #ccc;
    border-radius: 6px;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
    font-size: 0.85rem;
    color: #333;
  }

  .settings-menu label {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    cursor: pointer;
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
