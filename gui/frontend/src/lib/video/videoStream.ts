// Plays one camera's live stream from /ws/video/{id} through Media Source
// Extensions. The host sends JSON "status" and "format" messages and binary
// fragmented MP4 (an init segment, then one segment per frame), which are
// appended to a SourceBuffer in sequence mode: only durations matter, so the
// host's arrival-time estimates are fine.
//
// MSE buffers, so the player drifts behind live; it jumps forward whenever it
// falls more than maxLag behind the newest frame, and trims what's been
// watched so memory stays bounded.

import { video as text } from "../text/video";

export type Mode = "full" | "keyframes";

// Mirrors gui/internal/video's Status.
export interface StreamStatus {
  camera: number;
  address: string;
  active: boolean;
  receiving: boolean;
  packets: number;
  // The rate frames arrive from vision_processor over the last 200 ms, not
  // what this player shows (keyframes mode shows about one a second).
  fps: number;
  source?: string;
  // Why the host's socket isn't fully open; it retries until it is.
  problem?: string;
}

// Mirrors gui/internal/video's Format.
export interface StreamFormat {
  codec: string;
  width: number;
  height: number;
}

export interface StreamEvents {
  status: (status: StreamStatus) => void;
  format: (format: StreamFormat) => void;
  // null clears a previous error.
  error: (message: string | null) => void;
}

type Op =
  | { kind: "append"; data: ArrayBuffer }
  | { kind: "type"; mime: string }
  | { kind: "remove"; start: number; end: number };

// Seconds behind the newest frame before jumping to it. Keyframes mode has a
// frame a second, so it needs more slack.
const MAX_LAG: Record<Mode, number> = { full: 1, keyframes: 3 };
// Seconds of already watched video kept buffered.
const KEEP_BEHIND = 5;
const RECONNECT_MS = [500, 1000, 2000, 5000];

export class VideoStream {
  private ws: WebSocket | null = null;
  private source: MediaSource | null = null;
  private buffer: SourceBuffer | null = null;
  private objectURL: string | null = null;
  private mime: string | null = null;
  private queue: Op[] = [];
  private unsupported = false;
  private stopped = false;
  private attempts = 0;
  private timer: ReturnType<typeof setTimeout> | null = null;

  constructor(
    private readonly video: HTMLVideoElement,
    private readonly cameraId: number,
    private readonly mode: Mode,
    private readonly events: StreamEvents,
  ) {}

  start(): void {
    if (typeof MediaSource === "undefined") {
      this.events.error(text.noMSE);

      return;
    }

    this.video.addEventListener("error", this.onVideoError);
    this.connect();
  }

  stop(): void {
    this.stopped = true;

    if (this.timer !== null) clearTimeout(this.timer);

    this.ws?.close();
    this.ws = null;
    this.video.removeEventListener("error", this.onVideoError);
    this.resetMedia();
  }

  private connect(): void {
    const scheme = location.protocol === "https:" ? "wss" : "ws";
    const ws = new WebSocket(
      `${scheme}://${location.host}/ws/video/${String(this.cameraId)}?mode=${this.mode}`,
    );

    ws.binaryType = "arraybuffer";
    ws.onopen = () => {
      this.attempts = 0;
    };
    ws.onmessage = (e: MessageEvent<string | ArrayBuffer>) => {
      this.onMessage(e.data);
    };
    ws.onclose = () => {
      if (this.stopped || this.ws !== ws) return;

      this.ws = null;
      this.events.error(text.lostConnection);

      const delay =
        RECONNECT_MS[Math.min(this.attempts, RECONNECT_MS.length - 1)] ?? 5000;
      this.attempts++;
      this.timer = setTimeout(() => {
        this.timer = null;
        this.connect();
      }, delay);
    };

    this.ws = ws;
  }

