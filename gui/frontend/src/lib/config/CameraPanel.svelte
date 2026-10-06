<script lang="ts">
  import StatusAlert from "../StatusAlert.svelte";
  import RichText from "../RichText.svelte";
  import SettingsCard from "../SettingsCard.svelte";
  import { Input, Select, Badge, Heading, P } from "flowbite-svelte";
  import { cameraCount, regionLabel } from "../cameraLayout";
  import { categoryHref } from "../layout/nav.svelte";
  import { onMount } from "svelte";
  import type { VisionInstance } from "../layout/nav.svelte";
  import { config, cameraDoc } from "../config.svelte";
  import {
    DRIVERS,
    cameraSettings,
    setCameraKey,
    loadVideoDevices,
    type Driver,
    type VideoDevice,
    type WhiteBalance,
  } from "../cameraSettings.svelte";
  import { CAMERA_RANGES, toDisplay, toFile } from "../cameraRanges";
  import VideoPlayer from "../video/VideoPlayer.svelte";
  import FormRow from "../FormRow.svelte";
  import SliderField from "./SliderField.svelte";
  import { camera as text } from "../text/camera";
  import { NO_HOST, PLACEHOLDER_RANGE } from "../text/common";

  interface Props {
    instance: VisionInstance | undefined;
  }

  let { instance }: Props = $props();

  let cameraId = $derived(instance?.cameraId ?? 0);
  let settings = $derived(cameraSettings(cameraId));
  let driver = $derived(DRIVERS[settings.driver]);
  // Placeholder slider ranges; see cameraRanges.ts.
  let ranges = $derived(CAMERA_RANGES[settings.driver]);
  let configPath = $derived(cameraDoc(cameraId)?.configPath);

  let devices = $state<VideoDevice[]>([]);

  onMount(() => {
    void loadVideoDevices().then((d) => {
      devices = d;
    });
  });

  function set<K extends keyof typeof settings>(
    key: K,
    value: (typeof settings)[K] | undefined,
  ): void {
    setCameraKey(cameraId, key, value);
  }

  // Device

  let effectivePath = $derived(
    settings.path ?? `/dev/video${String(settings.id)}`,
  );

  const KIND_LABEL: Record<VideoDevice["kind"], string> = text.path.kind;

  let pathNotes = $derived.by((): string[] => {
    const notes = [...text.path.notes];
    const device = devices.find((d) => d.path === effectivePath);

    if (device?.kind === "by-id") notes.push(text.path.byId);
    if (device?.kind === "by-path") notes.push(text.path.byPath);
    if (!device && devices.length > 0 && effectivePath.startsWith("/dev/"))
      notes.push(text.path.notOnHost);

    return notes;
  });

  let pathWarnings = $derived(
    /^\/dev\/video\d+$/.test(effectivePath) ? [text.path.unstable] : [],
  );

  // Resolution

  const RESOLUTIONS: [number, number][] = [
    [0, 0],
    [3840, 2160],
    [2560, 1440],
    [1920, 1080],
    [1280, 720],
    [640, 480],
  ];

  let resolutionIndex = $derived(
    RESOLUTIONS.findIndex(
      ([w, h]) => w === settings.width && h === settings.height,
    ),
  );

  let draftSize = $state<[number, number]>([0, 0]);
  let sizeError = $state<string | null>(null);

  $effect(() => {
    draftSize = [settings.width, settings.height];
    sizeError = null;
  });

  function setSize(width: number, height: number): void {
    if (
      !Number.isInteger(width) ||
      !Number.isInteger(height) ||
      width < 0 ||
      height < 0
    ) {
      sizeError = text.resolution.notWhole;

      return;
    }

    if ((width === 0) !== (height === 0)) {
      sizeError = text.resolution.setBoth;

      return;
    }

    sizeError = null;
    set("width", width);
    set("height", height);
  }

  let resolutionNotes = $derived(
    settings.driver === "OPENCV" ? text.resolution.opencv : text.resolution.sdk,
  );

  // Exposure, gain, gamma

  let exposureNotes = $derived([
    ...text.exposure.notes,
    ...(settings.driver === "OPENCV"
      ? text.exposure.opencv
      : text.exposure.sdk),
  ]);

  let exposureWarnings = $derived(
    settings.driver === "OPENCV" && settings.exposure === 0
      ? [text.exposure.opencvAutoBroken]
      : [],
  );

  let gainNotes = $derived([
    ...text.gain.notes,
    ...(settings.driver === "OPENCV" ? text.gain.opencv : text.gain.sdk),
  ]);

  let gammaNotes = $derived([
    ...text.gamma.notes,
    ...(settings.driver === "OPENCV" ? text.gamma.opencv : text.gamma.sdk),
  ]);

  let gammaWarnings = $derived(
    settings.driver === "OPENCV" && settings.gamma !== 1
      ? [text.gamma.opencvIgnored]
      : [],
  );

  // White balance

  type WBMode = "OUTDOOR" | "INDOOR" | "auto" | "manual";

  let wbMode = $derived.by((): WBMode => {
    const wb = settings.white_balance;
    if (typeof wb === "object") return "manual";

    return driver.wbProfiles ? wb : "auto";
  });

  let manualWB = $derived(
    typeof settings.white_balance === "object"
      ? settings.white_balance
      : {
          red: toFile(ranges.whiteBalance, ranges.whiteBalance.fallback),
          blue: toFile(ranges.whiteBalance, ranges.whiteBalance.fallback),
        },
  );

  function setWBMode(mode: WBMode): void {
    const value: WhiteBalance =
      mode === "manual" ? { ...manualWB } : mode === "auto" ? "OUTDOOR" : mode;

    set("white_balance", value);
  }

  let wbNotes = $derived.by((): string[] => {
    if (driver.wbProfiles) return text.whiteBalance.profiles;
    if (settings.driver === "MVIMPACT") return text.whiteBalance.mvimpact;

    return text.whiteBalance.auto;
  });

  let wbChannelNotes = $derived(
    settings.driver === "OPENCV"
      ? text.whiteBalance.channelOpencv
      : [
          settings.driver === "SPINNAKER"
            ? text.whiteBalance.channelSpinnaker
            : text.whiteBalance.channelMvimpact,
          PLACEHOLDER_RANGE,
        ],
  );
