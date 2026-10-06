<script lang="ts">
  import { Button, Modal, P } from "flowbite-svelte";

  // A yes/no question as a Flowbite Modal, in place of the browser's
  // confirm(): it matches the rest of the GUI (dark mode included) and
  // doesn't block the page.
  interface Props {
    open?: boolean;
    title: string;
    message: string;
    confirmLabel: string;
    cancelLabel?: string;
    // A destructive action gets a red button.
    danger?: boolean;
    onconfirm: () => void;
  }

  let {
    open = $bindable(false),
    title,
    message,
    confirmLabel,
    cancelLabel = "Cancel",
    danger = false,
    onconfirm,
  }: Props = $props();
</script>

<Modal bind:open {title} size="sm" class="fixed inset-0 m-auto">
  <P class="text-sm text-gray-600 dark:text-gray-400">{message}</P>
  <div
    class="flex justify-end gap-2 border-t border-gray-200 pt-4 dark:border-gray-700"
  >
    <Button
      color="alternative"
      onclick={() => {
        open = false;
      }}>{cancelLabel}</Button
    >
    <Button
      color={danger ? "red" : "primary"}
      onclick={() => {
        open = false;
        onconfirm();
      }}>{confirmLabel}</Button
    >
  </div>
</Modal>
