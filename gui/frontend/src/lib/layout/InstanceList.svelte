<script lang="ts">
  import { nav } from "./nav.svelte";
</script>

<section class="instance-list">
  <h2>Vision processors</h2>
  <p class="hint">
    One row per camera role, not per machine -- a host running all 4 cameras of
    a quad setup appears here 4 times. Mock data for now; see internal/discovery
    in gui/CLAUDE.md's "Not yet built".
  </p>

  <ul>
    {#each nav.instances as instance (instance.id)}
      <li>
        <button
          type="button"
          class:selected={instance.id === nav.selectedInstanceId}
          onclick={() => {
            nav.selectedInstanceId = instance.id;
          }}
        >
          <span class="host">{instance.host}</span>
          <span class="cam">cam {instance.cameraId}</span>
        </button>
      </li>
    {/each}
  </ul>
</section>

<style>
  .instance-list {
    padding: 0.75rem;
    border-bottom: 1px solid #ddd;
  }

  h2 {
    font-size: 0.85rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    margin: 0 0 0.4rem;
    color: #555;
  }

  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  button {
    display: flex;
    justify-content: space-between;
    width: 100%;
    padding: 0.4rem 0.5rem;
    border: none;
    background: none;
    text-align: left;
    font-size: 0.85rem;
    border-radius: 4px;
    cursor: pointer;
  }

  button:hover {
    background: #f0f0f0;
  }

  button.selected {
    background: #dbe9ff;
    font-weight: 600;
  }

  .cam {
    color: #666;
  }

  .hint {
    color: #888;
    font-size: 0.75rem;
    font-style: italic;
    margin: 0 0 0.5rem;
  }
</style>
