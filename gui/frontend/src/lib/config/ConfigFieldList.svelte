<script lang="ts">
  import type { ConfigField } from "../layout/configCategories";

  interface Props {
    fields: ConfigField[];
  }

  let { fields }: Props = $props();
</script>

<dl>
  {#each fields as field (field.name)}
    <div class="field">
      <dt>{field.name}</dt>
      <dd>{field.comment}</dd>
    </div>
  {/each}
</dl>

<style>
  dl {
    margin: 1rem 0 0;
  }

  .field {
    display: flex;
    gap: 1rem;
    padding: 0.35rem 0;
    border-bottom: 1px solid #eee;
    font-size: 0.85rem;
  }

  dt {
    flex: 0 0 12rem;
    font-family: monospace;
    font-weight: 600;
  }

  dd {
    margin: 0;
    color: #555;
  }
</style>
