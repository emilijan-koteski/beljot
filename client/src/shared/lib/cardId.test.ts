import { describe, expect, it } from "vitest";

import { isCardId } from "./cardId";

describe("isCardId", () => {
  it("accepts every one of the 32 real card IDs", () => {
    const ranks = ["7", "8", "9", "T", "J", "Q", "K", "A"];
    const suits = ["S", "H", "D", "C"];
    const accepted: string[] = [];
    for (const r of ranks) {
      for (const s of suits) {
        const id = `${r}${s}`;
        expect(isCardId(id), id).toBe(true);
        accepted.push(id);
      }
    }
    expect(accepted).toHaveLength(32);
  });

  it("rejects anything that is not exactly rank+suit", () => {
    const bad: unknown[] = [
      "",
      "J",
      "S",
      "10S",
      "JSX",
      "js",
      "Js",
      "jS",
      "XS",
      "JX",
      "1S",
      "0S",
      " JS",
      "JS ",
      "SJ",
      undefined,
      null,
      2,
      ["J", "S"],
      { rank: "J", suit: "S" },
    ];
    for (const value of bad) {
      expect(isCardId(value), JSON.stringify(value)).toBe(false);
    }
  });
});
