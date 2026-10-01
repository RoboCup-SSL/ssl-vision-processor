<script lang="ts">
  import { connectionState } from "../wrapper-bus";
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
</script>

<div class="shell">
  <header>
    <h1>vision-processor</h1>
    <span class="badge" data-state={$connectionState}>
      {$connectionState.toUpperCase()} ({location.host})
    </span>
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
