import { describe, expect, it } from "vitest";

import type { Card } from "@/shared/types/matchTypes";

import { detectDeclarations } from "./declarations";

function card(id: string): Card {
  return { rank: id[0] as Card["rank"], suit: id[1] as Card["suit"] };
}

function hand(ids: string[]): Card[] {
  return ids.map(card);
}

function summarize(decls: { type: string; value: number; cards: Card[] }[]): string[] {
  return decls.map(
    (d) => `${d.type}|${d.value}|${d.cards.map((c) => `${c.rank}${c.suit}`).join(",")}`,
  );
}

describe("detectDeclarations", () => {
  it("returns empty when no combinations exist", () => {
    expect(detectDeclarations(hand(["7S", "TD", "JH", "KC", "AS"]), false)).toEqual([]);
  });

  it("detects a tierce (3-card sequence = 20)", () => {
    const decls = detectDeclarations(hand(["7S", "8S", "9S", "TD", "JH"]), false);
    expect(decls).toHaveLength(1);
    expect(decls[0]?.type).toBe("sequence");
    expect(decls[0]?.value).toBe(20);
    expect(decls[0]?.cards).toHaveLength(3);
  });

  it("detects a quarte (4-card sequence = 50)", () => {
    const decls = detectDeclarations(hand(["JD", "QD", "KD", "AD", "7C", "8C"]), false);
    expect(decls).toHaveLength(1);
    expect(decls[0]?.value).toBe(50);
    expect(decls[0]?.cards).toHaveLength(4);
  });

  it("detects a 5+ sequence as 100pts", () => {
    const decls = detectDeclarations(hand(["7S", "8S", "9S", "TS", "JS", "7C"]), false);
    expect(decls).toHaveLength(1);
    expect(decls[0]?.value).toBe(100);
    expect(decls[0]?.cards).toHaveLength(5);
  });

  it("detects FoaK of jacks = 200", () => {
    const decls = detectDeclarations(hand(["JS", "JH", "JD", "JC", "7S"]), false);
    expect(decls).toHaveLength(1);
    expect(decls[0]?.type).toBe("four_of_a_kind");
    expect(decls[0]?.value).toBe(200);
  });

  it("does not detect four 8s (no point value)", () => {
    expect(detectDeclarations(hand(["8S", "8H", "8D", "8C", "7S"]), false)).toEqual([]);
  });

  describe("Bitola dedup", () => {
    it("drops tierce sharing a card with higher-value FoaK", () => {
      const decls = detectDeclarations(
        hand(["7S", "8S", "9S", "9D", "9H", "9C", "JD", "QC"]),
        false,
      );
      expect(decls).toHaveLength(1);
      expect(decls[0]?.type).toBe("four_of_a_kind");
      expect(decls[0]?.value).toBe(150);
    });

    it("drops quarte sharing a card with higher-value FoaK of jacks", () => {
      const decls = detectDeclarations(
        hand(["9S", "TS", "JS", "QS", "JH", "JD", "JC", "7C"]),
        false,
      );
      expect(decls).toHaveLength(1);
      expect(decls[0]?.type).toBe("four_of_a_kind");
      expect(decls[0]?.value).toBe(200);
    });

    it("keeps two non-overlapping FoaKs", () => {
      const decls = detectDeclarations(
        hand(["9S", "9H", "9D", "9C", "AS", "AH", "AD", "AC"]),
        false,
      );
      expect(decls).toHaveLength(2);
    });

    it("keeps non-overlapping tierce and FoaK", () => {
      const decls = detectDeclarations(
        hand(["7S", "8S", "9S", "JS", "JH", "JD", "JC", "7C"]),
        false,
      );
      expect(decls).toHaveLength(2);
    });

    it("quarte subsumes tierce in detection — single declaration emitted", () => {
      const decls = detectDeclarations(
        hand(["JD", "QD", "KD", "AD", "7S", "8S", "7C", "8C"]),
        false,
      );
      expect(decls).toHaveLength(1);
      expect(decls[0]?.value).toBe(50);
      expect(decls[0]?.cards).toHaveLength(4);
    });

    it("equal-value clash keeps the carré, not the quinte", () => {
      const decls = detectDeclarations(
        hand(["TS", "JS", "QS", "KS", "AS", "TH", "TD", "TC"]),
        false,
      );
      expect(decls).toHaveLength(1);
      expect(decls[0]?.type).toBe("four_of_a_kind");
      expect(decls[0]?.value).toBe(100);
    });
  });

  describe("Croatian overlap", () => {
    it("keeps a quarte and a carré that share JS — 50 + 200", () => {
      const decls = detectDeclarations(
        hand(["9S", "TS", "JS", "QS", "JH", "JD", "JC", "7C"]),
        true,
      );
      expect(summarize(decls)).toEqual([
        "sequence|50|9S,TS,JS,QS",
        "four_of_a_kind|200|JS,JH,JD,JC",
      ]);
      expect(decls.reduce((acc, d) => acc + d.value, 0)).toBe(250);
    });

    it("keeps a tierce and a carré that share 9S — 20 + 150", () => {
      const decls = detectDeclarations(
        hand(["7S", "8S", "9S", "9D", "9H", "9C", "JD", "QC"]),
        true,
      );
      expect(summarize(decls)).toEqual(["sequence|20|7S,8S,9S", "four_of_a_kind|150|9S,9D,9H,9C"]);
      expect(decls.reduce((acc, d) => acc + d.value, 0)).toBe(170);
    });

    it("keeps both sides of an equal-value clash — 100 + 100", () => {
      const decls = detectDeclarations(
        hand(["TS", "JS", "QS", "KS", "AS", "TH", "TD", "TC"]),
        true,
      );
      expect(summarize(decls)).toEqual([
        "sequence|100|TS,JS,QS,KS,AS",
        "four_of_a_kind|100|TS,TH,TD,TC",
      ]);
      expect(decls.reduce((acc, d) => acc + d.value, 0)).toBe(200);
    });

    it("returns exactly the Bitola result on an overlap-free hand", () => {
      const cards = ["7S", "8S", "9S", "JS", "JH", "JD", "JC", "7C"];
      const withOverlap = detectDeclarations(hand(cards), true);
      const withDedup = detectDeclarations(hand(cards), false);
      expect(withOverlap).toEqual(withDedup);
      expect(summarize(withOverlap)).toEqual([
        "sequence|20|7S,8S,9S",
        "four_of_a_kind|200|JS,JH,JD,JC",
      ]);
    });

    it("still returns nothing when the hand holds no meld", () => {
      expect(detectDeclarations(hand(["7S", "TD", "JH", "KC", "AS"]), true)).toEqual([]);
    });
  });
});
