import { existsSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import type { Suit } from "@/shared/types/matchTypes";

import type { CardDeck } from "./cardFace";
import {
  SUIT_ACCENT,
  SUIT_GLOW_ALPHA,
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

// The reveal panel's gradient runs rgba(32,64,43,.98) -> rgba(13,38,23,.98)
// (TrumpReveal.tsx). The LIGHTEST end is the worst case for light ink, so that is
// what the floor is measured against.
const PANEL_LIGHTEST = "#20402b";

/** WCAG relative luminance. */
function luminance(hex: string): number {
  const channel = (c: number) => {
    const s = c / 255;
    return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  };
  // Read each channel by name rather than destructuring a mapped array: under
  // noUncheckedIndexedAccess the elements come back `number | undefined`.
  const r = channel(parseInt(hex.slice(1, 3), 16));
  const g = channel(parseInt(hex.slice(3, 5), 16));
  const b = channel(parseInt(hex.slice(5, 7), 16));
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(a: string, b: string): number {
  const la = luminance(a);
  const lb = luminance(b);
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
}

describe("SUIT_INK_ON_FELT legibility", () => {
  // The bar is French's own weaker anchor, so "Croatian is as readable as French"
  // is a measured claim rather than a design opinion. Acorn brown shipped at
  // 4.73 while these were judgment values; this is what caught it.
  const FLOOR = contrast(SUIT_INK_ON_FELT.french.H, PANEL_LIGHTEST);

  it("uses the french red anchor as the floor", () => {
    expect(FLOOR).toBeCloseTo(4.89, 2);
  });

  it.each(DECKS.flatMap((deck) => SUITS.map((suit) => [deck, suit] as const)))(
    "%s %s ink clears the floor on the panel's lightest end",
    (deck, suit) => {
      expect(contrast(SUIT_INK_ON_FELT[deck][suit], PANEL_LIGHTEST)).toBeGreaterThanOrEqual(FLOOR);
    },
  );

  it("keeps hearts identical across decks — the one suit that is red in both", () => {
    expect(SUIT_INK_ON_FELT.croatian.H).toBe(SUIT_INK_ON_FELT.french.H);
  });
});

describe("SUIT_GLOW_ALPHA derivation", () => {
  // Alpha rises with the accent's luminance, linear between French's two
  // anchors and clamped to them. The rule is only trustworthy because it
  // reproduces French's pre-existing 55/77/77/55 exactly — that is asserted
  // below, so a change to the rule fails on French before it reaches Croatian.
  const LO = { l: luminance("#1a1a1a"), a: 0x55 };
  const HI = { l: luminance("#c62828"), a: 0x77 };

  function expected(accent: string): number {
    const t = (luminance(accent) - LO.l) / (HI.l - LO.l);
    return Math.min(HI.a, Math.max(LO.a, Math.round(LO.a + t * (HI.a - LO.a))));
  }

  it.each(DECKS.flatMap((deck) => SUITS.map((suit) => [deck, suit] as const)))(
    "%s %s alpha follows the accent's luminance",
    (deck, suit) => {
      expect(parseInt(SUIT_GLOW_ALPHA[deck][suit], 16)).toBe(expected(SUIT_ACCENT[deck][suit]));
    },
  );

  it("reproduces the french asymmetry that predates the per-deck palette", () => {
    expect(SUIT_GLOW_ALPHA.french).toEqual({ S: "55", H: "77", D: "77", C: "55" });
  });
});