</script>

<section class="camera-panel">
  <div class="mb-2 flex flex-wrap items-center gap-3">
    <Heading tag="h2" class="w-auto text-xl font-semibold"
      >{text.heading}</Heading
    >
    {#if instance && config.doc}
      <Badge color="gray" href={categoryHref("layout")} title={text.slotChip}>
        {instance.host || NO_HOST} · camera {instance.cameraId} of {cameraCount(
          config.doc,
        )} · {regionLabel(config.doc, instance.cameraId)}
      </Badge>
    {/if}
  </div>

  {#if instance}
    <StatusAlert color="primary" class="mb-3 p-2 text-sm">
      <RichText text={text.restartBanner(instance.cameraId)} />
      <RichText
        text={configPath ? text.writesTo(configPath) : text.noConfigPath}
      />
    </StatusAlert>

    <div class="layout">
      <!-- Left: the picture, with the settings that only change what is
           captured. Right: the ones you tune by eye, next to the picture,
           so they can give live feedback once vision_processor applies
           camera settings without a restart. -->
      <div class="main">
        <div class="video">
          {#key instance.cameraId}
            <VideoPlayer cameraId={instance.cameraId} />
          {/key}
        </div>

        <div class="capture">
          <SettingsCard title="Device" class="mb-0">
            <FormRow label="Driver" for="cam-driver" notes={text.driver}>
              <Select
                id="cam-driver"
                size="sm"
                placeholder=""
                value={settings.driver}
                onchange={(e: Event) => {
                  set(
                    "driver",
                    (e.currentTarget as HTMLSelectElement).value as Driver,
                  );
                }}
              >
                {#each Object.entries(DRIVERS) as [key, info] (key)}
                  <option value={key}>{info.label}</option>
                {/each}
              </Select>
            </FormRow>

            {#if driver.selectsBy === "id"}
              <FormRow
                label="Camera index"
                for="cam-id"
                notes={text.cameraIndex}
              >
                <Input
                  id="cam-id"
                  type="number"
                  size="sm"
                  min="0"
                  step="1"
                  value={settings.id}
                  onchange={(e: Event) => {
                    const v = (e.currentTarget as HTMLInputElement)
                      .valueAsNumber;
                    if (Number.isInteger(v) && v >= 0) set("id", v);
                  }}
                />
              </FormRow>
            {:else}
              <FormRow
                label="Path"
                for="cam-path"
                notes={pathNotes}
                warnings={pathWarnings}
              >
                <Select
                  aria-label="Cameras on this host"
                  size="sm"
                  placeholder=""
                  value={devices.findIndex((d) => d.path === effectivePath)}
                  onchange={(e: Event) => {
                    const device =
                      devices[
                        Number((e.currentTarget as HTMLSelectElement).value)
                      ];
                    if (device) set("path", device.path);
                  }}
                >
                  {#each devices as device, i (device.path)}
                    <option value={i}>
                      {device.name} ({KIND_LABEL[device.kind]}) {device.path}
                    </option>
                  {/each}
                  <option value={-1} disabled>
                    {devices.length === 0 ? "No cameras on this host" : "Other"}
                  </option>
                </Select>
                <Input
                  id="cam-path"
                  type="text"
                  size="sm"
                  class="font-mono"
                  spellcheck="false"
                  value={settings.path ?? ""}
                  placeholder={`/dev/video${String(settings.id)}`}
                  onchange={(e: Event) => {
                    const v = (
                      e.currentTarget as HTMLInputElement
                    ).value.trim();
                    set("path", v === "" ? undefined : v);
                  }}
                />
              </FormRow>
            {/if}
          </SettingsCard>

          <SettingsCard title="Resolution" class="mb-0">
            <FormRow
              label="Size"
              for="cam-resolution"
              notes={resolutionNotes}
              errors={sizeError ? [sizeError] : []}
            >
              <Select
                id="cam-resolution"
                size="sm"
                placeholder=""
                value={resolutionIndex}
                onchange={(e: Event) => {
                  const choice =
                    RESOLUTIONS[
                      Number((e.currentTarget as HTMLSelectElement).value)
                    ];
                  if (choice) setSize(choice[0], choice[1]);
                }}
              >
                {#each RESOLUTIONS as [w, h], i (i)}
                  <option value={i}>
                    {w === 0 ? "Camera maximum" : `${String(w)} × ${String(h)}`}
                  </option>
                {/each}
                <option value={-1} disabled>Custom</option>
              </Select>
              <div class="size">
                <Input
                  type="number"
                  size="sm"
                  aria-label="Width"
                  min="0"
                  step="1"
                  bind:value={draftSize[0]}
                  onchange={() => {
                    setSize(draftSize[0], draftSize[1]);
                  }}
                />
                ×
                <Input
                  type="number"
                  size="sm"
                  aria-label="Height"
                  min="0"
                  step="1"
                  bind:value={draftSize[1]}
                  onchange={() => {
                    setSize(draftSize[0], draftSize[1]);
                  }}
                />
              </div>
            </FormRow>
          </SettingsCard>
        </div>
      </div>

      <div class="tuning">
        <SettingsCard title="Brightness">
          <SliderField
            id="cam-exposure"
            label="Exposure"
            value={toDisplay(ranges.exposure, settings.exposure)}
            min={ranges.exposure.min}
            max={ranges.exposure.max}
            step={ranges.exposure.step}
            unit={ranges.exposure.unit}
            auto={{ value: 0, fallback: ranges.exposure.fallback }}
            notes={exposureNotes}
            warnings={exposureWarnings}
            onchange={(v: number) => {
              set("exposure", toFile(ranges.exposure, v));
            }}
          />

          <SliderField
            id="cam-gain"
            label="Gain"
            value={toDisplay(ranges.gain, settings.gain)}
            min={ranges.gain.min}
            max={ranges.gain.max}
            step={ranges.gain.step}
            unit={ranges.gain.unit}
            auto={{ value: 0, fallback: ranges.gain.fallback }}
            notes={gainNotes}
            onchange={(v: number) => {
              set("gain", toFile(ranges.gain, v));
            }}
          />

          {#if driver.gamma}
            <SliderField
              id="cam-gamma"
              label="Gamma"
              value={toDisplay(ranges.gamma, settings.gamma)}
              min={ranges.gamma.min}
              max={ranges.gamma.max}
              step={ranges.gamma.step}
              unit={ranges.gamma.unit}
              auto={{ value: 1, fallback: ranges.gamma.fallback, label: "Off" }}
              notes={gammaNotes}
              warnings={gammaWarnings}
              onchange={(v: number) => {
                set("gamma", toFile(ranges.gamma, v));
              }}
            />
          {/if}
        </SettingsCard>

        <SettingsCard title="White balance">
          <FormRow label="Mode" for="cam-wb" notes={wbNotes}>
            <Select
              id="cam-wb"
              size="sm"
              placeholder=""
              value={wbMode}
              onchange={(e: Event) => {
                setWBMode(
                  (e.currentTarget as HTMLSelectElement).value as WBMode,
                );
              }}
            >
              {#if driver.wbProfiles}
                <option value="OUTDOOR"
                  >Automatic, outdoor (green carpet)</option
                >
                <option value="INDOOR">Automatic, indoor (gray carpet)</option>
              {:else}
                <option value="auto">Automatic</option>
              {/if}
              <option value="manual">Manual red / blue</option>
            </Select>
          </FormRow>

          {#if wbMode === "manual"}
            <SliderField
              id="cam-wb-red"
              label="Red"
              value={toDisplay(ranges.whiteBalance, manualWB.red)}
              min={ranges.whiteBalance.min}
              max={ranges.whiteBalance.max}
              step={ranges.whiteBalance.step}
              unit={ranges.whiteBalance.unit}
              notes={wbChannelNotes}
              onchange={(v: number) => {
                set("white_balance", {
                  ...manualWB,
                  red: toFile(ranges.whiteBalance, v),
                });
              }}
            />
            <SliderField
              id="cam-wb-blue"
              label="Blue"
              value={toDisplay(ranges.whiteBalance, manualWB.blue)}
              min={ranges.whiteBalance.min}
              max={ranges.whiteBalance.max}
              step={ranges.whiteBalance.step}
              unit={ranges.whiteBalance.unit}
              notes={wbChannelNotes}
              onchange={(v: number) => {
                set("white_balance", {
                  ...manualWB,
                  blue: toFile(ranges.whiteBalance, v),
                });
              }}
            />
          {/if}
        </SettingsCard>
      </div>
    </div>
  {:else}
    <P size="sm" class="mb-2 text-gray-600 dark:text-gray-400"
      >{text.noInstance}</P
    >
  {/if}
</section>

<style>
  .camera-panel {
    max-width: 1600px;
  }

  .layout {
    display: grid;
    grid-template-columns: minmax(0, 3fr) minmax(360px, 2fr);
    gap: 1.5rem;
    align-items: start;
  }

  .main {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    min-width: 0;
  }

  .capture {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(340px, 1fr));
    gap: 1rem;
    align-items: start;
  }

  @media (max-width: 1100px) {
    .layout {
      grid-template-columns: minmax(0, 1fr);
    }
  }

  .size {
    display: flex;
    align-items: center;
    gap: 0.35rem;
  }
</style>
