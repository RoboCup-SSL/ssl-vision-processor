<script lang="ts">
  import { onMount } from "svelte";
  import type { VisionInstance } from "../layout/nav.svelte";
  import { cameraDoc } from "../config.svelte";
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
  import NoteTip from "../NoteTip.svelte";
  import SliderField from "./SliderField.svelte";

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

  const KIND_LABEL: Record<VideoDevice["kind"], string> = {
    "by-id": "this camera, any port",
    "by-path": "this USB port",
    node: "may change",
  };

  let pathNotes = $derived.by((): string[] => {
    const notes = [
      "Device, image, or video file. Unset means /dev/video{id}.",
      "The list shows cameras on the host running this GUI.",
    ];
    const device = devices.find((d) => d.path === effectivePath);

    if (device?.kind === "by-id")
      notes.push("Follows this camera to any USB port.");
    if (device?.kind === "by-path")
      notes.push("Follows the USB port, whichever camera is in it.");
    if (!device && devices.length > 0 && effectivePath.startsWith("/dev/"))
      notes.push("Not on this host; fine if the camera is on another machine.");

    return notes;
  });

  let pathWarnings = $derived(
    /^\/dev\/video\d+$/.test(effectivePath)
      ? [
          "/dev/videoN can change when devices re-enumerate. A /dev/v4l/by-id path follows the camera.",
        ]
      : [],
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
      sizeError = "Width and height must be whole numbers.";

      return;
    }

    if ((width === 0) !== (height === 0)) {
      sizeError = "Set both, or 0 for both (the camera's maximum).";

      return;
    }

    sizeError = null;
    set("width", width);
    set("height", height);
  }

  let resolutionNotes = $derived(
    settings.driver === "OPENCV"
      ? [
          "0 asks for the largest the camera offers.",
          "Check the size under the video: OpenCV doesn't always get what it asks for.",
        ]
      : [
          "0 is the sensor's maximum.",
          "Bayer cameras are processed at half this internally.",
        ],
  );

  // Exposure, gain, gamma

  let exposureNotes = $derived([
    "Longer is brighter but blurs motion.",
    "At or above the frame time (33 ms at 30 fps) the frame rate drops.",
    ...(settings.driver === "OPENCV"
      ? [
          "Shown in the camera's 100 µs steps: vision_processor sends the file's value × 1000, so 166 here is 16.6 ms and the file stores 0.166.",
          "Placeholder range: the UC70 dev camera's 3-2047.",
        ]
      : ["Placeholder range until vision_processor reports the camera's."]),
  ]);

  let exposureWarnings = $derived(
    settings.driver === "OPENCV" && settings.exposure === 0
      ? [
          "vision_processor's OpenCV driver sends auto_exposure = 1 for Auto, which V4L2 cameras read as Manual: the camera keeps its last exposure.",
        ]
      : [],
  );

  let gainNotes = $derived([
    "Brighter but noisier. Noise means more false blobs and more processing time.",
    "Robot ids or team colors flickering means the image is too bright.",
    "0 means automatic, so a manual gain of exactly 0 isn't possible.",
    ...(settings.driver === "OPENCV"
      ? [
          "Auto leaves the camera's own setting: OpenCV doesn't turn auto gain on.",
          "Placeholder range: the UC70 dev camera's 0-8.",
        ]
      : ["Placeholder range until vision_processor reports the camera's."]),
  ]);

  let gammaNotes = $derived([
    "Below 1 evens out bright and dark areas; above 1 adds color contrast.",
    ...(settings.driver === "OPENCV"
      ? [
          "V4L2 counts gamma × 100 and vision_processor sends the file's value unscaled, so 100 here is a gamma of 1.0.",
          "Placeholder range: the UC70 dev camera's 100-300.",
        ]
      : ["Placeholder range until vision_processor reports the camera's."]),
  ]);

  let gammaWarnings = $derived(
    settings.driver === "OPENCV" && settings.gamma !== 1
      ? [
          "vision_processor's OpenCV driver currently ignores gamma other than 1.0 (opencvdriver.cpp:43 checks the wrong way round).",
        ]
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
    if (driver.wbProfiles)
      return [
        "Outdoor and indoor are Spinnaker's two auto profiles, for green and gray carpet.",
      ];

    if (settings.driver === "MVIMPACT")
      return ["Automatic calibrates once, from the first frame after start."];

    return ["Automatic turns on the camera's own auto white balance."];
  });

  let wbChannelNotes = $derived(
    settings.driver === "OPENCV"
      ? [
          "Sent as V4L2 red/blue balance in camera units. Many UVC webcams only have a color temperature control, so this may do nothing; the UC70 dev camera has no red/blue controls at all.",
          "Placeholder range: generic 8-bit.",
        ]
      : [
          settings.driver === "SPINNAKER"
            ? "Balance ratio relative to green; 1.0 is neutral."
            : "Gain relative to green; 1.0 is neutral.",
          "Placeholder range until vision_processor reports the camera's.",
        ],
  );
</script>

