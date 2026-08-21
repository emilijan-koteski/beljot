import { existsSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import type { Suit } from "@/shared/types/matchTypes";

import type { CardDeck } from "./cardFace";
import {
  SUIT_ACCENT,
  SUIT_INK_ON_FELT,
  suitAccent,
  suitGlowAlpha,
  suitIconUrl,
  suitInkOnFelt,
  suitNameKey,
} from "./suitArt";

const SUITS: Suit[] = ["S", "H", "D", "C"];
const DECKS: CardDeck[] = ["french", "croatian"];

const SUITS_DIR = join(process.cwd(), "public", "suits");

describe("suitIconUrl", () => {
  it("derives the icon URL from the deck and the suit", () => {
    expect(suitIconUrl("S", "croatian")).toBe("/suits/croatian/S.webp");
    expect(suitIconUrl("D", "croatian")).toBe("/suits/croatian/D.webp");
  });

  // The French deck ships no icon files at all — it keeps its Unicode glyphs,
  // which are already sized and kerned across four surfaces. Returning null
  // rather than a would-be-404 path is what forces callers to handle the glyph
  // case instead of silently rendering a broken image.
  it.each(SUITS)("returns null for the french deck (%s)", (suit) => {
    expect(suitIconUrl(suit, "french")).toBeNull();
  });
});

describe("suit icon assets", () => {
  // Same reasoning as the deck-face asset test: a missing icon fails silently
  // everywhere else. The img is decorative, the ancestor's label still reads
  // correctly, and production's SPA fallback answers 200 with index.html.
  it.each(SUITS)("ships a croatian icon for %s", (suit) => {
    expect(existsSync(join(SUITS_DIR, "croatian", `${suit}.webp`))).toBe(true);
  });
});

describe("SUIT_ACCENT", () => {
  // The whole point of Scope Amendment 1: the accent is per suit, not a
  // red/black split. Leaves and acorns are BOTH green-dominant in the artwork,
  // so acorns take the brown of their nut — collapsing them to one green would
  // defeat a per-suit palette, and nothing else would report it.
  it("gives the croatian deck four pairwise-distinct accents", () => {
    const accents = SUITS.map((suit) => SUIT_ACCENT.croatian[suit]);
    expect(new Set(accents).size).toBe(4);
  });

  it("keeps leaves and acorns distinct (the green/green collision)", () => {
    expect(SUIT_ACCENT.croatian.S).not.toBe(SUIT_ACCENT.croatian.C);
  });

  // French must be byte-identical to what shipped before this story: these are
  // the literals the four replaced call sites used, and `--suit-red` /
  // `--suit-black` resolve to exactly these inside `.game-table`.
  it("preserves the french palette exactly", () => {
    expect(SUIT_ACCENT.french).toEqual({
      S: "#1a1a1a",
      H: "#c62828",
      D: "#c62828",
      C: "#1a1a1a",
    });
  });

  it("never paints a croatian suit in the french red or black", () => {
    // A red halo round a gold bell, or a black one round a green leaf, is the
    // exact defect the amendment exists to avoid.
    for (const suit of ["S", "D", "C"] as Suit[]) {
      expect(SUIT_ACCENT.croatian[suit]).not.toBe("#c62828");
      expect(SUIT_ACCENT.croatian[suit]).not.toBe("#1a1a1a");
    }
  });

  it("covers every suit in every deck", () => {
    for (const deck of DECKS) {
      for (const suit of SUITS) {
        expect(SUIT_ACCENT[deck][suit]).toMatch(/^#[0-9a-f]{6}$/);
        expect(SUIT_INK_ON_FELT[deck][suit]).toMatch(/^#[0-9a-f]{6}$/);
      }
    }
  });

  it("keeps the on-felt french inks exactly as they were", () => {
    expect(SUIT_INK_ON_FELT.french).toEqual({
      S: "#f5f2e8",
      H: "#ff8585",
      D: "#ff8585",
      C: "#f5f2e8",
    });
  });
});

describe("palette accessors", () => {
  it.each(DECKS)("return the table value for every suit on %s", (deck) => {
    for (const suit of SUITS) {
      expect(suitAccent(suit, deck)).toBe(SUIT_ACCENT[deck][suit]);
      expect(suitInkOnFelt(suit, deck)).toBe(SUIT_INK_ON_FELT[deck][suit]);
      expect(suitGlowAlpha(suit, deck)).toMatch(/^[0-9a-f]{2}$/);
    }
  });

  // `Suit` is a compile-time union but not a runtime guarantee: the WS contract
  // types a suit as z.string(), so an unrecognised value reaches the UI. A raw
  // `SUIT_ACCENT[deck][suit]` yielded undefined, the consumers interpolated it,
  // and `border: "2px solid undefined"` was dropped by the browser — the orb's
  // whole chrome vanished. The old red/black test degraded to black instead, so
  // the accessors have to preserve that.
  it.each(DECKS)("degrade to a usable colour for an unknown suit on %s", (deck) => {
    const bogus = "X" as Suit;

    expect(suitAccent(bogus, deck)).toBe("#1a1a1a");
    expect(suitGlowAlpha(bogus, deck)).toBe("55");
    expect(suitInkOnFelt(bogus, deck)).toBe("#f5f2e8");
  });

  it("never returns undefined for any suit-shaped input", () => {
    for (const deck of DECKS) {
      for (const key of ["S", "H", "D", "C", "X", "", "toString"] as Suit[]) {
        expect(typeof suitAccent(key, deck)).toBe("string");
        expect(suitAccent(key, deck)).not.toBe("undefined");
        expect(typeof suitGlowAlpha(key, deck)).toBe("string");
        expect(typeof suitInkOnFelt(key, deck)).toBe("string");
      }
    }
  });
});

describe("suitNameKey", () => {
  // The seam that stops a gold bell being captioned "Diamonds". There is
  // deliberately only ONE suit vocabulary in the locale files — an earlier
  // word-keyed `match.suits.spades` set duplicated these values verbatim in all
  // four locales, and the parity test compares key SETS, not values, so nothing
  // could detect the two drifting apart.
  it("keys the name by deck as well as suit", () => {
    expect(suitNameKey("D", "french")).toBe("match.card.suit.french.D");
    expect(suitNameKey("D", "croatian")).toBe("match.card.suit.croatian.D");
  });

  it.each(DECKS)("resolves a distinct key per suit on %s", (deck) => {
    const keys = SUITS.map((suit) => suitNameKey(suit, deck));
    expect(new Set(keys).size).toBe(4);
  });
});
