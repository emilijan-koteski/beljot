import { existsSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import type { Rank, Suit } from "@/shared/types/matchTypes";

import type { CardDeck } from "./cardFace";
import { CARD_SIZES, cardFaceUrl, resolveCardDeck } from "./cardFace";

const RANKS: Rank[] = ["7", "8", "9", "T", "J", "Q", "K", "A"];
const SUITS: Suit[] = ["S", "H", "D", "C"];
const CARD_IDS = RANKS.flatMap((rank) => SUITS.map((suit) => `${rank}${suit}`));

const CARDS_DIR = join(process.cwd(), "public", "cards");

const DECKS: Array<{ deck: CardDeck; ext: string }> = [
  { deck: "french", ext: "svg" },
  { deck: "croatian", ext: "webp" },
];

describe("cardFaceUrl", () => {
  it("derives the URL from the deck and the card ID", () => {
    expect(cardFaceUrl("KS", "french")).toBe("/cards/french/KS.svg");
    expect(cardFaceUrl("KS", "croatian")).toBe("/cards/croatian/KS.webp");
  });

  it("uses the rank character, not the '10' display label", () => {
    expect(cardFaceUrl("TH", "french")).toBe("/cards/french/TH.svg");
    expect(cardFaceUrl("TH", "croatian")).toBe("/cards/croatian/TH.webp");
  });

  it("carries each deck's own file extension", () => {
    for (const { deck, ext } of DECKS) {
      expect(cardFaceUrl("AC", deck).endsWith(`.${ext}`)).toBe(true);
    }
  });
});

describe("resolveCardDeck", () => {
  it("passes through every shipped deck", () => {
    for (const { deck } of DECKS) {
      expect(resolveCardDeck(deck)).toBe(deck);
    }
  });

  it.each([
    undefined,
    null,
    "",
    "german",
    "croatia",
    "FRENCH",
    "toString",
    "constructor",
    "hasOwnProperty",
    "__proto__",
  ])("falls back to french for %o", (value) => {
    expect(resolveCardDeck(value)).toBe("french");
  });

  it("never lets a rejected value reach the asset path", () => {
    expect(cardFaceUrl("KS", resolveCardDeck("toString"))).toBe("/cards/french/KS.svg");
  });
});

describe("deck assets", () => {
  describe.each(DECKS)("$deck", ({ deck, ext }) => {
    it.each(CARD_IDS)("ships artwork for %s", (cardId) => {
      expect(existsSync(join(CARDS_DIR, deck, `${cardId}.${ext}`))).toBe(true);
    });
  });
});

describe("CARD_SIZES", () => {
  it.each(Object.entries(CARD_SIZES))("%s matches the artwork's 5:7 ratio", (_size, dims) => {
    expect(dims.height / dims.width).toBeCloseTo(336 / 240, 10);
  });

  it.each(Object.entries(CARD_SIZES))("%s radius matches the artwork's rx", (_size, dims) => {
    expect(dims.radius / dims.width).toBeCloseTo(12 / 240, 10);
  });
});
