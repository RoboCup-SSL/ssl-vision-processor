<script lang="ts">
  // Text from lib/text/ with `backticks` around code, shown as <code>. Built
  // from text nodes, never HTML, so a string can't inject markup.
  interface Props {
    text: string;
  }

  let { text }: Props = $props();

  // Odd parts sat between backticks.
  let parts = $derived(text.split("`"));
</script>

{#each parts as part, i (i)}{#if i % 2 === 1}<code>{part}</code
    >{:else}{part}{/if}{/each}
