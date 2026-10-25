<script lang="ts">
  import {
    Checkbox,
    Heading,
    Indicator,
    P,
    Radio,
    Range,
    Select,
    Toggle,
  } from "flowbite-svelte";
  import type { SSL_WrapperPacketJson } from "../../proto/vision/ssl_vision_wrapper_pb";
  import StatusAlert from "../StatusAlert.svelte";
  import SettingsCard from "../SettingsCard.svelte";
  import RichText from "../RichText.svelte";
  import { topic } from "../wrapper-bus";
  import { network } from "../network.svelte";
  import { teamName } from "../teams.svelte";
  import { categoryHref } from "../layout/nav.svelte";
  import type { LiveFieldSnapshot } from "../apiTypes";
  import { liveField as text } from "../text/liveField";
  import FieldCanvas from "./FieldCanvas.svelte";
  import { tint, type DrawnLayer } from "./draw";
  import {
    VISION,
    layersOf,
    processingMs,
    sourcesOf,
    type Source,
  } from "./sources";
  import {
    liveView,
    overlayFor,
    setOverlay,
    toggleCamera,
  } from "./liveView.svelte";

  // Raw, not deeply reactive: a new snapshot replaces the old one up to 60
  // times a second, and nothing edits it.
  let snapshot = $state.raw<LiveFieldSnapshot | null>(null);
  let packet = $state.raw<SSL_WrapperPacketJson | null>(null);

  $effect(() =>
    topic<LiveFieldSnapshot>("live_field.frames").subscribe((s) => {
      if (s) snapshot = s;
    }),
  );

  $effect(() =>
    topic<SSL_WrapperPacketJson>("wrapper_packet.out").subscribe((p) => {
      if (p) packet = p;
    }),
  );

  let field = $derived(packet?.geometry?.field);
  let sources = $derived(sourcesOf(snapshot));

  // Each source's outline color: vision's, then the autorefs' in list order.
  let tints = $derived(
    new Map(sources.map((s, i): [string, string] => [s.key, tint(i)])),
  );

  function label(s: Source): string {
    return s.kind === "vision" ? text.sources.vision : s.name;
  }

  // The primary may be an autoref that isn't running right now: it stays
  // chosen, and is listed as not heard.
  let options = $derived.by(() => {
    const listed = sources.map((s) => ({ value: s.key, name: label(s) }));

    if (!sources.some((s) => s.key === liveView.primary)) {
      listed.push({
        value: liveView.primary,
        name: text.sources.notHeard(liveView.primary.replace(/^tracker:/, "")),
      });
    }

    return listed;
  });

  let overlays = $derived(sources.filter((s) => s.key !== liveView.primary));

  function layersFor(key: string, primary: boolean): DrawnLayer[] {
    const opacity = primary ? 1 : overlayFor(key).opacity;

    return layersOf(snapshot, key, liveView.hiddenCameras).map((l) => ({
      layer: l.layer,
      style: {
        robotStyle: liveView.robotStyle,
        alpha: opacity,
        outline:
          l.camera !== undefined && liveView.cameraTint
            ? tint(l.camera + 1)
            : primary
              ? null
              : (tints.get(key) ?? null),
        velocities: liveView.velocities,
        confidence: primary && key === VISION,
      },
    }));
  }

  // Overlays first, so the primary is drawn on top.
  let layers = $derived([
    ...overlays
      .filter((s) => overlayFor(s.key).on)
      .flatMap((s) => layersFor(s.key, false)),
    ...layersFor(liveView.primary, true),
  ]);

  let tracker = $derived(network.state?.tracker);

  let teams = $derived(
    teamName("yellow") || teamName("blue")
      ? text.teams(teamName("yellow"), teamName("blue"))
      : "",
  );

  function status(
    s: { receiving: boolean; fps: number },
    ageMs: number | null,
  ): string {
    return s.receiving
      ? text.status.receiving(s.fps, ageMs)
      : text.status.silent;
  }

  function cameraAge(receivedAt: string): number | null {
    if (!snapshot) return null;

    return new Date(snapshot.at).getTime() - new Date(receivedAt).getTime();
  }
</script>

