<script lang="ts">
  // The host's log, docked at the bottom of the page: the terminal's last
  // records, at a level picked here rather than the host's -logLevel.
  // Shell.svelte places it and leaves room for it.
  import { Button, Select } from "flowbite-svelte";
  import {
    ChevronDownOutline,
    ChevronUpOutline,
    CloseOutline,
    PauseOutline,
    PlayOutline,
  } from "flowbite-svelte-icons";
  import type { LogLevel } from "../apiTypes";
  import { preferences } from "../preferences.svelte";
  import { logs as text } from "../text/logs";
  import {
    clearLog,
    followLog,
    hostLog,
    logRecords,
    setPaused,
  } from "./hostLog.svelte";

  const LEVELS: LogLevel[] = ["debug", "info", "warn", "error"];

  const LEVEL_CLASS: Record<LogLevel, string> = {
    debug: "text-gray-500 dark:text-gray-400",
    info: "text-gray-900 dark:text-gray-100",
    warn: "text-yellow-700 dark:text-yellow-400",
    error: "text-red-700 dark:text-red-400",
  };

  // Followed only while it's expanded; folded, it's just its title bar.
  $effect(() => {
    if (!preferences.logExpanded) return;

    return followLog(preferences.logLevel);
  });

  let records = $derived(logRecords());

  // Stays scrolled to the newest record unless scrolled up to read.
  let body = $state<HTMLElement>();
  let atBottom = true;

  function onscroll(): void {
    if (!body) return;

    atBottom = body.scrollHeight - body.scrollTop - body.clientHeight < 8;
  }

  $effect(() => {
    void records;
    if (body && atBottom) body.scrollTop = body.scrollHeight;
  });

  // The records' height: dragged by the handle on the top edge, kept in the
  // preferences once dropped. At least a few lines, and never so tall the
  // page above disappears.
  const MIN_HEIGHT = 64;
  const PAGE_LEFT = 160;
  const KEY_STEP = 32;

  let innerHeight = $state(window.innerHeight);
  let dragHeight = $state<number | null>(null);
  let drag: { startY: number; startHeight: number } | null = null;

  function clampHeight(h: number): number {
    return Math.round(
      Math.max(MIN_HEIGHT, Math.min(h, innerHeight - PAGE_LEFT)),
    );
  }

  let height = $derived(clampHeight(dragHeight ?? preferences.logHeight));

  function onpointerdown(e: PointerEvent): void {
    // No text selection while dragging.
    e.preventDefault();
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    drag = { startY: e.clientY, startHeight: height };
  }

  function onpointermove(e: PointerEvent): void {
    if (!drag) return;

    dragHeight = clampHeight(drag.startHeight + drag.startY - e.clientY);
  }

  // Saved once, on drop, not on every move.
  function endDrag(): void {
    if (dragHeight !== null) preferences.logHeight = dragHeight;

    drag = null;
    dragHeight = null;
  }

  function onkeydown(e: KeyboardEvent): void {
    const step = { ArrowUp: KEY_STEP, ArrowDown: -KEY_STEP }[e.key];
    if (step === undefined) return;

    e.preventDefault();
    preferences.logHeight = clampHeight(height + step);
  }

  // The host's wall clock, as its console prints it: HH:MM:SS.mmm.
  function clock(time: string): string {
    return time.slice(11, 23);
  }
</script>

<svelte:window bind:innerHeight />

<section
  class="relative flex flex-col border-t border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800"
  aria-label={text.title}
