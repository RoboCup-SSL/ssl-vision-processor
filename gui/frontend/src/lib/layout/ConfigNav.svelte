<script lang="ts">
  import { nav, selectedInstance } from "./nav.svelte";
  import { CONFIG_CATEGORIES } from "./configCategories";

  let sharedCategories = $derived(
    CONFIG_CATEGORIES.filter((c) => c.scope === "shared"),
  );
  let instanceCategories = $derived(
    CONFIG_CATEGORIES.filter((c) => c.scope === "per-instance"),
  );

  // Computed once, matching MainContent.svelte's own pattern for the same
  // function, rather than calling selectedInstance() again at each of the
  // template sites below.
  let instance = $derived(selectedInstance());
</script>

<nav class="config-nav">
  <h2>Shared</h2>
  <ul>
    {#each sharedCategories as category (category.id)}
      <li>
        <button
          type="button"
          class:selected={category.id === nav.selectedCategoryId}
          onclick={() => {
            nav.selectedCategoryId = category.id;
          }}
        >
          {category.label}
        </button>
      </li>
    {/each}
  </ul>

  <h2>
    {#if instance}
      {instance.host} / cam {instance.cameraId}
    {:else}
      Per-instance (select one above)
    {/if}
  </h2>
  <ul>
    {#each instanceCategories as category (category.id)}
      <li>
        <button
          type="button"
          class:selected={category.id === nav.selectedCategoryId}
          disabled={!instance}
          onclick={() => {
            nav.selectedCategoryId = category.id;
          }}
        >
          {category.label}
        </button>
      </li>
    {/each}
  </ul>
</nav>

<style>
  .config-nav {
    padding: 0.75rem;
    overflow-y: auto;
  }

  h2 {
    font-size: 0.85rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    margin: 0.75rem 0 0.4rem;
    color: #555;
  }

  h2:first-child {
    margin-top: 0;
  }

  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  button {
    display: block;
    width: 100%;
    padding: 0.35rem 0.5rem;
    border: none;
    background: none;
    text-align: left;
    font-size: 0.85rem;
    border-radius: 4px;
    cursor: pointer;
  }

  button:hover:not(:disabled) {
    background: #f0f0f0;
  }

  button.selected {
    background: #dbe9ff;
    font-weight: 600;
  }

  button:disabled {
    color: #bbb;
    cursor: not-allowed;
  }
</style>
