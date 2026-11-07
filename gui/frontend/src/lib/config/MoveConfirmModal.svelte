<script lang="ts">
  import { Button, Li, List, Modal } from "flowbite-svelte";
  import StatusAlert from "../StatusAlert.svelte";
  import { layout as text } from "../text/layout";
  import type { ConfigDocument } from "../config.svelte";
  import {
    cameraDevice,
    deviceLabel,
    hostName,
    type PendingMove,
  } from "../cameraLayout";

  // Confirms a move, swap, or camera count change on the Camera Layout page,
  // listing what happens to each camera's settings, corners and calibration.
  interface Props {
    doc: ConfigDocument;
    // The change waiting on this dialog; null closes it.
    pending?: PendingMove | null;
    onapply: (move: PendingMove) => void;
  }

  let { doc, pending = $bindable(null), onapply }: Props = $props();
</script>

<Modal
  open={pending !== null}
  title={pending?.title ?? ""}
  size="md"
  class="fixed inset-0 m-auto"
  onclose={() => {
    pending = null;
  }}
>
  {#if pending}
    <List class="list-none space-y-2 text-sm">
      {#each pending.plan as step (step.from)}
        <Li>
          <span class="font-semibold">
            {`${hostName(step.camera)} · ${deviceLabel(cameraDevice(doc, step.camera))}:`}
          </span>
          {step.from !== step.to
            ? text.confirm.moved(step.from, step.to)
            : text.confirm.newRegion(step.from)}
          <List
            position="outside"
            class="ms-4 text-gray-600 dark:text-gray-400"
          >
            <Li>{text.confirm.settingsStay}</Li>
            {#if step.seedStale}
              <Li>{text.confirm.seedStale}</Li>
            {/if}
            {#if step.calibrationRemoved}
              <Li>{text.confirm.calibrationRemoved}</Li>
            {/if}
            {#if !step.camera.configPath}
              <Li>{text.confirm.remote}</Li>
            {/if}
          </List>
        </Li>
      {/each}
    </List>

    <StatusAlert color="yellow" class="mt-3 p-2 text-sm">
      {text.confirm.restart}
    </StatusAlert>
  {/if}

  <div
    class="flex justify-end gap-2 border-t border-gray-200 dark:border-gray-700 pt-4"
  >
    <Button
      color="alternative"
      onclick={() => {
        pending = null;
      }}>Cancel</Button
    >
    <Button
      onclick={() => {
        if (pending) onapply(pending);
        pending = null;
      }}>Apply</Button
    >
  </div>
</Modal>
