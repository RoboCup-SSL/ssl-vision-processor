// The setup checklist (checklist/): the top-bar pill and the docked panel.
// Each item has a title, why it matters, and what "done" looks like; notes
// are the short per-item or per-camera status lines.

import { plural } from "./common";

export const checklist = {
  pill: "Setup Checklist",
  pillTitle: "Open the setup checklist: a guide through setting up vision.",
  title: "Setup checklist",
  intro:
    "A guide through a new setup, in the order it's usually done. Skip ahead whenever you like: nothing here is locked.",
  progress: (done: number, total: number): string =>
    `${String(done)} of ${String(total)} done`,
  close: "Close checklist",
  restart: "Restart checklist",
  restartTitle: "Restart the checklist?",
  restartMessage:
    "This clears your ticks only. No settings change. Steps the host checks for itself will show as done again straight away.",
  restartConfirm: "Restart",
  complete: "Setup complete",
  completeDetail:
    "Every step is done. Close the checklist to get the space back; the pill stays in the top bar, and comes back yellow if something needs attention.",
  why: "Why",
  doneWhen: "Done when",
  usuallyAfter: (titles: string[]): string =>
    `Usually done after: ${titles.join(", ")}`,
  confirm: "I've checked this",
  reconfirm: "Checked again",
  noCameras: "No cameras yet: add them under Layout.",
  camera: (id: number): string => `Camera ${String(id)}`,
  seeAlerts: "See alerts",

  // Settings menu entries (layout/Shell.svelte).
  menuOpen: "Setup checklist",
  menuShow: "Show setup checklist button",

  status: {
    done: "Done",
    todo: "To do",
    attention: "Needs attention",
    recheck: "Changed since you checked it",
  },

  notes: {
    connecting: "Connecting to the host…",
    disconnected:
      "Lost the connection to the host. Is it still running? This page reconnects by itself.",
    noNetworkState: "Waiting for the host's socket status…",
    networkProblem: "A network alert is open.",
    layoutProblem: "A camera layout alert is open.",
    heard: (fps: number | undefined): string =>
      fps ? `Sending detections, ${String(fps)} fps` : "Sending detections",
    notHeard: "Nothing heard from this camera yet",
    notCalibrated: "Not calibrated yet",
    calibratedNotLocked:
      "Calibrated, not locked: lock it so a restart doesn't recalibrate",
    locked: "Calibrated and locked",
    teamsProblem: "A team height alert is open.",
    firstRun: "Not saved yet: nothing is published until the first save",
    unsaved: (n: number): string =>
      `${String(n)} unsaved ${plural(n, "change")}`,
    saved: "Saved",
  },

  items: {
    connection: {
      title: "Connected to host",
      why: "This page talks to the host program, which talks to the cameras. Nothing else on this list works without it.",
      doneWhen:
        "The User Interface badge in the top bar is green and says OPEN.",
      action: "",
    },
    network: {
      title: "Network",
      why: "Vision processors, the game controller and the autoref find each other through multicast addresses. A wrong address or network interface is the most common reason nothing shows up.",
      doneWhen:
        "The host's sockets are open with no network alerts. Testing on one machine? Turn on loopback multicast when the alert asks for it.",
      action: "Open Network",
    },
    field: {
      title: "Field dimensions",
      why: "Calibration fits the camera image to these measurements. Wrong numbers here mean a wrong calibration later, and changing them afterwards means calibrating again.",
      doneWhen:
        "The field matches the one you actually have, including which optional lines are painted. Applying the setup wizard ticks this for you.",
      action: "Open setup wizard",
    },
    layout: {
      title: "Camera layout",
      why: "Each camera covers one region of the field and is known by its camera ID. Two cameras on one ID overwrite each other's detections.",
      doneWhen:
        "Every camera has its own region and ID, and there are no layout alerts.",
      action: "Open Layout",
    },
    processors: {
      title: "Vision processors running",
      why: "A vision processor reads one camera and sends its detections. The host only sees a camera once its processor is running.",
      doneWhen:
        "Every camera below shows as sending. If one stays silent, check that its vision_processor is running and its camera is plugged in.",
      doneWhenSupervised:
        "Every camera below shows as sending. This host starts them itself; if one stays silent, check its camera is plugged in and look at the Camera page.",
      action: "Open Camera",
    },
    image: {
      title: "Camera image",
      why: "Calibration and color detection both work on the image. Settle mounting, focus, exposure, gain and white balance first: changing them later can undo the calibration and colors.",
      doneWhen:
        "The camera is mounted for good and in focus. Robot blobs are colored, not washed out to white, and field lines are white, not orange.",
      action: "Open Camera",
    },
    calibration: {
      title: "Calibrate and lock",
      why: "Calibration maps the image onto the field. Locking keeps it, so restarting the host or a vision processor doesn't calibrate again.",
      doneWhen:
        "Each camera's reprojected view lines up with the field lines, and its calibration is locked.",
      action: "Open Geometry",
    },
    color: {
      title: "Color references",
      why: "Robots and the ball are found by their colors. References tuned for different lighting misread team colors and lose the ball.",
      doneWhen:
        "The ball and every robot's markers are detected with the right colors across the whole camera view.",
      action: "Open Color",
    },
    teams: {
      title: "Teams and robot heights",
      why: "Robot height corrects where a robot appears from a camera's angle. A wrong height puts robots in the wrong place, worst at the field edges.",
      doneWhen:
        "Every team in use has its real robot height, and there are no team height alerts.",
      action: "Open Teams",
    },
    save: {
      title: "Save",
      why: "Changes apply live but stay in memory until saved, and are lost when the host restarts. A new configuration also publishes nothing until its first save.",
      doneWhen: "There are no unsaved changes.",
      action: "Save…",
    },
    liveField: {
      title: "Check on Live Field",
      why: "The last check is the one that counts: what the teams and the autoref will actually receive.",
      doneWhen:
        "A ball and a robot moved around the field show up where they really are, with the right team color and ID, without flickering between cameras.",
      action: "Open Live Field",
    },
  },
};
