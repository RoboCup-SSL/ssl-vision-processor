<script lang="ts">
  import {
    VideoStream,
    type Mode,
    type StreamFormat,
    type StreamStatus,
  } from "./videoStream";

  // One camera's live stream. Built to be reused by a grid later: each
  // player owns its own connection, and "keyframes" mode costs a fraction of
  // the bandwidth and decode work.
  interface Props {
    cameraId: number;
    mode?: Mode;
  }

  let { cameraId, mode = "full" }: Props = $props();

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

  $effect(() => {
    if (!video || !visible) return;

    status = null;

    const stream = new VideoStream(video, cameraId, mode, {
      status: (s) => {
        status = s;
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

  let overlay = $derived.by((): string | null => {
    if (error) return error;
    if (!status) return "Connecting…";
    if (!status.active)
      return `Streaming is off for this camera (stream.active). Nothing to show.`;
    if (status.problem)
      return `Can't listen for video on ${status.address}: ${status.problem}. Retrying…`;
    if (!status.receiving)
      return `Waiting for video on ${status.address}. Is the vision_processor running?`;
    if (!format) return "Waiting for a keyframe…";

    return null;
  });
</script>

<div class="player">
  <video bind:this={video} muted autoplay playsinline></video>

  {#if overlay}
    <div class="overlay">{overlay}</div>
  {/if}
</div>

<p class="meta">
  {#if format}
    {format.width}×{format.height}, {format.codec}
  {/if}
  {#if status}
    · {status.address}{status.source ? ` from ${status.source}` : ""}
  {/if}
  {#if mode === "keyframes"}
    · keyframes only
  {/if}
</p>

<style>
  .player {
    position: relative;
    width: 100%;
    aspect-ratio: 16 / 9;
    border-radius: 4px;
    background: #111;
    overflow: hidden;
  }

  video {
    width: 100%;
    height: 100%;
    object-fit: contain;
  }

  .overlay {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 1rem;
    color: #ddd;
    font-size: 0.9rem;
    text-align: center;
  }

  .meta {
    margin: 0.3rem 0 0;
    color: #666;
    font-size: 0.8rem;
  }
</style>