  private onMessage(data: string | ArrayBuffer): void {
    if (typeof data !== "string") {
      if (!this.unsupported) this.enqueue({ kind: "append", data });

      return;
    }

    const message = JSON.parse(data) as { type: string };

    if (message.type === "status") {
      this.events.status(message as unknown as StreamStatus);
    } else if (message.type === "format") {
      this.onFormat(message as unknown as StreamFormat);
    }
  }

  private onFormat(format: StreamFormat): void {
    this.events.format(format);

    const mime = `video/mp4; codecs="${format.codec}"`;

    if (!MediaSource.isTypeSupported(mime)) {
      this.unsupported = true;
      this.events.error(text.unsupportedCodec(format.codec));

      return;
    }

    this.unsupported = false;
    this.events.error(null);

    if (!this.source) {
      this.openMedia(mime);
    } else if (mime !== this.mime) {
      this.enqueue({ kind: "type", mime });
    }

    this.mime = mime;
  }

  private openMedia(mime: string): void {
    const source = new MediaSource();
    this.source = source;
    this.objectURL = URL.createObjectURL(source);
    this.video.src = this.objectURL;

    source.addEventListener(
      "sourceopen",
      () => {
        if (this.source !== source) return;

        const buffer = source.addSourceBuffer(mime);
        buffer.mode = "sequence";
        buffer.addEventListener("updateend", () => {
          this.followLive();
          this.pump();
        });

        this.buffer = buffer;
        this.pump();
      },
      { once: true },
    );
  }

  private resetMedia(): void {
    this.queue = [];
    this.buffer = null;
    this.source = null;
    this.mime = null;

    if (this.objectURL) {
      URL.revokeObjectURL(this.objectURL);
      this.objectURL = null;
    }

    this.video.removeAttribute("src");
    this.video.load();
  }

  // A decode error kills the media element for good; start over, which
  // makes the host resend the format and a keyframe.
  private readonly onVideoError = (): void => {
    if (this.stopped || !this.video.error) return;

    this.events.error(text.decodeError(this.video.error.message));
    this.resetMedia();
    this.ws?.close();
  };

  private enqueue(op: Op): void {
    this.queue.push(op);
    this.pump();
  }

  private pump(): void {
    const buffer = this.buffer;
    if (!buffer || buffer.updating) return;

    const op = this.queue.shift();
    if (!op) return;

    try {
      switch (op.kind) {
        case "append":
          buffer.appendBuffer(op.data);
          break;
        case "type":
          buffer.changeType(op.mime);
          this.pump();
          break;
        case "remove":
          buffer.remove(op.start, op.end);
          break;
      }
    } catch (err) {
      if (err instanceof DOMException && err.name === "QuotaExceededError") {
        // Full: drop what's been watched and try this one again after.
        this.queue.unshift(op);
        this.trim(0);
      } else {
        this.events.error(text.playbackFailed(String(err)));
        this.resetMedia();
        this.ws?.close();
      }
    }
  }

  // Jumps to near the newest frame if playback fell behind, and trims what's
  // already been watched.
  private followLive(): void {
    const ranges = this.video.buffered;
    if (ranges.length === 0) return;

    const end = ranges.end(ranges.length - 1);

    if (end - this.video.currentTime > MAX_LAG[this.mode]) {
      this.video.currentTime = Math.max(
        ranges.start(ranges.length - 1),
        end - 0.1,
      );
    }

    if (this.video.paused) {
      this.video.play().catch(() => {
        // Autoplay can refuse until the user interacts; the muted video
        // normally doesn't.
      });
    }

    this.trim(KEEP_BEHIND);
  }

  private trim(keep: number): void {
    const ranges = this.video.buffered;
    if (ranges.length === 0) return;

    const start = ranges.start(0);
    const until = this.video.currentTime - keep;

    if (until - start > 1 && !this.queue.some((op) => op.kind === "remove")) {
      this.queue.push({ kind: "remove", start, end: until });
    }
  }
}
