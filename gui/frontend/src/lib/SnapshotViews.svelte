<script lang="ts">
  import { A, Button, P } from "flowbite-svelte";
  import SettingsCard from "./SettingsCard.svelte";
  import { api, apiPath, unwrap } from "./api";
  import { snapshots as text, type SnapshotView } from "./text/snapshots";

  // One camera's debug images from img/, listed when shown and on Refresh,
  // not polled: they can be large and every open browser would fetch them.
  interface Props {
    cameraId: number;
    title: string;
    intro: string;
    empty: string;
    views: SnapshotView[];
  }

  let { cameraId, title, intro, empty, views }: Props = $props();

  let written = $state<Set<string>>(new Set());
  let stamp = $state(0);
  let error = $state<string | null>(null);

  let shown = $derived(views.filter((v) => written.has(v.view)));

  async function load(camera: number): Promise<void> {
    try {
      const all = await unwrap(api.GET("/api/snapshots"));
      written = new Set(
        all.filter((s) => s.cameraId === camera).map((s) => s.view),
      );
      stamp = Date.now();
      error = null;
    } catch (err) {
      error = err instanceof Error ? err.message : String(err);
    }
  }

  $effect(() => {
    void load(cameraId);
  });

  const src = (view: string): string =>
    `${apiPath("/api/snapshots/{id}/{view}", { id: cameraId, view })}?t=${String(stamp)}`;
</script>

<SettingsCard {title}>
  <div class="flex items-start justify-between gap-3">
    <P size="sm" class="text-gray-600 dark:text-gray-400">{intro}</P>
    <Button
      size="xs"
      color="alternative"
      onclick={() => {
        void load(cameraId);
      }}>{text.refresh}</Button
    >
  </div>

  {#if error}
    <P size="sm" class="mt-2 text-red-700 dark:text-red-400"
      >{text.failed(error)}</P
    >
  {:else if shown.length === 0}
    <P size="sm" class="mt-2 text-gray-600 dark:text-gray-400">{empty}</P>
  {:else}
    <div class="mt-2 grid grid-cols-2 gap-3">
      {#each shown as v (v.view)}
        <figure>
          <A href={src(v.view)} target="_blank" rel="noopener">
            <img
              src={src(v.view)}
              alt={`cam ${String(cameraId)} ${v.label}`}
              class="w-full rounded border border-gray-200 dark:border-gray-700"
            />
          </A>
          <figcaption class="mt-1 text-xs text-gray-600 dark:text-gray-400">
            {v.label}
          </figcaption>
        </figure>
      {/each}
    </div>
  {/if}
</SettingsCard>
