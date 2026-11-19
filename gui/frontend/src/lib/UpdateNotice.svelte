<script lang="ts">
  import { Button } from "flowbite-svelte";
  import StatusAlert from "./StatusAlert.svelte";
  import { hostUpdated, hostVersion } from "./hostVersion.svelte";
  import { app } from "./text/app";
</script>

{#if hostUpdated()}
  <StatusAlert
    color="yellow"
    class="fixed start-1/2 top-4 z-60 flex max-w-xl -translate-x-1/2 items-center gap-3 shadow-lg"
  >
    <span class="text-sm"
      >{app.hostUpdated(hostVersion.page ?? "", hostVersion.host ?? "")}</span
    >
    <Button
      size="xs"
      onclick={() => {
        location.reload();
      }}>{app.reload}</Button
    >
  </StatusAlert>
{/if}
