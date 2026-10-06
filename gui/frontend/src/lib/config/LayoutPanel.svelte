<script lang="ts">
  import {
    Alert,
    Button,
    ButtonGroup,
    Heading,
    Modal,
    P,
    Select,
    Table,
    TableBody,
    TableBodyCell,
    TableBodyRow,
    TableHead,
    TableHeadCell,
  } from "flowbite-svelte";
  import SettingsCard from "../SettingsCard.svelte";
  import FormRow from "../FormRow.svelte";
  import LayoutMap from "./LayoutMap.svelte";
  import { layout as text } from "../text/layout";
  import { layoutView, flips, type FeedMode } from "../layoutView.svelte";
  import { config, cameraStatus } from "../config.svelte";
  import { navHref, selectedInstance } from "../layout/nav.svelte";
  import {
    CAMERA_COUNTS,
    applyPlan,
    cameraCount,
    cameraDevice,
    deviceLabel,
    hostColor,
    hostName,
    liveLabel,
    unplacedSources,
    addHeardCamera,
    setDisplay,
    minCameraCount,
    planCount,
    planMove,
    slots,
    type Consequence,
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
  let pending = $state<{
    title: string;
    plan: Consequence[];
    count?: number;
  } | null>(null);

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

  function confirm(): void {
    if (pending) applyPlan(pending.plan, pending.count);
    pending = null;
  }

  function cameraName(cameraId: number): string {
    const camera = doc?.cameras.find((c) => c.cameraId === cameraId);
    if (!doc || !camera) return "";

    return `${hostName(camera)} · ${deviceLabel(cameraDevice(doc, camera))}`;
  }

  function mm(v: number): string {
    return String(Math.round(v));
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
          {doc.cameras.length} camera{doc.cameras.length === 1 ? "" : "s"} on
          {hostCount} host{hostCount === 1 ? "" : "s"}
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

      <SettingsCard title={`Region ${String(selected)} (${slot.region})`}>
        <FormRow label="Bounds" labelWidth="5rem">
          <span class="text-sm">
            x {mm(slot.slice.minX)} … {mm(slot.slice.maxX)}, y {mm(
              slot.slice.minY,
            )} … {mm(slot.slice.maxY)} mm
          </span>
        </FormRow>

        <FormRow
          label="Live"
          labelWidth="5rem"
          errors={slot.conflict ? [text.conflict] : []}
        >
          <span class="text-sm">{liveLabel(slot.sources)}</span>
        </FormRow>

        <FormRow
          label="Camera"
          for="layout-camera"
          labelWidth="5rem"
          notes={text.cameraPicker}
        >
          <Select
            id="layout-camera"
            size="sm"
            placeholder=""
            value={slot.camera ? selected : -1}
            onchange={(e: Event) => {
              const from = Number((e.currentTarget as HTMLSelectElement).value);
              requestMove(from, selected);
              // The select shows the change only once it's confirmed.
              (e.currentTarget as HTMLSelectElement).value = String(
                slot.camera ? selected : -1,
              );
            }}
          >
            {#if !slot.camera}
              <option value={-1} disabled>No camera</option>
            {/if}
            {#each doc.cameras as c (c.cameraId)}
              <option value={c.cameraId}>
                {cameraName(c.cameraId)}{c.cameraId === selected
                  ? ""
                  : ` (now ${String(c.cameraId)})`}
              </option>
            {/each}
          </Select>
        </FormRow>

        {#if slot.camera}
          {@const camera = slot.camera}
          <FormRow
            label="Video"
            labelWidth="5rem"
            notes={text.videoOrientation}
          >
            <span class="flex items-center gap-2">
              <Button
                size="xs"
                color="alternative"
                onclick={() => {
                  setDisplay(selected, {
                    rotate: (camera.display?.rotate ?? 0) + 90,
                  });
                }}>↻ Rotate</Button
              >
              <Button
                size="xs"
                color={camera.display?.mirror ? "primary" : "alternative"}
                onclick={() => {
                  setDisplay(selected, { mirror: !camera.display?.mirror });
                }}>⇋ Mirror</Button
              >
              <span class="text-sm text-gray-600 dark:text-gray-400">
                {camera.display?.rotate ?? 0}°{camera.display?.mirror
                  ? ", mirrored"
                  : ""}
              </span>
            </span>
          </FormRow>
          <FormRow label="Device" labelWidth="5rem">
            <span class="text-sm break-all">
              {cameraDevice(doc, camera).path ??
                deviceLabel(cameraDevice(doc, camera))}
            </span>
          </FormRow>
          <FormRow
            label="Config"
            labelWidth="5rem"
            hints={camera.configPath ? [] : [text.remote]}
          >
            <span class="text-sm break-all">
              {camera.configPath ?? "remote"}
            </span>
          </FormRow>
          <FormRow
            label="Calibration"
            labelWidth="5rem"
            warnings={(status?.warnings ?? []).map((w) => w.message)}
          >
            <span class="text-sm">
              {status?.calibration ?? "none"}{camera.seed?.lineCorners
                .length === 4
                ? ", corners picked"
                : ", no corners"}
            </span>
          </FormRow>

          <div class="mt-2 flex flex-wrap gap-2">
            <Button
              size="xs"
              color="alternative"
              href={navHref(selected, "camera")}
            >
              Camera Settings →
            </Button>
            <Button
              size="xs"
              color="alternative"
              href={navHref(selected, "geometry")}
            >
              Geometry →
            </Button>
          </div>
        {/if}
      </SettingsCard>
    </div>

    <SettingsCard title="Assignments">
      <Table hoverable>
        <TableHead>
          <TableHeadCell class="px-2 py-1.5">ID</TableHeadCell>
          <TableHeadCell class="px-2 py-1.5">Region</TableHeadCell>
          <TableHeadCell class="px-2 py-1.5">Host</TableHeadCell>
          <TableHeadCell class="px-2 py-1.5">Device</TableHeadCell>
          <TableHeadCell class="px-2 py-1.5">Live</TableHeadCell>
          <TableHeadCell class="px-2 py-1.5">Config</TableHeadCell>
          <TableHeadCell class="px-2 py-1.5">Calibration</TableHeadCell>
          <TableHeadCell class="px-2 py-1.5">Warnings</TableHeadCell>
        </TableHead>
        <TableBody>
          {#each allSlots as s (s.cameraId)}
            {@const camera = s.camera}
            {@const rowStatus = cameraStatus(s.cameraId)}
            <TableBodyRow
              class={`cursor-pointer ${s.severity === "warning" ? "bg-yellow-50 dark:bg-yellow-900/30" : ""} ${s.severity === "error" ? "bg-red-50 dark:bg-red-900/30" : ""} ${s.cameraId === selected ? "font-semibold" : ""}`}
              onclick={() => {
                picked = s.cameraId;
              }}
            >
              <TableBodyCell class="px-2 py-1.5">{s.cameraId}</TableBodyCell>
              <TableBodyCell class="px-2 py-1.5">{s.region}</TableBodyCell>
              {#if camera}
                <TableBodyCell class="px-2 py-1.5">
                  <span class="flex items-center gap-1.5">
                    <span
                      class="inline-block h-2 w-2 rounded-full"
                      style:background={hostColor(doc, camera.instance ?? "")}
                    ></span>
                    {hostName(camera)}
                  </span>
                </TableBodyCell>
                <TableBodyCell
                  class="px-2 py-1.5"
                  title={cameraDevice(doc, camera).path}
                >
                  {deviceLabel(cameraDevice(doc, camera))}
                </TableBodyCell>
                <TableBodyCell
                  class={`px-2 py-1.5 ${s.conflict ? "text-red-700" : ""}`}
                >
                  {liveLabel(s.sources)}
                </TableBodyCell>
                <TableBodyCell class="px-2 py-1.5">
                  {camera.configPath ? "local" : "remote"}
                </TableBodyCell>
                <TableBodyCell class="px-2 py-1.5">
                  {rowStatus?.calibration ?? "none"}
                </TableBodyCell>
                <TableBodyCell
                  class="px-2 py-1.5"
                  title={(rowStatus?.warnings ?? [])
                    .map((w) => w.message)
                    .join("\n")}
                >
                  {#if (rowStatus?.warnings.length ?? 0) > 0}
                    <span class="text-yellow-700">
                      ⚠ {rowStatus?.warnings.map((w) => w.code).join(", ")}
                    </span>
                  {/if}
                </TableBodyCell>
              {:else}
                <TableBodyCell
                  class="px-2 py-1.5 text-gray-500 dark:text-gray-400 italic"
                >
                  No camera
                </TableBodyCell>
                <TableBodyCell></TableBodyCell>
                <TableBodyCell class="px-2 py-1.5"
                  >{liveLabel(s.sources)}</TableBodyCell
                >
                <TableBodyCell class="px-2 py-1.5"></TableBodyCell>
                <TableBodyCell class="px-2 py-1.5"></TableBodyCell>
                <TableBodyCell class="px-2 py-1.5"></TableBodyCell>
              {/if}
            </TableBodyRow>
          {/each}
        </TableBody>
      </Table>
    </SettingsCard>

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
      <ul class="space-y-2 text-sm">
        {#each pending.plan as step (step.from)}
          <li>
            <span class="font-semibold">
              {`${hostName(step.camera)} · ${deviceLabel(cameraDevice(doc, step.camera))}:`}
            </span>
            {step.from !== step.to
              ? text.confirm.moved(step.from, step.to)
              : text.confirm.newRegion(step.from)}
            <ul class="ms-4 list-disc text-gray-600 dark:text-gray-400">
              <li>{text.confirm.settingsStay}</li>
              {#if step.seedStale}
                <li>{text.confirm.seedStale}</li>
              {/if}
              {#if step.calibrationRemoved}
                <li>{text.confirm.calibrationRemoved}</li>
              {/if}
              {#if !step.camera.configPath}
                <li>{text.confirm.remote}</li>
              {/if}
            </ul>
          </li>
        {/each}
      </ul>

      <Alert color="yellow" class="mt-3 p-2 text-sm">
        {text.confirm.restart}
      </Alert>
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
      <Button onclick={confirm}>Apply</Button>
    </div>
  </Modal>
{/if}
