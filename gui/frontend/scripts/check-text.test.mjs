import { describe, expect, it } from "vitest";
import {
  isClassList,
  isMarkupProse,
  isProse,
  literals,
  markupText,
  svelteCode,
} from "./check-text.mjs";

const all = (gen) => [...gen].map(([text]) => text);

describe("isProse", () => {
  it("flags a sentence", () => {
    expect(isProse("The camera reads this value only at startup.")).toBe(true);
  });

  it("ignores short labels and CSS class lists", () => {
    expect(isProse("Camera Layout")).toBe(false);
    expect(
      isProse(
        "flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400",
      ),
    ).toBe(false);
    expect(isClassList("mt-1 mb-3 flex rounded border")).toBe(true);
  });

  it("still flags lowercase prose without hyphens", () => {
    expect(isClassList("plus the boundary on the outer edges")).toBe(false);
  });
});

describe("literals", () => {
  it("finds strings and skips comments", () => {
    const src = `// "a commented out sentence that is long enough to flag"
const a = "one"; /* "also a comment" */ const b = 'two';`;

    expect(all(literals(src))).toEqual(["one", "two"]);
  });

  it("turns template expressions into a placeholder", () => {
    expect(all(literals("const t = `cam ${String(id)} of ${n}`;"))).toEqual([
      "cam x of x",
    ]);
  });

  it("reports the line a string starts on", () => {
    expect([...literals('\n\nconst s = "x";')][0][1]).toBe(3);
  });
});

describe("svelte files", () => {
  it("checks code and attributes, not markup text", () => {
    const code = svelteCode(
      `<script>const s = "in script";</script>\n<p title="an attribute">Markup text, isn't code.</p>`,
    );

    expect(all(literals(code))).toEqual(["in script", "an attribute"]);
  });

  it("finds markup prose and drops {expressions}", () => {
    const text = all(
      markupText(
        `<p>Every region {count} has exactly one camera assigned to it.</p><b>{label}</b>`,
      ),
    );

    expect(text.some(isMarkupProse)).toBe(true);
    expect(isMarkupProse(" ")).toBe(false);
  });
});
