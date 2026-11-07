// Global, client-only UI preferences -- these control how this browser's UI
// behaves, not anything vision_processor or config.yml cares about, so they
// live in localStorage rather than going through any /api/config endpoint.
// Same module-level $state pattern as color.svelte.ts's colorConfig; any
// component can import `preferences` and both read it and bind: to it
// directly.
const STORAGE_KEY = "vision-processor-gui:preferences";

interface Preferences {
  // Shows supplementary explanations for surprising/non-obvious behavior
  // (e.g. YuvPositionPane's "only X% of colors are reachable here" callout)
  // -- a returning user likely doesn't need these repeated forever.
  tooltipsEnabled: boolean;
  // Skips confirmation ceremonies around settings that are safe for an
  // experienced operator to change freely -- e.g. the minimum-reference-weight
  // floor (WeightTriangle.svelte) starts unlocked instead of behind its
  // warning popup.
  expertUser: boolean;
}

function loadPreferences(): Preferences {
  const defaults: Preferences = { tooltipsEnabled: true, expertUser: false };

  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return defaults;

    const parsed = JSON.parse(raw) as Partial<Preferences>;
    return {
      tooltipsEnabled: parsed.tooltipsEnabled ?? defaults.tooltipsEnabled,
      expertUser: parsed.expertUser ?? defaults.expertUser,
    };
  } catch {
    return defaults;
  }
}

export const preferences = $state<Preferences>(loadPreferences());

// Persists on every change. $effect.root is what makes this legal at module
// scope: a plain $effect() here would throw (effects normally need an
// enclosing component to own their lifecycle), but a root effect runs
// immediately and stays alive for the life of the page -- exactly right for
// a global preference every component just binds to directly, with no
// dedicated setter functions to remember to call.
$effect.root(() => {
  $effect(() => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(preferences));
    } catch {
      // Private browsing / storage disabled -- preference just won't persist
      // across reloads, not worth surfacing as an error.
    }
  });
});
