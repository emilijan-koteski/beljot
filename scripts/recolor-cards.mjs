#!/usr/bin/env node
// Retarget the vendored card faces onto the app's design tokens.
//
// The faces come out of https://www.me.uk/cards/makeadeck.cgi with a generic
// print palette: pure #ff0000 / #000000 ink on an opaque white face. Two of
// those choices fight the app:
//
//   1. #ff0000 is not our red. The same view draws suit glyphs with
//      `var(--suit-red)`, so an untreated deck puts two different reds for the
//      same suit on screen at once (trump prompt: candidate card above its
//      picker tiles).
//   2. The opaque white face forces a tint layer on top to warm it to the
//      table's parchment. Blending is lossy and environment-dependent — it
//      shifts every ink on the card, tints the top and bottom indices
//      differently under a vertical gradient, and inverts under forced-colors.
//      Dropping the face to transparent lets `PlayingCard`'s own parchment
//      background BE the face, so the ink lands on its exact token value.
//
// SVGs loaded through `<img>` are isolated from page CSS — no `currentColor`,
// no custom properties reach inside — so this has to happen in the files.
//
// Idempotent: safe to re-run. Run it after re-downloading from the builder.
//   node scripts/recolor-cards.mjs

import { readdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

// The FRENCH deck only. `public/cards/` is now one folder per deck, and this
// script is French-specific by construction: the substitutions below name the
// builder's palette (`red`, `black`, `#44F`) and the builder only produces the
// French set. The Croatian deck is owner-authored raster imported by
// `scripts/import-croatian-deck.py` and must never be run through here.
const CARDS_DIR = resolve(
  dirname(fileURLToPath(import.meta.url)),
  "../client/public/cards/french",
);

// Keep in sync with the `.game-table` scope in client/src/index.css — these are
// the values the suit glyphs beside the cards resolve to.
const SUIT_RED = "#c62828";
const SUIT_BLACK = "#1a1a1a";

// Court-card accents (J/Q/K only). These have no counterpart in the theme — they
// are deck-local, and this script is their one definition. The builder's #44F is
// a bright periwinkle that, once the red is muted to #c62828, becomes the loudest
// thing on every court card and reads cold against the parchment; a navy recedes
// and lets the red and gold carry the artwork. The gold is deliberately NOT
// remapped to --brass: brass sits too close to the parchment face in both hue and
// value, which flattens the crowns and collars.
const COURT_NAVY = "#34497f";

// The face rect's outline is `stroke` with no `stroke-width`, i.e. 1 unit of a
// 240-unit viewBox — which renders at 0.2-0.4px and effectively disappears,
// leaving overlapping cards with no separating edge. 3 units puts it at ~1px at
// the `md` box and scales with the card instead of being a fixed CSS border.
const OUTLINE_WIDTH = "3";

const replacements = [
  // Transparent face — PlayingCard's parchment background shows through.
  [/fill="white"/g, 'fill="none"'],
  [/(fill|stroke)="red"/g, `$1="${SUIT_RED}"`],
  [/(fill|stroke)="black"/g, `$1="${SUIT_BLACK}"`],
  [/(fill|stroke)="#44F"/gi, `$1="${COURT_NAVY}"`],
];

function addOutlineWidth(svg) {
  return svg.replace(/<rect\b[^>]*>/, (rect) =>
    rect.includes("stroke-width") ? rect : rect.replace(/\sstroke=/, ` stroke-width="${OUTLINE_WIDTH}" stroke=`),
  );
}

const files = readdirSync(CARDS_DIR).filter((f) => f.endsWith(".svg"));
if (files.length === 0) {
  console.error(`No SVGs found in ${CARDS_DIR}`);
  process.exit(1);
}

let changed = 0;
for (const file of files) {
  const path = join(CARDS_DIR, file);
  const original = readFileSync(path, "utf8");

  let out = original;
  for (const [pattern, value] of replacements) out = out.replace(pattern, value);
  out = addOutlineWidth(out);

  if (out !== original) {
    writeFileSync(path, out);
    changed += 1;
  }
}

console.log(`recolor-cards: ${files.length} faces scanned, ${changed} rewritten`);
console.log(`  red   -> ${SUIT_RED}`);
console.log(`  black -> ${SUIT_BLACK}`);
console.log(`  #44F  -> ${COURT_NAVY} (court accent; gold #FC4 left alone)`);
console.log(`  face  -> transparent (outline stroke-width ${OUTLINE_WIDTH})`);
