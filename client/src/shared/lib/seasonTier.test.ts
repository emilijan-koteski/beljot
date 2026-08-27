import { describe, expect, it } from "vitest";

import {
  normalizeSeasonTier,
  SEASON_TIER_COLOR,
  SEASON_TIER_FLOORS,
  SEASON_TIER_LINE,
  SEASON_TIER_SOFT,
  SEASON_TIERS,
  seasonBarFill,
  seasonDaysRemaining,
  seasonSpOrZero,
  seasonTierForSp,
} from "./seasonTier";

describe("SEASON_TIERS", () => {
  it("lists the eight tokens the server emits, ascending", () => {
    // Literals, not derived from SEASON_TIER_FLOORS: re-deriving the expectation
    // from the same table the implementation reads would pass for any table.
    expect([...SEASON_TIERS]).toEqual([
      "iron",
      "bronze",
      "silver",
      "gold",
      "platinum",
      "diamond",
      "master",
      "grandmaster",
    ]);
  });

  it("mirrors the server thresholds", () => {
    expect(SEASON_TIER_FLOORS.map(([, floor]) => floor)).toEqual([
      0, 500, 1500, 3000, 5500, 8500, 12500, 18000,
    ]);
  });
});

describe("seasonTierForSp", () => {
  it.each([
    [0, "iron"],
    [250, "iron"],
    [499, "iron"],
    [500, "bronze"],
    [1499, "bronze"],
    [1500, "silver"],
    [2999, "silver"],
    [3000, "gold"],
    [5499, "gold"],
    [5500, "platinum"],
    [8499, "platinum"],
    [8500, "diamond"],
    [12499, "diamond"],
    [12500, "master"],
    [17999, "master"],
    [18000, "grandmaster"],
    [250000, "grandmaster"],
  ])("buckets %i SP as %s", (sp, tier) => {
    expect(seasonTierForSp(sp)).toBe(tier);
  });

  it("treats zero as Iron rather than as a missing value", () => {
    expect(seasonTierForSp(0)).toBe("iron");
  });

  it("clamps a negative total to Iron", () => {
    expect(seasonTierForSp(-1)).toBe("iron");
  });
});

describe("seasonSpOrZero", () => {
  it("keeps a real zero", () => {
    expect(seasonSpOrZero(0)).toBe(0);
  });

  it("keeps a real total", () => {
    expect(seasonSpOrZero(4200)).toBe(4200);
  });

  it.each([[undefined], [null], [NaN], [Infinity]])("coerces %s to zero", (value) => {
    expect(seasonSpOrZero(value as number | null | undefined)).toBe(0);
  });

  it("clamps a negative total", () => {
    expect(seasonSpOrZero(-30)).toBe(0);
  });
});

describe("normalizeSeasonTier", () => {
  it("passes a known token through", () => {
    expect(normalizeSeasonTier("diamond", 9000)).toBe("diamond");
  });

  it("falls back to the SP bucket for an unknown token", () => {
    // The version-skew case: a server that grows a ninth tier must not make a
    // stale bundle render a missing i18n key.
    expect(normalizeSeasonTier("mythic", 9000)).toBe("diamond");
  });

  it("falls back to iron for an unknown token at zero SP", () => {
    expect(normalizeSeasonTier("", 0)).toBe("iron");
  });
});

describe("seasonBarFill", () => {
  it("is empty at a tier floor", () => {
    expect(seasonBarFill(0, 500)).toBe(0);
  });

  it("is half way through a band", () => {
    expect(seasonBarFill(250, 500)).toBe(0.5);
  });

  it("is FULL at Grandmaster, where there is no next tier", () => {
    // spForNextTier 0 is the terminal case. An empty bar at the top of the
    // ladder would be the exact opposite of the truth.
    expect(seasonBarFill(2000, 0)).toBe(1);
  });

  it("clamps above one", () => {
    expect(seasonBarFill(900, 500)).toBe(1);
  });

  it("clamps below zero", () => {
    expect(seasonBarFill(-100, 500)).toBe(0);
  });

  it("survives absent values", () => {
    expect(seasonBarFill(undefined as unknown as number, 500)).toBe(0);
    expect(seasonBarFill(100, undefined as unknown as number)).toBe(1);
  });
});

describe("colour maps", () => {
  it("covers every tier in all three maps", () => {
    for (const tier of SEASON_TIERS) {
      expect(SEASON_TIER_COLOR[tier]).toMatch(/^var\(--/);
      expect(SEASON_TIER_SOFT[tier]).toMatch(/^var\(--/);
      expect(SEASON_TIER_LINE[tier]).toMatch(/^var\(--/);
    }
  });

  it("gives every tier a distinct colour token", () => {
    const values = SEASON_TIERS.map((t) => SEASON_TIER_COLOR[t]);
    expect(new Set(values).size).toBe(SEASON_TIERS.length);
  });
});

describe("seasonDaysRemaining", () => {
  const now = Date.parse("2026-08-27T12:00:00Z");

  it("rounds a partial day up so a live season never reads as over", () => {
    expect(seasonDaysRemaining("2026-08-27T18:00:00Z", now)).toBe(1);
  });

  it("counts whole days", () => {
    expect(seasonDaysRemaining("2026-09-06T12:00:00Z", now)).toBe(10);
  });

  it("is zero once the window has closed", () => {
    expect(seasonDaysRemaining("2026-08-01T12:00:00Z", now)).toBe(0);
  });

  it("is zero exactly at the boundary", () => {
    expect(seasonDaysRemaining("2026-08-27T12:00:00Z", now)).toBe(0);
  });

  it.each([[undefined], [null], [""], ["not-a-date"]])("is zero for %s", (value) => {
    expect(seasonDaysRemaining(value as string | null | undefined, now)).toBe(0);
  });
});