>
  {#if preferences.logExpanded}
    <!-- Straddles the top border, so it's easy to grab without taking a row.
         A raw button, unstyled on purpose: focusable and keyboard-usable
         without any a11y exceptions. -->
    <button
      type="button"
      aria-label={text.resize}
      title={text.resize}
      class="absolute inset-x-0 -top-1 z-10 h-2 cursor-row-resize hover:bg-primary-500/40 focus-visible:bg-primary-500/40 focus-visible:outline-none"
      class:bg-primary-500={dragHeight !== null}
      {onpointerdown}
      {onpointermove}
      onpointerup={endDrag}
      onlostpointercapture={endDrag}
      {onkeydown}
    ></button>
  {/if}

  <div class="flex items-center gap-1.5 px-2 py-0.5">
    <Button
      color="alternative"
      size="xs"
      class="h-6 w-6 shrink-0 p-0"
      aria-label={preferences.logExpanded ? text.collapse : text.expand}
      title={preferences.logExpanded ? text.collapse : text.expand}
      onclick={() => {
        preferences.logExpanded = !preferences.logExpanded;
      }}
    >
      {#if preferences.logExpanded}
        <ChevronDownOutline size="xs" />
      {:else}
        <ChevronUpOutline size="xs" />
      {/if}
    </Button>
    <h2 class="text-xs font-semibold whitespace-nowrap">{text.title}</h2>

    {#if preferences.logExpanded}
      <!-- An Input/Select keeps its own w-full, so the wrapper sizes it. -->
      <div class="w-40 shrink-0">
        <Select
          size="sm"
          classes={{ select: "py-0.5 pl-2 pr-7" }}
          aria-label={text.level}
          title={text.level}
          placeholder=""
          bind:value={preferences.logLevel}
        >
          {#each LEVELS as level (level)}
            <option value={level}>{text.levels[level]}</option>
          {/each}
        </Select>
      </div>
      <Button
        color="alternative"
        size="xs"
        class="h-6 shrink-0 gap-1 px-2 py-0"
        onclick={() => {
          setPaused(!hostLog.paused);
        }}
      >
        {#if hostLog.paused}
          <PlayOutline size="xs" />{text.resume}
        {:else}
          <PauseOutline size="xs" />{text.pause}
        {/if}
      </Button>
      <Button
        color="alternative"
        size="xs"
        class="h-6 shrink-0 px-2 py-0"
        onclick={clearLog}>{text.clear}</Button
      >
      <span class="min-w-0 truncate text-xs text-gray-500 dark:text-gray-400">
        {#if hostLog.error}
          <span class="text-red-700 dark:text-red-400"
            >{text.unreachable(hostLog.error)}</span
          >
        {:else if hostLog.paused}
          {text.paused(hostLog.waiting)}
        {:else if hostLog.missed}
          {text.missed}
        {/if}
      </span>
    {/if}

    <Button
      color="alternative"
      size="xs"
      class="ml-auto h-6 w-6 shrink-0 p-0"
      aria-label={text.close}
      title={text.close}
      onclick={() => {
        preferences.logPanel = false;
      }}
    >
      <CloseOutline size="xs" />
    </Button>
  </div>

  {#if preferences.logExpanded}
    <div
      bind:this={body}
      {onscroll}
      style:height={`${String(height)}px`}
      class="overflow-y-auto border-t border-gray-200 px-2 py-0.5 font-mono text-xs dark:border-gray-700"
    >
      {#each records as r (r.seq)}
        <div class="leading-4 break-all whitespace-pre-wrap" title={r.source}>
          <span class="text-gray-500 dark:text-gray-400">{clock(r.time)}</span>
          <span
            class={`inline-block w-12 font-semibold ${LEVEL_CLASS[r.level]}`}
            >{r.level.toUpperCase()}</span
          >
          <span class={LEVEL_CLASS[r.level]}>{r.message}</span>
          {#each r.attrs as a, i (i)}
            <span class="ml-2 text-gray-500 dark:text-gray-400"
              >{a.key}=<span class="text-gray-700 dark:text-gray-300"
                >{a.value}</span
              ></span
            >
          {/each}
        </div>
      {:else}
        <p class="py-1 text-gray-500 dark:text-gray-400">{text.empty}</p>
      {/each}
    </div>
  {/if}
</section>
