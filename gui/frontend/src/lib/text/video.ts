// Live video (video/VideoPlayer.svelte, video/videoStream.ts): what shows
// over the picture while there's nothing to play.

export const video = {
  connecting: "Connecting…",
  streamingOff:
    "Streaming is off for this camera (stream.active). Nothing to show.",
  cantListen: (address: string, problem: string): string =>
    `Can't listen for video on ${address}: ${problem}. Retrying…`,
  waiting: (address: string): string =>
    `Waiting for video on ${address}. Is the vision_processor running?`,
  waitingKeyframe: "Waiting for a keyframe…",
  // The rate frames arrive from vision_processor.
  receivedFps: (fps: number): string => `${fps.toFixed(3)} fps received`,
  fps: (fps: number): string => `${fps.toFixed(3)} fps`,
  fpsTitle:
    "Frames per second arriving from vision_processor, not the rate shown here.",
  noMSE: "This browser doesn't support Media Source Extensions.",
  unsupportedCodec: (codec: string): string =>
    `This browser can't play ${codec} (H.264). On Linux, Chromium and Firefox need the system's FFmpeg libraries; Google Chrome bundles its own.`,
  lostConnection: "Lost the connection to the host. Reconnecting…",
  decodeError: (message: string): string => `Video decode error: ${message}`,
  playbackFailed: (err: string): string => `Playback failed: ${err}`,
};
