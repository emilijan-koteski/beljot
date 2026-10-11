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

  it.each(SUITS)("returns null for the french deck (%s)", (suit) => {
    expect(suitIconUrl(suit, "french")).toBeNull();
  });
});

describe("suit icon assets", () => {
  it.each(SUITS)("ships a croatian icon for %s", (suit) => {
    expect(existsSync(join(SUITS_DIR, "croatian", `${suit}.webp`))).toBe(true);
  });
});

describe("SUIT_ACCENT", () => {
  it("gives the croatian deck four pairwise-distinct accents", () => {
    const accents = SUITS.map((suit) => SUIT_ACCENT.croatian[suit]);
    expect(new Set(accents).size).toBe(4);
  });

  it("keeps leaves and acorns distinct (the green/green collision)", () => {
    expect(SUIT_ACCENT.croatian.S).not.toBe(SUIT_ACCENT.croatian.C);
  });

  it("preserves the french palette exactly", () => {
    expect(SUIT_ACCENT.french).toEqual({
      S: "#1a1a1a",
      H: "#c62828",
      D: "#c62828",
      C: "#1a1a1a",
    });
  });

  it("never paints a croatian suit in the french red or black", () => {
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
  it("keys the name by deck as well as suit", () => {
    expect(suitNameKey("D", "french")).toBe("match.card.suit.french.D");
    expect(suitNameKey("D", "croatian")).toBe("match.card.suit.croatian.D");
  });

  it.each(DECKS)("resolves a distinct key per suit on %s", (deck) => {
    const keys = SUITS.map((suit) => suitNameKey(suit, deck));
    expect(new Set(keys).size).toBe(4);
  });
});

const REVEAL_PANEL_LIGHTEST_STOP = "#20402b";

/** WCAG relative luminance. */
function luminance(hex: string): number {
  const channel = (c: number) => {
    const s = c / 255;
    return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  };
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
  const FLOOR = contrast(SUIT_INK_ON_FELT.french.H, REVEAL_PANEL_LIGHTEST_STOP);

  it("uses the french red anchor as the floor", () => {
    expect(FLOOR).toBeCloseTo(4.89, 2);
  });

  it.each(DECKS.flatMap((deck) => SUITS.map((suit) => [deck, suit] as const)))(
    "%s %s ink clears the floor on the panel's lightest end",
    (deck, suit) => {
      expect(
        contrast(SUIT_INK_ON_FELT[deck][suit], REVEAL_PANEL_LIGHTEST_STOP),
      ).toBeGreaterThanOrEqual(FLOOR);
    },
  );

  it("keeps hearts identical across decks — the one suit that is red in both", () => {
    expect(SUIT_INK_ON_FELT.croatian.H).toBe(SUIT_INK_ON_FELT.french.H);
  });
});

describe("SUIT_GLOW_ALPHA derivation", () => {
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
