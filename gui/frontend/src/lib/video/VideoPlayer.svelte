<script lang="ts">
  import { Spinner } from "flowbite-svelte";
  import {
    VideoStream,
    type Mode,
    type StreamFormat,
    type StreamStatus,
  } from "./videoStream";
  import { video as text } from "../text/video";

  // One camera's live stream. Built to be reused by a grid later: each
  // player owns its own connection, and "keyframes" mode costs a fraction of
  // the bandwidth and decode work.
  interface Props {
    cameraId: number;
    mode?: Mode;
    // Fills its parent instead of a 16:9 box, with no caption: for tiles.
    compact?: boolean;
    // Degrees clockwise, a multiple of 90. 90 and 270 swap the video's
    // width and height so it still fits the box.
    rotate?: number;
    mirror?: boolean;
    // Each status message from the host, e.g. for its received frame rate.
    onstatus?: (status: StreamStatus) => void;
  }

  let {
    cameraId,
    mode = "full",
    compact = false,
    rotate = 0,
    mirror = false,
    onstatus,
  }: Props = $props();

  let quarterTurn = $derived(((rotate % 180) + 180) % 180 === 90);
  let transform = $derived(
    `translate(-50%, -50%) rotate(${String(rotate)}deg)${mirror ? " scaleX(-1)" : ""}`,
  );

  let video: HTMLVideoElement | undefined = $state();
  let status = $state<StreamStatus | null>(null);
  let format = $state<StreamFormat | null>(null);
  let error = $state<string | null>(null);

  // A hidden tab (or one navigated away from) disconnects: no bandwidth or
  // decode work for video nobody can see, and the host closes the camera's
  // socket once no one is watching.
  let visible = $state(document.visibilityState === "visible");

  $effect(() => {
    const update = (): void => {
      visible = document.visibilityState === "visible";
    };

    document.addEventListener("visibilitychange", update);

    return () => {
      document.removeEventListener("visibilitychange", update);
    };
  });

  // The stream effect reads these, not the props: a prop is a getter on the
  // parent's expression, so the effect would otherwise rerun (reconnecting
  // and blanking the video) whenever anything that expression reads changes,
  // e.g. the Camera Layout tiles' slot objects, rebuilt every second. A
  // $derived only signals when the value itself changes.
  let streamCamera = $derived(cameraId);
  let streamMode = $derived(mode);

  $effect(() => {
    if (!video || !visible) return;

    status = null;

    const stream = new VideoStream(video, streamCamera, streamMode, {
      status: (s) => {
        status = s;
        onstatus?.(s);
      },
      format: (f) => {
        format = f;
      },
      error: (message) => {
        error = message;
      },
    });

    stream.start();

    return () => {
      stream.stop();
    };
  });

  // Still expecting video, as opposed to an error or streaming being off.
  let waiting = $derived(
    !error && (!status || (status.active && (!status.receiving || !format))),
  );

  let overlay = $derived.by((): string | null => {
    if (error) return error;
    if (!status) return text.connecting;
    if (!status.active) return text.streamingOff;
    if (status.problem) return text.cantListen(status.address, status.problem);
    if (!status.receiving) return text.waiting(status.address);
    if (!format) return text.waitingKeyframe;

    return null;
  });
</script>

<div class="player" class:compact>
  <video
    bind:this={video}
    muted
    autoplay
    playsinline
    class:quarter-turn={quarterTurn}
    style:transform
  ></video>

  {#if overlay}
    <div class="overlay">
      {#if waiting}<Spinner size={compact ? "4" : "6"} color="gray" />{/if}
      <span>{overlay}</span>
    </div>
  {/if}
</div>

{#if !compact}
  <p class="meta">
    {#if format}
      {format.width}×{format.height}, {format.codec}
    {/if}
    {#if status}
      · {status.address}{status.source ? ` from ${status.source}` : ""}
      {#if status.receiving}· {text.receivedFps(status.fps)}{/if}
    {/if}
    {#if mode === "keyframes"}
      · keyframes only
    {/if}
  </p>
{/if}

<style>
  .player {
    position: relative;
    width: 100%;
    aspect-ratio: 16 / 9;
    border-radius: 4px;
    background: #111;
    overflow: hidden;
    /* Lets the video size itself in container units when rotated. */
    container-type: size;
  }

  .player.compact {
    position: absolute;
    inset: 0;
    aspect-ratio: auto;
    border-radius: 0;
  }

  video {
    position: absolute;
    top: 50%;
    left: 50%;
    width: 100cqw;
    height: 100cqh;
    object-fit: contain;
  }

  video.quarter-turn {
    width: 100cqh;
    height: 100cqw;
  }

  .compact .overlay {
    gap: 0.4rem;
    padding: 0.5rem;
    font-size: 0.7rem;
  }

  .overlay {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.75rem;
    padding: 1rem;
    color: #ddd;
    font-size: 0.9rem;
    text-align: center;
  }

  .meta {
    margin: 0.3rem 0 0;
    color: var(--text-muted);
    font-size: 0.8rem;
  }
</style>
