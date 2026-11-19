<script lang="ts">
  import { onMount } from "svelte";
  import Shell from "./lib/layout/Shell.svelte";
  import SetupWizard from "./lib/wizard/SetupWizard.svelte";
  import UpdateNotice from "./lib/UpdateNotice.svelte";
  import { loadConfig } from "./lib/config.svelte";

  onMount(() => {
    void loadConfig();
  });
</script>

<Shell />

<SetupWizard />
<UpdateNotice />
