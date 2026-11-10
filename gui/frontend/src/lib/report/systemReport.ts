// The system report: one JSON file a user attaches to an issue, so the
// developers know what they ran, where, and in which browser. Built in the
// browser from the host's /api/system, /api/network and /api/alerts (and
// /api/config, if the user includes it) plus what the browser knows about
// itself, and only ever downloaded or copied, never sent.
//
// vision_processor's own state (its logs, its announced config) isn't in it
// yet.

import { get } from "svelte/store";
import { requestJSON } from "../api";
import type {
  ConfigResponse,
  HostAlert,
  NetworkState,
  SystemInfo,
} from "../apiTypes";
import { preferences } from "../preferences.svelte";
import { connectionState } from "../wrapper-bus";
import { recentErrors } from "./errorLog";

// Bumped when a field changes meaning or moves, so a reader can tell old
// reports apart.
const FORMAT = 1;

// What a part of the report holds when the host couldn't answer for it. The
// report is still made: a host that is down is when one is most wanted.
export interface Unavailable {
  error: string;
}

export interface SystemReport {
  report: {
    format: number;
    generated: string;
    generatedLocal: string;
  };
  host: SystemInfo | Unavailable;
  network: NetworkState | Unavailable;
  alerts: HostAlert[] | Unavailable;
  // The working vision.yml and its unsaved changes, only when asked for.
  config?: ConfigResponse | Unavailable;
  frontend: ReturnType<typeof frontend>;
  browser: Awaited<ReturnType<typeof browser>>;
  errors: typeof recentErrors;
}

export function isUnavailable(value: object): value is Unavailable {
  return "error" in value && typeof value.error === "string";
}

async function fetchOr<T>(url: string): Promise<T | Unavailable> {
  try {
    return (await (await requestJSON(url)).json()) as T;
  } catch (err) {
    return { error: err instanceof Error ? err.message : String(err) };
  }
}

function frontend() {
  return {
    ...__FRONTEND_BUILD__,
    // Vite's dev server (npm run dev) rather than the bundle the host embeds.
    dev: import.meta.env.DEV,
    url: location.href,
    connection: get(connectionState),
    preferences: { ...preferences },
  };
}

// Chromium's structured user agent. Not in TypeScript's DOM types, and not in
// Firefox or Safari.
interface UserAgentData {
  getHighEntropyValues(hints: string[]): Promise<Record<string, unknown>>;
}

async function userAgentData(): Promise<Record<string, unknown> | null> {
  const data = (navigator as { userAgentData?: UserAgentData }).userAgentData;
  if (!data) return null;

  try {
    return await data.getHighEntropyValues([
      "platform",
      "platformVersion",
      "architecture",
      "bitness",
      "fullVersionList",
    ]);
  } catch {
    return null;
  }
}

function mediaQuery(query: string): boolean | null {
  return typeof matchMedia === "function" ? matchMedia(query).matches : null;
}

// The H.264 profiles a camera's stream may use, as MSE codec strings. Live
// video needs MediaSource and the stream's profile.
const H264 = {
  constrainedBaseline: "avc1.42E01F",
  main: "avc1.4D401F",
  high: "avc1.64001F",
};

function videoSupport() {
  const mse = typeof MediaSource !== "undefined";

  return {
    mediaSource: mse,
    managedMediaSource: "ManagedMediaSource" in window,
    h264: Object.fromEntries(
      Object.entries(H264).map(([profile, codec]) => [
        profile,
        mse && MediaSource.isTypeSupported(`video/mp4; codecs="${codec}"`),
      ]),
    ),
  };
}

async function browser() {
  const dates = Intl.DateTimeFormat().resolvedOptions();
  const hours = new Intl.DateTimeFormat(undefined, {
    hour: "numeric",
  }).resolvedOptions();

  return {
    userAgent: navigator.userAgent,
    userAgentData: await userAgentData(),
    locale: {
      languages: [...navigator.languages],
      dates: dates.locale,
      numbers: Intl.NumberFormat().resolvedOptions().locale,
      calendar: dates.calendar,
      numberingSystem: dates.numberingSystem,
      hourCycle: hours.hourCycle ?? null,
    },
    time: {
      zone: dates.timeZone,
      utcOffsetMinutes: -new Date().getTimezoneOffset(),
    },
    display: {
      screen: `${String(screen.width)}x${String(screen.height)}`,
      viewport: `${String(innerWidth)}x${String(innerHeight)}`,
      devicePixelRatio,
      colorDepth: screen.colorDepth,
      prefersDark: mediaQuery("(prefers-color-scheme: dark)"),
      darkMode: document.documentElement.classList.contains("dark"),
      reducedMotion: mediaQuery("(prefers-reduced-motion: reduce)"),
    },
    hardware: {
      cpus: navigator.hardwareConcurrency,
      // Chromium only, rounded to a power of 2 and capped at 8.
      memoryGB: (navigator as { deviceMemory?: number }).deviceMemory ?? null,
    },
    online: navigator.onLine,
    cookies: navigator.cookieEnabled,
    // Not in jsdom.
    secureContext:
      (globalThis as { isSecureContext?: boolean }).isSecureContext ?? null,
    video: videoSupport(),
  };
}

export async function buildReport(
  options: { includeConfig?: boolean } = {},
): Promise<SystemReport> {
  const now = new Date();
  const [host, network, alerts, config, browserInfo] = await Promise.all([
    fetchOr<SystemInfo>("/api/system"),
    fetchOr<NetworkState>("/api/network"),
    fetchOr<HostAlert[]>("/api/alerts"),
    options.includeConfig ? fetchOr<ConfigResponse>("/api/config") : undefined,
    browser(),
  ]);

  return {
    report: {
      format: FORMAT,
      generated: now.toISOString(),
      generatedLocal: now.toString(),
    },
    host,
    network,
    alerts,
    ...(config && { config }),
    frontend: frontend(),
    browser: browserInfo,
    errors: [...recentErrors],
  };
}

// vision-gui-report-<host>-<UTC time>.json: sorts by time, and says whose it
// is when several are attached to one issue.
export function reportFileName(report: SystemReport): string {
  const hostname = isUnavailable(report.host)
    ? location.hostname
    : report.host.host.hostname || location.hostname;
  const stamp = report.report.generated
    .slice(0, 19)
    .replaceAll(/[-:]/g, "")
    .replace("T", "-");

  return `vision-gui-report-${hostname.replaceAll(/[^\w.-]/g, "_")}-${stamp}Z.json`;
}

export function downloadReport(report: SystemReport): void {
  const blob = new Blob([JSON.stringify(report, null, 2) + "\n"], {
    type: "application/json",
  });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");

  link.href = url;
  link.download = reportFileName(report);
  link.click();
  // Not straight away: Firefox can still be starting the download.
  setTimeout(() => {
    URL.revokeObjectURL(url);
  }, 1000);
}
