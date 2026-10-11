#!/usr/bin/env node
import { readdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const FRENCH_CARDS_DIR = resolve(
  dirname(fileURLToPath(import.meta.url)),
  "../client/public/cards/french",
);

const SUIT_RED = "#c62828";
const SUIT_BLACK = "#1a1a1a";
const COURT_NAVY = "#34497f";
const OUTLINE_WIDTH = "3";

const replacements = [
  [/fill="white"/g, 'fill="none"'],
  [/(fill|stroke)="red"/g, `$1="${SUIT_RED}"`],
  [/(fill|stroke)="black"/g, `$1="${SUIT_BLACK}"`],
  [/(fill|stroke)="#44F"/gi, `$1="${COURT_NAVY}"`],
];

/** Gives the face rect a visible stroke width unless it already has one. */
function addOutlineWidth(svg) {
  return svg.replace(/<rect\b[^>]*>/, (rect) =>
    rect.includes("stroke-width")
      ? rect
      : rect.replace(/\sstroke=/, ` stroke-width="${OUTLINE_WIDTH}" stroke=`),
  );
}

const files = readdirSync(FRENCH_CARDS_DIR).filter((f) => f.endsWith(".svg"));
if (files.length === 0) {
  console.error(`No SVGs found in ${FRENCH_CARDS_DIR}`);
  process.exit(1);
}

let changed = 0;
for (const file of files) {
  const path = join(FRENCH_CARDS_DIR, file);
  const original = readFileSync(path, "utf8");

  let out = original;
  for (const [pattern, value] of replacements)
    out = out.replace(pattern, value);
  out = addOutlineWidth(out);

  if (out !== original) {
    writeFileSync(path, out);
    changed += 1;
  }
}

console.log(
  `recolor-cards: ${files.length} faces scanned, ${changed} rewritten`,
);
console.log(`  red   -> ${SUIT_RED}`);
console.log(`  black -> ${SUIT_BLACK}`);
console.log(`  #44F  -> ${COURT_NAVY} (court accent; gold #FC4 left alone)`);
console.log(`  face  -> transparent (outline stroke-width ${OUTLINE_WIDTH})`);
