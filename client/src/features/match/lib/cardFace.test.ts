import { existsSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import type { Rank, Suit } from "@/shared/types/matchTypes";

import { CARD_SIZES, cardFaceUrl } from "./cardFace";

const RANKS: Rank[] = ["7", "8", "9", "T", "J", "Q", "K", "A"];
const SUITS: Suit[] = ["S", "H", "D", "C"];

const CARDS_DIR = join(process.cwd(), "public", "cards");

describe("cardFaceUrl", () => {
  it("derives the URL from the card ID", () => {
    expect(cardFaceUrl("KS")).toBe("/cards/KS.svg");
  });

  it("uses the rank character, not the '10' display label", () => {
    expect(cardFaceUrl("TH")).toBe("/cards/TH.svg");
  });
});

describe("deck assets", () => {
  // A face resolved from a card ID fails silently at every other layer: the img
  // is decorative so there is no broken-image affordance, the aria-label still
  // reads correctly, and in production the SPA fallback answers with a 200. This
  // is the only thing standing between a renamed asset and a blank card in play.
  it.each(RANKS.flatMap((rank) => SUITS.map((suit) => `${rank}${suit}`)))(
    "ships artwork for %s",
    (cardId) => {
      expect(existsSync(join(CARDS_DIR, `${cardId}.svg`))).toBe(true);
    },
  );
});

describe("CARD_SIZES", () => {
  // The faces are authored `preserveAspectRatio="none"`, so they stretch to fill
  // whatever box they are given. A box at any other ratio distorts the artwork
  // without failing anything else — hence pinning the ratios here.
  it.each(Object.entries(CARD_SIZES))("%s matches the artwork's 5:7 ratio", (_size, dims) => {
    expect(dims.height / dims.width).toBeCloseTo(336 / 240, 10);
  });

  it.each(Object.entries(CARD_SIZES))("%s radius matches the artwork's rx", (_size, dims) => {
    expect(dims.radius / dims.width).toBeCloseTo(12 / 240, 10);
  });
});
