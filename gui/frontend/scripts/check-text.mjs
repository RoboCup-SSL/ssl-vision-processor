// Fails when user-facing prose is written outside src/lib/text/, so tooltips,
// messages, and docs stay in the one folder an editor looks through. Run by
// `npm run check:text` (and so `make test`).
//
// It flags string literals ("...", '...', `...`) that read like sentences
// (at least MIN_WORDS words and MIN_CHARS characters, not a CSS class list),
// and in .svelte files, plain text between tags with MIN_WORDS real words.
// Short labels and button text can stay in components.
// Comments are ignored. A "text-ok" comment on the line or up to two above
// exempts it, for the rare sentence that is code (an internal error, say).
import { readFileSync, readdirSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = new URL("../src/", import.meta.url).pathname;
const SKIP = ["lib/text/", "proto/"];
const MIN_WORDS = 6;
const MIN_CHARS = 40;

// Yields [literal text, line number] for every string literal in source,
// skipping comments. Template expressions (${...}) become "x".
export function* literals(source) {
  let i = 0;
  let line = 1;

  while (i < source.length) {
    const c = source[i];
    const next = source[i + 1];

    if (c === "\n") {
      line++;
      i++;
    } else if (c === "/" && next === "/") {
      while (i < source.length && source[i] !== "\n") i++;
    } else if (c === "/" && next === "*") {
      const end = source.indexOf("*/", i + 2);
      const stop = end === -1 ? source.length : end + 2;
      line += source.slice(i, stop).split("\n").length - 1;
      i = stop;
    } else if (c === "<" && source.startsWith("<!--", i)) {
      const end = source.indexOf("-->", i + 4);
      const stop = end === -1 ? source.length : end + 3;
      line += source.slice(i, stop).split("\n").length - 1;
      i = stop;
    } else if (c === '"' || c === "'" || c === "`") {
      const start = line;
      let text = "";
      i++;

      while (i < source.length && source[i] !== c) {
        if (source[i] === "\\") {
          text += source[i + 1] ?? "";
          i += 2;
        } else if (c === "`" && source[i] === "$" && source[i + 1] === "{") {
          let depth = 1;
          i += 2;
          while (i < source.length && depth > 0) {
            if (source[i] === "{") depth++;
            if (source[i] === "}") depth--;
            if (source[i] === "\n") line++;
            i++;
          }
          text += "x";
        } else {
          if (source[i] === "\n") {
            // A quote with no partner on its line is an apostrophe in
            // markup text, not a string; resynchronize at the next line.
            if (c !== "`") break;
            line++;
          }
          text += source[i];
          i++;
        }
      }

      i++;
      yield [text, start];
    } else {
      i++;
    }
  }
}

// "flex items-center gap-2": all class-shaped tokens, a good share of them
// hyphenated or variant-prefixed.
export function isClassList(text) {
  const tokens = text.trim().split(/\s+/);
  const shaped = tokens.every((t) => /^[a-z0-9!:[\]/.\-_%#()&>=*]+$/.test(t));
  const marked = tokens.filter((t) => /[-:]/.test(t)).length;

  return shaped && marked * 3 >= tokens.length;
}

// The parts of a .svelte file that hold code: <script> blocks, {...}
// expressions, and quoted attribute values. Markup text between tags is
// prose by nature and isn't checked here. Line breaks are kept so line
// numbers still match.
export function svelteCode(source) {
  let out = "";
  let i = 0;

  const keepLines = (text) => text.replace(/[^\n]/g, " ");

  while (i < source.length) {
    if (source.startsWith("<script", i)) {
      const end = source.indexOf("</script>", i);
      const stop = end === -1 ? source.length : end;
      out += source.slice(i, stop);
      i = stop;
    } else if (source.startsWith("<style", i)) {
      const end = source.indexOf("</style>", i);
      const stop = end === -1 ? source.length : end;
      out += keepLines(source.slice(i, stop));
      i = stop;
    } else if (source.startsWith("<!--", i)) {
      const end = source.indexOf("-->", i);
      const stop = end === -1 ? source.length : end + 3;
      out += keepLines(source.slice(i, stop));
      i = stop;
    } else if (source[i] === "{") {
      let depth = 0;
      const start = i;
      do {
        if (source[i] === "{") depth++;
        if (source[i] === "}") depth--;
        i++;
      } while (i < source.length && depth > 0);
      out += source.slice(start, i);
    } else if (source[i] === "=" && source[i + 1] === '"') {
      const end = source.indexOf('"', i + 2);
      const stop = end === -1 ? source.length : end + 1;
      out += source.slice(i, stop);
      i = stop;
    } else {
      out += source[i] === "\n" ? "\n" : " ";
      i++;
    }
  }

  return out;
}

// Yields [text, line number] for plain text between tags in a .svelte file
// (outside <script>, <style>, comments, and {...} expressions).
export function* markupText(source) {
  const blank = (m) => m.replace(/[^\n]/g, " ");
  const stripped = source
    .replace(/<script[\s\S]*?<\/script>/g, blank)
    .replace(/<style[\s\S]*?<\/style>/g, blank)
    .replace(/<!--[\s\S]*?-->/g, blank);

  for (const m of stripped.matchAll(/>([^<>]+)</g)) {
    let text = "";
    let depth = 0;

    for (const c of m[1]) {
      if (c === "{") depth++;
      else if (c === "}") depth--;
      else if (depth === 0) text += c;
    }

    const line =
      stripped.slice(0, m.index).split("\n").length +
      (m[1].match(/^\s*/)?.[0].split("\n").length ?? 1) -
      1;

    yield [text, line];
  }
}

// Real words only: values in {...} were dropped above.
export function isMarkupProse(text) {
  return (text.match(/[A-Za-z][A-Za-z']+/g) ?? []).length >= MIN_WORDS;
}

export function isProse(text) {
  const words = text.trim().split(/\s+/);

  return (
    text.length >= MIN_CHARS &&
    words.length >= MIN_WORDS &&
    /[A-Za-z]{3,}/.test(text) &&
    !isClassList(text)
  );
}

// Every problem in src/, as "path:line: text..." lines.
export function check(root = ROOT) {
  const problems = [];

  for (const entry of readdirSync(root, { recursive: true })) {
    const path = String(entry);
    // Tests name what they check; that's not user-facing text.
    const isTest = /\.test\.ts$/.test(path) || path === "test-setup.ts";

    if (
      !/\.(ts|svelte)$/.test(path) ||
      isTest ||
      SKIP.some((s) => path.startsWith(s))
    ) {
      continue;
    }

    const raw = readFileSync(join(root, path), "utf8");
    const lines = raw.split("\n");
    const source = path.endsWith(".svelte") ? svelteCode(raw) : raw;

    const exempt = (line) =>
      [lines[line - 1], lines[line - 2], lines[line - 3]].some((l) =>
        l?.includes("text-ok"),
      );
    const report = (text, line) => {
      problems.push(
        `src/${path}:${String(line)}: ${text.trim().replace(/\s+/g, " ").slice(0, 70)}...`,
      );
    };

    for (const [text, line] of literals(source)) {
      if (isProse(text) && !exempt(line)) report(text, line);
    }

    if (path.endsWith(".svelte")) {
      for (const [text, line] of markupText(raw)) {
        if (isMarkupProse(text) && !exempt(line)) report(text, line);
      }
    }
  }

  return problems;
}

// Run directly (npm run check:text), not when a test imports this file.
if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const problems = check();

  if (problems.length > 0) {
    console.error(
      "User-facing text belongs in src/lib/text/ (see its README.md):\n",
    );
    for (const p of problems) console.error(`  ${p}`);
    process.exit(1);
  }

  console.log(`check-text: OK (${relative(process.cwd(), ROOT)})`);
}
