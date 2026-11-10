<script lang="ts">
  import { AccordionItem, Button } from "flowbite-svelte";
  import { ArrowRightOutline } from "flowbite-svelte-icons";
  import { checklist as text } from "../text/checklist";
  import { categoryHref, navHref } from "../layout/nav.svelte";
  import { fileDialogs } from "../layout/fileDialogs.svelte";
  import { openWizard } from "../wizard/wizard.svelte";
  import { processes } from "../processes.svelte";
  import StatusIcon from "./StatusIcon.svelte";
  import ConfirmTick from "./ConfirmTick.svelte";
  import {
    checklist,
    ITEMS,
    pendingBefore,
    tickKey,
    type ItemState,
    type Note,
  } from "./checklist.svelte";

  interface Props {
    item: ItemState;
    // Every item, for the "usually done after" advice.
    items: ItemState[];
  }

  let { item, items }: Props = $props();

  // The page each item is done on. Per-camera items link each camera's row
  // to that camera's page.
  const PAGE: Record<string, string> = {
    network: "network",
    layout: "layout",
    processors: "camera",
    image: "camera",
    calibration: "geometry",
    color: "color",
    teams: "team_assignment",
    liveField: "live_field",
  };

  // Items whose "attention" comes from a host alert, which says what's wrong.
  const FROM_ALERTS = new Set(["network", "layout", "teams"]);

  let def = $derived(ITEMS[item.id]);
  let words = $derived(text.items[item.id]);
  let page = $derived(PAGE[item.id]);
  let before = $derived(pendingBefore(item.id, items));
  let doneWhen = $derived(
    // This host runs the vision_processors itself (-runCameras).
    item.id === "processors" && processes.list.length > 0
      ? text.items.processors.doneWhenSupervised
      : words.doneWhen,
  );
  let cameraCount = $derived(
    item.cameras
      ? `${String(item.cameras.filter((c) => c.status === "done").length)}/${String(item.cameras.length)}`
      : "",
  );

  function noteText(note: Note): string {
    switch (note.key) {
      case "heard":
        return text.notes.heard(note.fps);
      case "unsaved":
        return text.notes.unsaved(note.count);
      case "warning":
        return note.message;
      default:
        return text.notes[note.key];
    }
  }

  function setOpen(open: boolean): void {
    if (open) checklist.expanded = item.id;
    else if (checklist.expanded === item.id) checklist.expanded = null;
  }
</script>

<AccordionItem
  bind:open={() => checklist.expanded === item.id, setOpen}
  classes={{
    button: "px-1 py-2.5 text-sm",
    content: "px-1 pt-0 pb-4",
  }}
>
  {#snippet header()}
    <span class="flex w-full items-center gap-2 pr-2">
      <StatusIcon status={item.status} />
      <span
        class="grow text-left font-medium text-gray-900 dark:text-white"
        title={text.status[item.status]}>{words.title}</span
      >
      {#if cameraCount}
        <span class="text-xs text-gray-500 dark:text-gray-400"
          >{cameraCount}</span
        >
      {/if}
    </span>
  {/snippet}

  <div class="flex flex-col gap-2 text-sm text-gray-700 dark:text-gray-300">
    {#if item.note}
      <p
        class={item.status === "attention"
          ? "font-medium text-yellow-700 dark:text-yellow-300"
          : "text-gray-500 dark:text-gray-400"}
      >
        {noteText(item.note)}
      </p>
    {/if}
    {#if item.status === "recheck" && !item.cameras}
      <p class="text-yellow-700 dark:text-yellow-300">{text.status.recheck}</p>
    {/if}

    <p>
      <span class="font-semibold">{text.why}:</span>
      {words.why}
    </p>
    <p>
      <span class="font-semibold">{text.doneWhen}:</span>
      {doneWhen}
    </p>

    {#if before.length > 0 && item.status !== "done"}
      <p class="text-xs text-gray-500 italic dark:text-gray-400">
        {text.usuallyAfter(before.map((id) => text.items[id].title))}
      </p>
    {/if}

    {#if item.cameras}
      {#if item.cameras.length === 0}
        <p class="text-gray-500 dark:text-gray-400">{text.noCameras}</p>
      {:else}
        <ul class="flex flex-col gap-1.5">
          {#each item.cameras as row (row.cameraId)}
            <li class="flex flex-wrap items-center gap-x-2 gap-y-1">
              <StatusIcon status={row.status} />
              {#if page}
                <a
                  class="font-medium text-primary-700 hover:underline dark:text-primary-400"
                  href={navHref(row.cameraId, page)}
                  >{text.camera(row.cameraId)}</a
                >
              {:else}
                <span class="font-medium">{text.camera(row.cameraId)}</span>
              {/if}
              {#if row.note}
                <span class="text-xs text-gray-500 dark:text-gray-400"
                  >{noteText(row.note)}</span
                >
              {/if}
              {#if def.manual}
                <span class="ml-auto"
                  ><ConfirmTick key={row.key} status={row.status} /></span
                >
              {/if}
            </li>
          {/each}
        </ul>
      {/if}
    {:else if def.manual}
      <ConfirmTick key={tickKey(item.id)} status={item.status} />
    {/if}

    <div class="flex flex-wrap gap-2 pt-1">
      {#if item.id === "field"}
        <Button size="xs" onclick={openWizard}>{words.action}</Button>
      {:else if item.id === "save"}
        <Button
          size="xs"
          onclick={() => {
            fileDialogs.save = true;
          }}>{words.action}</Button
        >
      {:else if page}
        <Button size="xs" href={categoryHref(page)}
          >{words.action}<ArrowRightOutline class="ms-1" size="xs" /></Button
        >
      {/if}
      {#if item.status === "attention" && FROM_ALERTS.has(item.id)}
        <Button size="xs" color="alternative" href={categoryHref("alerts")}
          >{text.seeAlerts}</Button
        >
      {/if}
    </div>
  </div>
</AccordionItem>
