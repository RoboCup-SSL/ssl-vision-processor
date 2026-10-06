<script lang="ts">
  import { Badge, Button, Heading, Modal, P } from "flowbite-svelte";
  import SettingsCard from "../SettingsCard.svelte";
  import { alertsOf, SEVERITIES, type Severity } from "./alerts";
  import { RESOLUTION_DOCS, type DocId } from "../text/docs";
  import { alerts as text } from "../text/alerts";

  // Left border per severity, so a long list still reads by color.
  const BORDER: Record<Severity, string> = {
    error: "border-red-500",
    warning: "border-orange-400",
    caution: "border-yellow-400",
  };

  const EMPTY: Record<Severity, string> = text.empty;

  let openDoc = $state<DocId | null>(null);
  let doc = $derived(openDoc ? RESOLUTION_DOCS[openDoc] : null);
</script>

<div class="max-w-5xl">
  <Heading tag="h2" class="mb-2 text-xl font-semibold">{text.heading}</Heading>
  <P size="sm" class="mb-2 text-gray-600 dark:text-gray-400">
    {text.intro}
  </P>

  {#each SEVERITIES as { id, label } (id)}
    {@const list = alertsOf(id)}
    <div id={`alerts-${id}`} class="scroll-mt-4">
      <SettingsCard title={`${label} (${String(list.length)})`}>
        {#if list.length === 0}
          <P size="sm" class="text-gray-500 dark:text-gray-400">{EMPTY[id]}</P>
        {:else}
          <ul class="flex flex-col gap-1">
            {#each list as alert (alert.id)}
              <li
                class={`flex flex-wrap items-center gap-x-3 gap-y-1 rounded border border-s-4 border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 px-3 py-1.5 ${BORDER[id]}`}
              >
                <div class="min-w-0 flex-1">
                  <div class="flex flex-wrap items-center gap-1.5">
                    <span class="text-sm font-semibold">{alert.title}</span>
                    {#if alert.cameraId !== undefined}
                      <Badge color="gray" class="px-1.5 py-0 text-xs"
                        >camera {alert.cameraId}</Badge
                      >
                    {/if}
                  </div>
                  <p class="text-xs text-gray-600 dark:text-gray-400">
                    {alert.detail}
                  </p>
                </div>
                {#if alert.actions.length > 0}
                  <div class="flex flex-wrap gap-1.5">
                    {#each alert.actions as action (action.label)}
                      {#if action.kind === "doc"}
                        <Button
                          size="xs"
                          onclick={() => {
                            openDoc = action.doc;
                          }}>{action.label}</Button
                        >
                      {:else if action.kind === "page"}
                        <Button size="xs" color="alternative" href={action.href}
                          >{action.label} →</Button
                        >
                      {:else}
                        <Button
                          size="xs"
                          color="alternative"
                          href={action.url}
                          target="_blank"
                          rel="noopener">{action.label} ↗</Button
                        >
                      {/if}
                    {/each}
                  </div>
                {/if}
              </li>
            {/each}
          </ul>
        {/if}
      </SettingsCard>
    </div>
  {/each}
</div>

<Modal
  open={doc !== null}
  title={doc?.title ?? ""}
  size="md"
  class="fixed inset-0 m-auto"
  onclose={() => {
    openDoc = null;
  }}
>
  {#if doc}
    <p class="text-sm text-gray-600 dark:text-gray-400">{doc.cause}</p>
    <ol class="ms-5 list-decimal space-y-1 text-sm">
      {#each doc.steps as step, i (i)}
        {#if step.startsWith("$ ")}
          <li class="list-none">
            <pre
              class="rounded bg-gray-100 dark:bg-gray-700 px-2 py-1 text-xs">{step.slice(
                2,
              )}</pre>
          </li>
        {:else}
          <li>{step}</li>
        {/if}
      {/each}
    </ol>
  {/if}
  <div
    class="flex justify-end border-t border-gray-200 dark:border-gray-700 pt-4"
  >
    <Button
      color="alternative"
      onclick={() => {
        openDoc = null;
      }}>Close</Button
    >
  </div>
</Modal>
