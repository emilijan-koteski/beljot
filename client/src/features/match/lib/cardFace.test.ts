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

// One row per shipped deck: the folder under `public/cards/` and the file
// extension its faces carry. Both assertions below iterate this, so adding a
// third deck is one row rather than a copied describe block.
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
  // The store's `user` is null before hydration and on unauthenticated paths,
  // and HTTP responses are cast rather than parsed — so an unknown value is
  // reachable at runtime even though the type says otherwise. Falling back to
  // French is what stops it resolving to a missing asset (a blank card, which
  // nothing else would report).
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
    // The game VARIANT value, which is the enum most likely to be cross-wired.
    "croatia",
    "FRENCH",
    // Prototype-chain keys. `value in DECK_EXT` accepted these — `"toString" in
    // DECK_EXT` is true — so a garbled preference resolved to a "deck" named
    // toString and the face URL became
    // `cards/toString/KS.function toString() { [native code] }`. Object.hasOwn
    // is the fix; these two rows are the regression guard.
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
  // A face resolved from a card ID fails silently at every other layer: the img
  // is decorative so there is no broken-image affordance, the aria-label still
  // reads correctly, and in production the SPA fallback answers with a 200. This
  // is the only thing standing between a renamed asset and a blank card in play.
  describe.each(DECKS)("$deck", ({ deck, ext }) => {
    it.each(CARD_IDS)("ships artwork for %s", (cardId) => {
      expect(existsSync(join(CARDS_DIR, deck, `${cardId}.${ext}`))).toBe(true);
    });
  });
});

describe("CARD_SIZES", () => {
  // The faces are authored `preserveAspectRatio="none"` (French) or encoded to
  // 5:7 at import time (Croatian), so they stretch to fill whatever box they are
  // given. A box at any other ratio distorts the artwork without failing
  // anything else — hence pinning the ratios here.
  it.each(Object.entries(CARD_SIZES))("%s matches the artwork's 5:7 ratio", (_size, dims) => {
    expect(dims.height / dims.width).toBeCloseTo(336 / 240, 10);
  });

  it.each(Object.entries(CARD_SIZES))("%s radius matches the artwork's rx", (_size, dims) => {
    expect(dims.radius / dims.width).toBeCloseTo(12 / 240, 10);
  });
});
