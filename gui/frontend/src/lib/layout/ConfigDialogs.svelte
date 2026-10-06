<script lang="ts">
  import RichText from "../RichText.svelte";
  import { app } from "../text/app";
  const text = app.dialogs;
  import { Modal, Button, Input, Label, Alert, Toast } from "flowbite-svelte";
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

<!-- Keyed on the message, so each new error is a fresh toast rather than one
     that was dismissed staying hidden. -->
{#if config.error}
  {#key config.error}
    <Toast
      color="red"
      class="fixed end-4 bottom-4 z-60 max-w-lg whitespace-pre-wrap"
      closeAriaLabel="Dismiss"
      onclose={() => {
        config.error = null;
      }}
    >
      {config.error}
    </Toast>
  {/key}
{/if}

<Modal
  bind:open={fileDialogs.save}
  title="Save configuration"
  size="lg"
  class={MODAL_CLASS}
>
  <p class="text-sm text-gray-600 dark:text-gray-400">
    <RichText text={text.writesTo(config.state?.path ?? "")} />
  </p>

  {#if changes.length === 0}
    <p class="text-sm">{text.noChanges}</p>
  {:else}
    <ChangeList {changes} />
  {/if}

  <div
    class="flex justify-end gap-2 border-t border-gray-200 dark:border-gray-700 pt-4"
  >
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
  <div class="flex flex-col gap-1">
    <Label for="save-as-path">Path on the host</Label>
    <Input
      id="save-as-path"
      type="text"
      class="font-mono"
      bind:value={pathInput}
      placeholder="vision.yml"
    />
  </div>
  <p class="text-sm text-gray-600 dark:text-gray-400">
    {text.saveAsNote}
  </p>

  <div
    class="flex justify-end gap-2 border-t border-gray-200 dark:border-gray-700 pt-4"
  >
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
  <div class="flex flex-col gap-1">
    <Label for="load-path">Path on the host</Label>
    <Input
      id="load-path"
      type="text"
      class="font-mono"
      bind:value={pathInput}
      placeholder="vision.yml"
    />
  </div>

  {#if changes.length > 0}
    <Alert color="yellow" class="p-2 text-sm">
      {text.loadDiscards(changes.length)}
    </Alert>
  {:else}
    <p class="text-sm text-gray-600 dark:text-gray-400">
      {text.loadApplies}
    </p>
  {/if}

  <div
    class="flex justify-end gap-2 border-t border-gray-200 dark:border-gray-700 pt-4"
  >
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
    <RichText text={text.editedOutside(config.state?.path ?? "")} />
  </p>

  {#if external?.error}
    <Alert color="red" class="p-2 text-sm">
      {text.cantLoad}
      <pre class="error-detail">{external.error}</pre>
    </Alert>
  {:else if external && external.changes.length > 0}
    <p class="text-sm text-gray-600 dark:text-gray-400">
      {text.wouldChange}
    </p>
    <ChangeList changes={external.changes} />
  {:else}
    <p class="text-sm text-gray-600 dark:text-gray-400">
      {text.formattingOnly}
    </p>
  {/if}

  {#if changes.length > 0}
    <Alert color="yellow" class="p-2 text-sm">
      {text.unsavedConflict(changes.length)}
    </Alert>
  {/if}

  <div
    class="flex justify-end gap-2 border-t border-gray-200 dark:border-gray-700 pt-4"
  >
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
  .error-detail {
    max-height: 30vh;
    overflow: auto;
    margin-top: 0.4rem;
    padding: 0.5rem;
    border-radius: 4px;
    background: rgb(255 255 255 / 0.6);
  }

  :global(.dark) .error-detail {
    background: rgb(0 0 0 / 0.3);
    font-size: 0.8rem;
    white-space: pre-wrap;
  }
</style>
