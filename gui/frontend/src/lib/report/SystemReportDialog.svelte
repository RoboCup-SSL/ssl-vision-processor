<script lang="ts">
  import {
    Button,
    Checkbox,
    Helper,
    List,
    Li,
    Modal,
    P,
    Spinner,
  } from "flowbite-svelte";
  import RichText from "../RichText.svelte";
  import StatusAlert from "../StatusAlert.svelte";
  import { about } from "../text/about";
  import { issueUrl } from "./issue";
  import {
    buildReport,
    downloadReport,
    isUnavailable,
    reportFileName,
    type SystemReport,
  } from "./systemReport";

  const text = about.report;

  // Says what the report holds before it's made, then shows the very report
  // that Download saves: gathered once per open (and again when the
  // configuration checkbox changes), so what was read is what gets attached.
  interface Props {
    open?: boolean;
  }

  let { open = $bindable(false) }: Props = $props();

  let report = $state<SystemReport | null>(null);
  let json = $derived(report ? JSON.stringify(report, null, 2) : "");
  let showing = $state(false);
  let copied = $state(false);
  // Off each time it opens: the configuration is the most a user might not
  // want to post publicly.
  let includeConfig = $state(false);
  // Only the latest gathering's report is shown, if the checkbox is toggled
  // while one is still running.
  let generation = 0;

  $effect(() => {
    if (!open) {
      includeConfig = false;
      return;
    }

    const mine = ++generation;
    report = null;
    copied = false;
    void buildReport({ includeConfig }).then((r) => {
      if (mine === generation) report = r;
    });
  });

  let newIssue = $derived(
    issueUrl(report && !isUnavailable(report.host) ? report.host.build : null),
  );

  // The Clipboard API needs a secure context: localhost, not the host's LAN
  // address over plain http.
  const canCopy = isSecureContext && "clipboard" in navigator;

  async function copy(): Promise<void> {
    await navigator.clipboard.writeText(json);
    copied = true;
  }
</script>

<Modal bind:open title={text.title} size="lg" class="fixed inset-0 m-auto">
  <P class="text-sm">{text.intro}</P>
  <div>
    <P class="text-sm">{text.contains}</P>
    <List class="mt-1 space-y-1 text-sm text-gray-600 dark:text-gray-400">
      {#each text.sections as section (section)}
        <Li>{section}</Li>
      {/each}
    </List>
  </div>
  <P class="text-sm text-gray-600 dark:text-gray-400">
    <RichText text={text.notIncluded} />
  </P>
  <div>
    <Checkbox bind:checked={includeConfig}>{text.includeConfig}</Checkbox>
    <Helper class="ms-6">{text.includeConfigNote}</Helper>
  </div>
  <StatusAlert color="yellow" class="p-2 text-sm">{text.privacy}</StatusAlert>

  {#if report}
    <div>
      <Button
        color="alternative"
        size="xs"
        onclick={() => {
          showing = !showing;
        }}>{showing ? text.hideContents : text.showContents}</Button
      >
      <span class="ms-2 font-mono text-xs text-gray-500 dark:text-gray-400"
        >{reportFileName(report)}</span
      >
      {#if showing}
        <pre
          class="mt-2 max-h-80 overflow-auto rounded bg-gray-100 p-2 text-xs dark:bg-gray-800"
          aria-label="Report contents">{json}</pre>
      {/if}
    </div>
  {:else}
    <P class="flex items-center gap-2 text-sm"
      ><Spinner size="4" /> {text.gathering}</P
    >
  {/if}

  <div
    class="flex justify-end gap-2 border-t border-gray-200 pt-4 dark:border-gray-700"
  >
    <Button
      color="alternative"
      class="me-auto"
      href={newIssue}
      target="_blank"
      rel="noopener noreferrer">{text.newIssue}</Button
    >
    <Button
      color="alternative"
      onclick={() => {
        open = false;
      }}>Cancel</Button
    >
    {#if canCopy}
      <Button color="alternative" disabled={!report} onclick={copy}
        >{copied ? text.copied : "Copy"}</Button
      >
    {/if}
    <Button
      disabled={!report}
      onclick={() => {
        if (report) downloadReport(report);
        open = false;
      }}>Download</Button
    >
  </div>
</Modal>
