<script lang="ts">
  import { plural } from "../text/common";
  import { Button, ButtonGroup, Heading, P } from "flowbite-svelte";
  import SettingsCard from "../SettingsCard.svelte";
  import FormRow from "../FormRow.svelte";
  import LayoutMap from "./LayoutMap.svelte";
  import RegionCard from "./RegionCard.svelte";
  import AssignmentsTable from "./AssignmentsTable.svelte";
  import MoveConfirmModal from "./MoveConfirmModal.svelte";
  import { layout as text } from "../text/layout";
  import { layoutView, flips, type FeedMode } from "../layoutView.svelte";
  import { config, cameraStatus } from "../config.svelte";
  import { selectedInstance } from "../layout/nav.svelte";
  import {
    CAMERA_COUNTS,
    applyPlan,
    cameraCount,
    unplacedSources,
    addHeardCamera,
    minCameraCount,
    planCount,
    planMove,
    slots,
    type PendingMove,
  } from "../cameraLayout";

  let doc = $derived(config.doc);
  let count = $derived(doc ? cameraCount(doc) : 1);
  let allSlots = $derived(doc ? slots(doc) : []);

  let picked = $state(selectedInstance()?.cameraId ?? 0);
  let selected = $derived(Math.min(picked, count - 1));
  let slot = $derived(allSlots[selected]);
  let status = $derived(cameraStatus(selected));

  let unplaced = $derived(doc ? unplacedSources(doc) : []);
  let uncovered = $derived(
    allSlots.filter((s) => !s.camera).map((s) => s.cameraId),
  );
  let hostCount = $derived(
    new Set(doc?.cameras.map((c) => c.instance ?? "")).size,
  );

  // A change waiting on the confirm dialog.
  let pending = $state<PendingMove | null>(null);

  function requestCount(next: number): void {
    if (!doc || next === count) return;

    pending = {
      title: text.confirm.splitTitle(next),
      plan: planCount(doc, next),
      count: next,
    };
  }

  function requestMove(from: number, to: number): void {
    if (!doc) return;

    const plan = planMove(doc, from, to);
    if (plan.length === 0) return;

    pending = {
      title:
        plan.length === 2
          ? text.confirm.swapTitle(from, to)
          : text.confirm.moveTitle(from, to),
      plan,
    };
  }

  function mm(v: number): string {
    return String(Math.round(v));
  }

  function applyPending(move: PendingMove): void {
    applyPlan(move.plan, move.count);
  }

  const LABEL_WIDTH = "7rem";

  const FEEDS: { mode: FeedMode; label: string }[] = [
    { mode: "off", label: "Off" },
    { mode: "keyframes", label: "Keyframes" },
    { mode: "live", label: "Live" },
  ];

  let axes = $derived.by(() => {
    const f = flips();

    return text.axes(f.x, f.y);
  });
</script>

{#if doc && slot}
  <div class="max-w-7xl">
    <Heading tag="h2" class="mb-2 text-xl font-semibold">{text.heading}</Heading
    >
    <P size="sm" class="mb-3 text-gray-600 dark:text-gray-400">{text.intro}</P>

    <SettingsCard title="Layout">
      <FormRow
        label="Cameras"
        labelWidth={LABEL_WIDTH}
        notes={text.cameraCount}
        warnings={uncovered.length > 0 ? [text.uncovered(uncovered)] : []}
      >
        <ButtonGroup>
          {#each CAMERA_COUNTS as n (n)}
            {@const tooSmall = n < minCameraCount(doc)}
            <Button
              size="sm"
              color={n === count ? "primary" : "alternative"}
              disabled={tooSmall}
              title={tooSmall ? text.countTooSmall : undefined}
              onclick={() => {
                requestCount(n);
              }}
            >
              {n}
            </Button>
          {/each}
        </ButtonGroup>
      </FormRow>
      <FormRow label="Each region" labelWidth={LABEL_WIDTH}>
        <span class="text-sm">
          {mm(slot.slice.maxX - slot.slice.minX)} × {mm(
            slot.slice.maxY - slot.slice.minY,
          )} mm, {text.eachRegion}
        </span>
      </FormRow>
      <FormRow label="Hardware" labelWidth={LABEL_WIDTH}>
        <span class="text-sm">
          {doc.cameras.length}
          {plural(doc.cameras.length, "camera")} on
          {hostCount}
          {plural(hostCount, "host")}
        </span>
      </FormRow>
    </SettingsCard>

    <div class="grid gap-4 xl:grid-cols-[minmax(0,3fr)_minmax(20rem,2fr)]">
      <SettingsCard title="Field">
        <LayoutMap
          {doc}
          {selected}
          onselect={(id: number) => {
            picked = id;
          }}
        />
        <div class="mt-2 flex flex-wrap items-center gap-2 text-sm">
          <span class="text-gray-600 dark:text-gray-400">Video</span>
          <ButtonGroup>
            {#each FEEDS as { mode, label } (mode)}
              <Button
                size="xs"
                color={layoutView.feeds === mode ? "primary" : "alternative"}
                onclick={() => {
                  layoutView.feeds = mode;
                }}>{label}</Button
              >
            {/each}
          </ButtonGroup>
          <span class="ms-2 text-gray-600 dark:text-gray-400">View</span>
          <Button
            size="xs"
            color={layoutView.rotate180 ? "primary" : "alternative"}
            onclick={() => {
              layoutView.rotate180 = !layoutView.rotate180;
            }}>↻ Rotate 180°</Button
          >
          <Button
            size="xs"
            color={layoutView.mirror ? "primary" : "alternative"}
            onclick={() => {
              layoutView.mirror = !layoutView.mirror;
            }}>⇋ Mirror</Button
          >
        </div>
        <P size="xs" class="mt-1 text-gray-500 dark:text-gray-400">
          {axes}. {text.mapHelp}
        </P>
      </SettingsCard>

      <RegionCard {doc} {slot} {status} onmove={requestMove} />
    </div>

    <AssignmentsTable
      {doc}
      slots={allSlots}
      {selected}
      onselect={(id: number) => {
        picked = id;
      }}
    />

    {#if unplaced.length > 0}
      <SettingsCard title={text.unplaced.title}>
        <P size="sm" class="mb-2 text-gray-600 dark:text-gray-400"
          >{text.unplaced.intro}</P
        >
        {#each unplaced as source (`${String(source.cameraId)}@${source.address}`)}
          <div class="flex items-center gap-3 py-1 text-sm">
            <span class="font-semibold">Camera {source.cameraId}</span>
            <span>{source.address}</span>
            <span class="text-gray-500 dark:text-gray-400">
              {source.receiving ? `${String(source.fps)} fps` : "silent"}
            </span>
            {#if source.cameraId < count}
              <Button
                size="xs"
                color="alternative"
                onclick={() => {
                  addHeardCamera(source);
                }}
              >
                Add as camera {source.cameraId}
              </Button>
            {:else}
              <span class="text-yellow-700">{text.unplaced.outside(count)}</span
              >
            {/if}
          </div>
        {/each}
      </SettingsCard>
    {/if}
  </div>

  <MoveConfirmModal {doc} bind:pending onapply={applyPending} />
{/if}
