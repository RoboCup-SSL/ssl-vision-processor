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

  // The host's wall clock, as its console prints it: HH:MM:SS.mmm.
  function clock(time: string): string {
    return time.slice(11, 23);
  }
</script>

<section
  class="flex flex-col border-t border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800"
  aria-label={text.title}
>
  <div class="flex items-center gap-2 px-3 py-1">
    <Button
      color="alternative"
      size="xs"
      class="h-7 w-7 p-0"
      aria-label={preferences.logExpanded ? text.collapse : text.expand}
      title={preferences.logExpanded ? text.collapse : text.expand}
      onclick={() => {
        preferences.logExpanded = !preferences.logExpanded;
      }}
    >
      {#if preferences.logExpanded}
        <ChevronDownOutline size="sm" />
      {:else}
        <ChevronUpOutline size="sm" />
      {/if}
    </Button>
    <h2 class="text-sm font-semibold whitespace-nowrap">{text.title}</h2>

    {#if preferences.logExpanded}
      <!-- An Input/Select keeps its own w-full, so the wrapper sizes it. -->
      <div class="w-48">
        <Select
          size="sm"
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
        class="gap-1"
        onclick={() => {
          setPaused(!hostLog.paused);
        }}
      >
        {#if hostLog.paused}
          <PlayOutline size="sm" />{text.resume}
        {:else}
          <PauseOutline size="sm" />{text.pause}
        {/if}
      </Button>
      <Button color="alternative" size="xs" onclick={clearLog}
        >{text.clear}</Button
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
      class="ml-auto h-7 w-7 shrink-0 p-0"
      aria-label={text.close}
      title={text.close}
      onclick={() => {
        preferences.logPanel = false;
      }}
    >
      <CloseOutline size="sm" />
    </Button>
  </div>

  {#if preferences.logExpanded}
    <div
      bind:this={body}
      {onscroll}
      class="h-64 overflow-y-auto border-t border-gray-200 px-3 py-1 font-mono text-xs dark:border-gray-700"
    >
      {#each records as r (r.seq)}
        <div class="leading-5 break-all whitespace-pre-wrap" title={r.source}>
          <span class="text-gray-500 dark:text-gray-400">{clock(r.time)}</span>
          <span class={`inline-block w-12 font-semibold ${LEVEL_CLASS[r.level]}`}
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
        <p class="py-2 text-gray-500 dark:text-gray-400">{text.empty}</p>
      {/each}
    </div>
  {/if}
</section>
