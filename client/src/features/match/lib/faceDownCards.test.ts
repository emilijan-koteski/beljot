import { describe, expect, it } from "vitest";

import type { Card } from "@/shared/types/matchTypes";

import { mergeRevealedFaceDownCards } from "./faceDownCards";

// A Croatian open hand: six cards. The two face-down cards below complete it.
function sixCardHand(): Card[] {
  return [
    { rank: "7", suit: "S" },
    { rank: "8", suit: "S" },
    { rank: "9", suit: "S" },
    { rank: "T", suit: "S" },
    { rank: "7", suit: "H" },
    { rank: "8", suit: "H" },
  ];
}

const ids = (hand: Card[]) => hand.map((c) => `${c.rank}${c.suit}`);

describe("mergeRevealedFaceDownCards", () => {
  it("merges the viewer's own two revealed cards, taking the rendered hand to eight", () => {
    const hand = sixCardHand();
    const merged = mergeRevealedFaceDownCards(hand, 2, 2, ["JS", "QS"]);

    expect(merged).toHaveLength(8);
    expect(ids(merged)).toEqual(["7S", "8S", "9S", "TS", "7H", "8H", "JS", "QS"]);
    // Parsed rank-then-suit — swapping the indices would yield {rank:"S"}.
    expect(merged[6]).toEqual({ rank: "J", suit: "S" });
    expect(merged[7]).toEqual({ rank: "Q", suit: "S" });
    // The input array is not mutated.
    expect(hand).toHaveLength(6);
  });

  it("does not double-add on a duplicated or replayed event", () => {
    // The reconnect path re-sends the reveal, and the merge runs on every
    // render — a card already in the hand must not appear twice.
    const once = mergeRevealedFaceDownCards(sixCardHand(), 0, 0, ["JS", "QS"]);
    const twice = mergeRevealedFaceDownCards(once, 0, 0, ["JS", "QS"]);

    expect(twice).toHaveLength(8);
    expect(ids(twice).filter((id) => id === "JS")).toHaveLength(1);
    expect(ids(twice).filter((id) => id === "QS")).toHaveLength(1);
    // Nothing to add — the same reference comes back, so renders are unaffected.
    expect(twice).toBe(once);
  });

  it("ignores a payload naming a different seat — the last stop before another player's cards", () => {
    const hand = sixCardHand();
    const merged = mergeRevealedFaceDownCards(hand, 2, 3, ["JS", "QS"]);

    expect(merged).toBe(hand);
    expect(merged).toHaveLength(6);
    expect(ids(merged)).not.toContain("JS");
  });

  it("skips malformed ids instead of slicing them into a wrong card", () => {
    const tests: { name: string; ids: string[]; expected: string[] }[] = [
      { name: "too short", ids: ["J"], expected: [] },
      { name: "too long", ids: ["10S"], expected: [] },
      { name: "empty", ids: [""], expected: [] },
      { name: "unknown rank", ids: ["XS"], expected: [] },
      { name: "unknown suit", ids: ["JX"], expected: [] },
      { name: "lowercase", ids: ["js"], expected: [] },
      { name: "one valid, one malformed", ids: ["JS", "10S"], expected: ["JS"] },
    ];

    for (const tc of tests) {
      const merged = mergeRevealedFaceDownCards(sixCardHand(), 1, 1, tc.ids);
      const added = ids(merged).slice(6);
      expect(added, tc.name).toEqual(tc.expected);
    }
  });

  it("returns the hand untouched when there is nothing to merge (the Bitola path)", () => {
    const hand = sixCardHand();

    expect(mergeRevealedFaceDownCards(hand, 0, null, undefined)).toBe(hand);
    expect(mergeRevealedFaceDownCards(hand, 0, 0, undefined)).toBe(hand);
    expect(mergeRevealedFaceDownCards(hand, 0, 0, [])).toBe(hand);
    // Viewer's seat not resolved yet (a race on first mount) — merge nothing
    // rather than guess which seat the payload belongs to.
    expect(mergeRevealedFaceDownCards(hand, null, 0, ["JS", "QS"])).toBe(hand);
  });

  it("merges for seat 0 — a falsy seat index must not be read as 'no seat'", () => {
    const merged = mergeRevealedFaceDownCards(sixCardHand(), 0, 0, ["JS", "QS"]);
    expect(merged).toHaveLength(8);
  });
});
