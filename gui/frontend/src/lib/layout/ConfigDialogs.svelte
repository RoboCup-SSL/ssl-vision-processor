<script lang="ts">
  import { Modal, Button } from "flowbite-svelte";
  import {
    config,
    saveConfig,
    saveConfigAs,
    loadConfigFrom,
    reloadFromDisk,
  } from "../config.svelte";
  import { fileDialogs } from "./fileDialogs.svelte";
  import ChangeList from "./ChangeList.svelte";

  // Mounted once at the shell root. The Modal class pins position/centering
  // itself -- flowbite-svelte's dialog theme ships with its fixed-position
  // variant commented out (see wizard/SetupWizard.svelte).
  const MODAL_CLASS = "fixed inset-0 m-auto";

  let changes = $derived(config.state?.changes ?? []);
  let external = $derived(config.state?.external);
  let externalKey = $derived(external ? JSON.stringify(external) : "");
  let externalOpen = $derived(
    externalKey !== "" && externalKey !== fileDialogs.dismissedExternal,
  );

  let pathInput = $state("");

  $effect(() => {
    if (fileDialogs.saveAs || fileDialogs.load) {
      pathInput = config.state?.path ?? "";
    }
  });

  function onKeydown(e: KeyboardEvent): void {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") {
      e.preventDefault();
      fileDialogs.save = true;
    }
  }

  async function confirmSave(): Promise<void> {
    const conflict = await saveConfig();
    fileDialogs.save = false;

    // A disk_changed conflict leaves state.external set, which opens the
    // external-change dialog below on its own.
    if (conflict && conflict.error !== "disk_changed") {
      config.error = conflict.message;
    }
  }

  async function confirmSaveAs(): Promise<void> {
    if (!pathInput) return;

    await saveConfigAs(pathInput);
    if (!config.error) fileDialogs.saveAs = false;
  }

  async function confirmLoad(): Promise<void> {
    if (!pathInput) return;

    await loadConfigFrom(pathInput);
    if (!config.error) fileDialogs.load = false;
  }

  async function overwriteDisk(): Promise<void> {
    await saveConfig(true);
  }

  function later(): void {
    fileDialogs.dismissedExternal = externalKey;
  }
</script>

<svelte:window onkeydown={onKeydown} />

{#if config.error}
  <div class="error-banner" role="alert">
    <span>{config.error}</span>
    <button
      type="button"
      class="dismiss"
      aria-label="Dismiss"
      onclick={() => {
        config.error = null;
      }}>×</button
    >
  </div>
{/if}

<Modal
  bind:open={fileDialogs.save}
  title="Save configuration"
  size="lg"
  class={MODAL_CLASS}
>
  <p class="text-sm text-gray-600">
    Writes to <code>{config.state?.path}</code>.
  </p>

  {#if changes.length === 0}
    <p class="text-sm">No unsaved changes.</p>
  {:else}
    <ChangeList {changes} />
  {/if}

  <div class="flex justify-end gap-2 border-t border-gray-200 pt-4">
    <Button
      color="alternative"
      onclick={() => {
        fileDialogs.save = false;
      }}>Cancel</Button
    >
    <Button
      disabled={changes.length === 0 || config.busy}
      onclick={confirmSave}
    >
      Save {changes.length} change{changes.length === 1 ? "" : "s"}
    </Button>
  </div>
</Modal>

<Modal
  bind:open={fileDialogs.saveAs}
  title="Save configuration as"
  size="md"
  class={MODAL_CLASS}
>
  <label class="flex flex-col gap-1 text-sm">
    Path on the host
    <input
      type="text"
      class="rounded border border-gray-300 p-1.5"
      bind:value={pathInput}
      placeholder="vision.yml"
    />
  </label>
  <p class="text-sm text-gray-600">
    The GUI edits and watches the new file from then on.
  </p>

  <div class="flex justify-end gap-2 border-t border-gray-200 pt-4">
    <Button
      color="alternative"
      onclick={() => {
        fileDialogs.saveAs = false;
      }}>Cancel</Button
    >
    <Button disabled={!pathInput || config.busy} onclick={confirmSaveAs}
      >Save</Button
    >
  </div>
</Modal>

<Modal
  bind:open={fileDialogs.load}
  title="Load configuration"
  size="md"
  class={MODAL_CLASS}
>
  <label class="flex flex-col gap-1 text-sm">
    Path on the host
    <input
      type="text"
      class="rounded border border-gray-300 p-1.5"
      bind:value={pathInput}
      placeholder="vision.yml"
    />
  </label>

  {#if changes.length > 0}
    <p class="text-sm text-red-700">
      Loading discards {changes.length} unsaved change{changes.length === 1
        ? ""
        : "s"}, and applies the loaded file live.
    </p>
  {:else}
    <p class="text-sm text-gray-600">The loaded file applies live.</p>
  {/if}

  <div class="flex justify-end gap-2 border-t border-gray-200 pt-4">
    <Button
      color="alternative"
      onclick={() => {
        fileDialogs.load = false;
      }}>Cancel</Button
    >
    <Button disabled={!pathInput || config.busy} onclick={confirmLoad}
      >Load</Button
    >
  </div>
</Modal>

<Modal
  open={externalOpen}
  title="Configuration changed on disk"
  size="lg"
  class={MODAL_CLASS}
  dismissable={false}
>
  <p class="text-sm">
    <code>{config.state?.path}</code> was edited outside the GUI.
  </p>

  {#if external?.error}
    <p class="text-sm text-red-700">It can't be loaded as it is:</p>
    <pre class="error-detail">{external.error}</pre>
  {:else if external && external.changes.length > 0}
    <p class="text-sm text-gray-600">Loading it would change:</p>
    <ChangeList changes={external.changes} />
  {:else}
    <p class="text-sm text-gray-600">
      The values match what's running; only formatting or comments differ.
    </p>
  {/if}

  {#if changes.length > 0}
    <p class="text-sm text-red-700">
      You have {changes.length} unsaved change{changes.length === 1 ? "" : "s"}.
      Loading discards them; overwriting replaces the edited file with them.
    </p>
  {/if}

  <div class="flex justify-end gap-2 border-t border-gray-200 pt-4">
    <Button color="alternative" onclick={later}>Later</Button>
    <Button color="alternative" disabled={config.busy} onclick={overwriteDisk}
      >Overwrite disk</Button
    >
    <Button
      disabled={Boolean(external?.error) || config.busy}
      onclick={reloadFromDisk}>Load from disk</Button
    >
  </div>
</Modal>

<style>
  .error-banner {
    position: fixed;
    right: 1rem;
    bottom: 1rem;
    z-index: 60;
    display: flex;
    align-items: flex-start;
    gap: 0.75rem;
    max-width: 32rem;
    padding: 0.75rem 1rem;
    border: 1px solid #f5c2c7;
    border-radius: 6px;
    background: #fdecee;
    color: #842029;
    font-size: 0.85rem;
    white-space: pre-wrap;
  }

  .dismiss {
    padding: 0 0.3rem;
    border: none;
    background: none;
    font-size: 1rem;
    line-height: 1;
  }

  .error-detail {
    max-height: 30vh;
    overflow: auto;
    padding: 0.5rem;
    border-radius: 4px;
    background: #f5f5f5;
    font-size: 0.8rem;
    white-space: pre-wrap;
  }
</style>
