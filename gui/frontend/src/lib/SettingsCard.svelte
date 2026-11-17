<script lang="ts">
  import type { Snippet } from "svelte";
  import { Card, Heading } from "flowbite-svelte";

  // A titled group of settings in a panel: what each panel's sections (Device,
  // Brightness, Dimensions, ...) are drawn as, so they look the same
  // everywhere.
  interface Props {
    title: string;
    // Shown before the title, e.g. a status indicator.
    status?: Snippet;
    class?: string;
    children: Snippet;
  }

  let { title, status, class: className = "", children }: Props = $props();
</script>

<Card size="xl" shadow="xs" class={`mb-4 gap-1 p-4 ${className}`}>
  <Heading
    tag="h3"
    class="mb-1 flex items-center gap-2 text-base font-semibold"
  >
    {@render status?.()}
    {title}
  </Heading>
  {@render children()}
</Card>