<section class="camera-panel">
  <h2>Camera settings</h2>

  {#if instance}
    <p class="banner">
      These apply when cam {instance.cameraId}'s vision_processor restarts: it
      reads <code>camera:</code> only at startup.
      {#if configPath}
        The host writes them to <code>{configPath}</code>.
      {:else}
        This camera has no <code>config_path</code>, so the host doesn't write
        its config file; copy these to it by hand.
      {/if}
    </p>

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
          <fieldset>
            <legend>Device</legend>

            <div class="row">
              <label for="cam-driver">Driver</label>
              <select
                id="cam-driver"
                value={settings.driver}
                onchange={(e) => {
                  set("driver", e.currentTarget.value as Driver);
                }}
              >
                {#each Object.entries(DRIVERS) as [key, info] (key)}
                  <option value={key}>{info.label}</option>
                {/each}
              </select>
              <NoteTip
                id="cam-driver-notes"
                notes={[
                  "SPINNAKER and MVIMPACT need vision_processor built with their SDKs.",
                  "Unset means SPINNAKER.",
                ]}
              />
            </div>

            {#if driver.selectsBy === "id"}
              <div class="row">
                <label for="cam-id">Camera index</label>
                <input
                  id="cam-id"
                  type="number"
                  min="0"
                  step="1"
                  value={settings.id}
                  onchange={(e) => {
                    const v = e.currentTarget.valueAsNumber;
                    if (Number.isInteger(v) && v >= 0) set("id", v);
                  }}
                />
                <NoteTip
                  id="cam-id-notes"
                  notes={[
                    "Position in the SDK's camera list, in detection order.",
                  ]}
                />
              </div>
            {:else}
              <div class="row">
                <label for="cam-path">Path</label>
                <div class="stack">
                  <select
                    aria-label="Cameras on this host"
                    value={devices.findIndex((d) => d.path === effectivePath)}
                    onchange={(e) => {
                      const device = devices[Number(e.currentTarget.value)];
                      if (device) set("path", device.path);
                    }}
                  >
                    {#each devices as device, i (device.path)}
                      <option value={i}>
                        {device.name} ({KIND_LABEL[device.kind]}) {device.path}
                      </option>
                    {/each}
                    <option value={-1} disabled>
                      {devices.length === 0
                        ? "No cameras on this host"
                        : "Other"}
                    </option>
                  </select>
                  <input
                    id="cam-path"
                    type="text"
                    spellcheck="false"
                    value={settings.path ?? ""}
                    placeholder={`/dev/video${String(settings.id)}`}
                    onchange={(e) => {
                      const v = e.currentTarget.value.trim();
                      set("path", v === "" ? undefined : v);
                    }}
                  />
                </div>
                <NoteTip id="cam-path-notes" notes={pathNotes} />
              </div>
              {#each pathWarnings as warning (warning)}
                <p class="warning">{warning}</p>
              {/each}
            {/if}
          </fieldset>

          <fieldset>
            <legend>Resolution</legend>

            <div class="row">
              <label for="cam-resolution">Size</label>
              <div class="stack">
                <select
                  id="cam-resolution"
                  value={resolutionIndex}
                  onchange={(e) => {
                    const choice = RESOLUTIONS[Number(e.currentTarget.value)];
                    if (choice) setSize(choice[0], choice[1]);
                  }}
                >
                  {#each RESOLUTIONS as [w, h], i (i)}
                    <option value={i}>
                      {w === 0
                        ? "Camera maximum"
                        : `${String(w)} × ${String(h)}`}
                    </option>
                  {/each}
                  <option value={-1} disabled>Custom</option>
                </select>
                <div class="size">
                  <input
                    type="number"
                    aria-label="Width"
                    min="0"
                    step="1"
                    bind:value={draftSize[0]}
                    onchange={() => {
                      setSize(draftSize[0], draftSize[1]);
                    }}
                  />
                  ×
                  <input
                    type="number"
                    aria-label="Height"
                    min="0"
                    step="1"
                    bind:value={draftSize[1]}
                    onchange={() => {
                      setSize(draftSize[0], draftSize[1]);
                    }}
                  />
                </div>
              </div>
              <NoteTip id="cam-resolution-notes" notes={resolutionNotes} />
            </div>
            {#if sizeError}<p class="error">{sizeError}</p>{/if}
          </fieldset>
        </div>
      </div>

      <div class="tuning">
        <fieldset>
          <legend>Brightness</legend>

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
        </fieldset>

        <fieldset>
          <legend>White balance</legend>

          <div class="row">
            <label for="cam-wb">Mode</label>
            <select
              id="cam-wb"
              value={wbMode}
              onchange={(e) => {
                setWBMode(e.currentTarget.value as WBMode);
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
            </select>
            <NoteTip id="cam-wb-notes" notes={wbNotes} />
          </div>

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
        </fieldset>
      </div>
    </div>
  {:else}
    <p class="hint">Select a vision processor on the left first.</p>
  {/if}
</section>

<style>
  .camera-panel {
    max-width: 1600px;
  }

  h2 {
    margin: 0 0 0.5rem;
  }

  .banner {
    padding: 0.4rem 0.6rem;
    border-radius: 4px;
    background: #eef4ff;
    color: #1e3a8a;
    font-size: 0.85rem;
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

  .capture fieldset {
    margin-bottom: 0;
  }

  @media (max-width: 1100px) {
    .layout {
      grid-template-columns: minmax(0, 1fr);
    }
  }

  fieldset {
    border: 1px solid #ddd;
    border-radius: 4px;
    margin-bottom: 1rem;
  }

  .row {
    display: grid;
    grid-template-columns: 7rem 1fr 1.25rem;
    align-items: center;
    gap: 0.5rem;
    margin: 0.45rem 0;
    font-size: 0.85rem;
  }

  .stack {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    min-width: 0;
  }

  .size {
    display: flex;
    align-items: center;
    gap: 0.35rem;
  }

  .size input {
    width: 6rem;
  }

  select,
  input {
    font-size: 0.85rem;
  }

  input[type="text"] {
    font-family: monospace;
  }

  .warning,
  .error {
    margin: 0 0 0.4rem 7.5rem;
    font-size: 0.8rem;
  }

  .warning {
    color: #7a4a00;
  }

  .error {
    color: #c81e1e;
  }

  .hint {
    color: #666;
    font-size: 0.85rem;
  }
</style>