<div class="live-field">
  <Heading tag="h2" class="mb-2 text-xl font-semibold">{text.heading}</Heading>
  <P size="sm" class="mb-3 text-gray-600 dark:text-gray-400">{text.intro}</P>

  <div class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_20rem]">
    <div>
      {#if teams}
        <P size="sm" class="mb-1 text-gray-600 dark:text-gray-400">{teams}</P>
      {/if}
      {#if field}
        <FieldCanvas {field} {layers} flipped={liveView.flipped} />
      {:else}
        <StatusAlert color="gray" class="p-2 text-sm"
          >{text.waitingForField}</StatusAlert
        >
      {/if}
    </div>

    <div>
      <SettingsCard title={text.view.title}>
        <label class="mb-1 text-sm font-medium" for="live-primary"
          >{text.view.primary}</label
        >
        <Select
          id="live-primary"
          size="sm"
          class="mb-3"
          items={options}
          bind:value={liveView.primary}
        />

        <span class="mb-1 text-sm font-medium">{text.view.robots}</span>
        <div class="mb-3 flex gap-4">
          <Radio
            name="robotStyle"
            value="numbered"
            bind:group={liveView.robotStyle}>{text.view.numbered}</Radio
          >
          <Radio
            name="robotStyle"
            value="pattern"
            bind:group={liveView.robotStyle}>{text.view.pattern}</Radio
          >
        </div>

        <div class="flex flex-col gap-2">
          <Toggle size="small" bind:checked={liveView.velocities}
            >{text.view.velocities}</Toggle
          >
          <Toggle size="small" bind:checked={liveView.flipped}
            >{text.view.flipped}</Toggle
          >
          <Toggle size="small" bind:checked={liveView.cameraTint}
            >{text.view.cameraTint}</Toggle
          >
        </div>
      </SettingsCard>

      <SettingsCard title={text.sources.title}>
        <P size="xs" class="mb-2 text-gray-600 dark:text-gray-400"
          >{text.sources.overlayHint}</P
        >

        {#each sources as s (s.key)}
          {@const primary = s.key === liveView.primary}
          {@const overlay = overlayFor(s.key)}
          <div class="mb-2">
            <div class="flex items-center gap-2 text-sm">
              <span
                class="swatch"
                style:background={tints.get(s.key)}
                aria-hidden="true"
              ></span>
              {#if primary}
                <span class="font-semibold">{label(s)}</span>
              {:else}
                <Checkbox
                  checked={overlay.on}
                  onchange={(e: Event) => {
                    setOverlay(s.key, {
                      on: (e.currentTarget as HTMLInputElement).checked,
                    });
                  }}>{label(s)}</Checkbox
                >
              {/if}
              <span
                class="ms-auto flex items-center gap-1 text-xs text-gray-500"
              >
                <Indicator size="xs" color={s.receiving ? "green" : "gray"} />
                {status(s, s.ageMs)}
              </span>
            </div>
            {#if !primary && overlay.on}
              <div class="mt-1 flex items-center gap-2 ps-6 text-xs">
                <span>{text.sources.opacity}</span>
                <Range
                  size="sm"
                  min={0.1}
                  max={1}
                  step={0.05}
                  value={overlay.opacity}
                  oninput={(e: Event) => {
                    setOverlay(s.key, {
                      opacity: (e.currentTarget as HTMLInputElement)
                        .valueAsNumber,
                    });
                  }}
                />
              </div>
            {/if}
          </div>
        {/each}

        {#if tracker?.problem}
          <StatusAlert color="yellow" class="mt-2 p-2 text-xs"
            >{text.sources.trackerProblem(tracker.problem)}</StatusAlert
          >
        {:else if sources.length === 1 && tracker}
          <P size="xs" class="mt-2 text-gray-600 dark:text-gray-400"
            ><a class="underline" href={categoryHref("network")}
              ><RichText text={text.sources.noTrackers(tracker.address)} /></a
            ></P
          >
        {/if}
      </SettingsCard>

      <SettingsCard title={text.cameras.title}>
        {#if (snapshot?.cameras.length ?? 0) === 0}
          <P size="xs" class="text-gray-600 dark:text-gray-400"
            >{text.cameras.none}</P
          >
        {:else}
          <table class="w-full text-xs">
            <thead class="text-left text-gray-500">
              <tr>
                <th>{text.cameras.shown}</th>
                <th>{text.cameras.camera}</th>
                <th>{text.cameras.rate}</th>
                <th title={text.cameras.processingHint}
                  >{text.cameras.processing}</th
                >
              </tr>
            </thead>
            <tbody>
              {#each snapshot?.cameras ?? [] as c (`${String(c.cameraId)}@${c.address}`)}
                {@const ms = processingMs(c)}
                <tr title={`${text.cameras.address} ${c.address}`}>
                  <td>
                    <Checkbox
                      checked={!liveView.hiddenCameras.includes(c.cameraId)}
                      onchange={(e: Event) => {
                        toggleCamera(
                          c.cameraId,
                          (e.currentTarget as HTMLInputElement).checked,
                        );
                      }}
                    />
                  </td>
                  <td>
                    <span class="flex items-center gap-1">
                      {#if liveView.cameraTint}
                        <span
                          class="swatch"
                          style:background={tint(c.cameraId + 1)}
                          aria-hidden="true"
                        ></span>
                      {/if}
                      {c.cameraId}
                    </span>
                  </td>
                  <td>{status(c, cameraAge(c.receivedAt))}</td>
                  <td>{ms === null ? "" : text.cameras.ms(ms)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </SettingsCard>
    </div>
  </div>
</div>

<style>
  .swatch {
    display: inline-block;
    width: 0.75rem;
    height: 0.75rem;
    flex-shrink: 0;
    border: 1px solid var(--text-muted);
    border-radius: 9999px;
  }
</style>
