import { describe, expect, it } from "vitest";

import type { Card, Rank, Suit } from "@/shared/types/matchTypes";

import declarationMeldsGolden from "../../../../../server/internal/game/testdata/contract/declaration_melds.json";
import variantRulesGolden from "../../../../../server/internal/game/testdata/contract/variant_rules.json";
import { detectDeclarations } from "./declarations";
import { rulesForVariant } from "./variantRules";

function parseHand(ids: string[]): Card[] {
  return ids.map((id) => ({ rank: id[0] as Rank, suit: id[1] as Suit }));
}

function toCardIds(cards: Card[]): string[] {
  return cards.map((c) => `${c.rank}${c.suit}`);
}

describe("variantRules.ts mirrors game.RulesFor", () => {
  it.each(["bitola", "croatia"] as const)(
    "%s declarationOverlap matches the Go preset",
    (variant) => {
      expect(rulesForVariant(variant).declarationOverlap).toBe(
        variantRulesGolden[variant].declarationOverlap,
      );
    },
  );

  it("the two variants genuinely differ on overlap", () => {
    expect(variantRulesGolden.bitola.declarationOverlap).toBe(false);
    expect(variantRulesGolden.croatia.declarationOverlap).toBe(true);
  });

  it("flags any Go rule field the client does not yet mirror", () => {
    expect(Object.keys(variantRulesGolden.bitola).sort()).toEqual([
      "allPassOutcome",
      "dealShape",
      "declarationOverlap",
      "declarationTiming",
      "hasTrumpCandidate",
      "tieRule",
    ]);
  });
});

describe("declarations.ts mirrors game.detectDeclarations", () => {
  it.each(declarationMeldsGolden.map((c) => [c.name, c] as const))(
    "%s — reproduces the Go melds under both overlap settings",
    (_name, c) => {
      const hand = parseHand(c.hand);

      const withoutOverlap = detectDeclarations(hand, false).map((d) => ({
        type: d.type,
        value: d.value,
        cards: toCardIds(d.cards),
      }));
      expect(withoutOverlap).toEqual(c.withoutOverlap);

      const withOverlap = detectDeclarations(hand, true).map((d) => ({
        type: d.type,
        value: d.value,
        cards: toCardIds(d.cards),
      }));
      expect(withOverlap).toEqual(c.withOverlap);
    },
  );

  it("the golden actually exercises the overlap divergence", () => {
    const diverging = declarationMeldsGolden.filter(
      (c) => JSON.stringify(c.withoutOverlap) !== JSON.stringify(c.withOverlap),
    );
    expect(diverging.length).toBeGreaterThan(0);
  });
});
