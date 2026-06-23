import { describe, expect, it } from "vitest";

import { xpBarFill, xpFraction, xpThreshold } from "./xpLevel";

// These boundaries MUST match the server curve (server/internal/user/level.go,
// 50·N²). If the server constant changes, this test should change in the same
// commit — it is the client half of the manual-sync contract.
describe("xpLevel curve (mirrors server 50·N²)", () => {
  it("computes thresholds as 50·N²", () => {
    expect(xpThreshold(0)).toBe(0);
    expect(xpThreshold(1)).toBe(50);
    expect(xpThreshold(2)).toBe(200);
    expect(xpThreshold(3)).toBe(450);
    expect(xpThreshold(4)).toBe(800);
    expect(xpThreshold(5)).toBe(1250);
  });

  it("decomposes a total into within-level progress at the server-given level", () => {
    // Level 3 band: 450 .. 800 (span 350); 600 total → 150 into the band.
    expect(xpBarFill(600, 3)).toEqual({
      xpIntoLevel: 150,
      xpForNextLevel: 350,
      fraction: 150 / 350,
    });
  });

  it("returns a zero fraction at a level threshold and full-ish near the next", () => {
    expect(xpBarFill(450, 3).fraction).toBe(0);
    expect(xpBarFill(0, 0)).toEqual({ xpIntoLevel: 0, xpForNextLevel: 50, fraction: 0 });
  });

  it("clamps the bar fraction to [0, 1]", () => {
    expect(xpFraction(0, 0)).toBe(0); // guard against divide-by-zero
    expect(xpFraction(-10, 350)).toBe(0);
    expect(xpFraction(400, 350)).toBe(1);
    expect(xpFraction(175, 350)).toBe(0.5);
  });
});
