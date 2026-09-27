import "@/shared/i18n/i18n";

import { afterEach, describe, expect, it } from "vitest";

import { i18n } from "@/shared/i18n/i18n";

import {
  formatSpChange,
  normalizeSeasonDivision,
  normalizeSeasonTier,
  SEASON_TIER_COLOR,
  SEASON_TIER_FLOORS,
  SEASON_TIER_LINE,
  SEASON_TIER_SOFT,
  SEASON_TIERS,
  seasonBarFill,
  seasonDaysRemaining,
  seasonRankLabel,
  seasonSpOrZero,
  seasonTierForSp,
  seasonTierHasDivisions,
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

  it("mirrors the server floors", () => {
    expect(SEASON_TIER_FLOORS.map(([, floor]) => floor)).toEqual([
      0, 150, 300, 600, 800, 1000, 1200, 1400,
    ]);
  });
});

describe("seasonTierForSp", () => {
  it.each([
    [0, "iron"],
    [75, "iron"],
    [149, "iron"],
    [150, "bronze"],
    [299, "bronze"],
    [300, "silver"],
    [599, "silver"],
    [600, "gold"],
    [799, "gold"],
    [800, "platinum"],
    [999, "platinum"],
    [1000, "diamond"],
    [1199, "diamond"],
    [1200, "master"],
    [1399, "master"],
    [1400, "grandmaster"],
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
    expect(normalizeSeasonTier("diamond", 1100)).toBe("diamond");
  });

  it("falls back to the SP bucket for an unknown token", () => {
    // The version-skew case: a server that grows a ninth tier must not make a
    // stale bundle render a missing i18n key.
    expect(normalizeSeasonTier("mythic", 1100)).toBe("diamond");
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

describe("seasonTierHasDivisions", () => {
  it("splits Iron through Diamond, and not Master or Grandmaster", () => {
    expect(SEASON_TIERS.filter(seasonTierHasDivisions)).toEqual([
      "iron",
      "bronze",
      "silver",
      "gold",
      "platinum",
      "diamond",
    ]);
  });
});

describe("normalizeSeasonDivision", () => {
  it.each([1, 2, 3])("keeps division %i on a divided tier", (d) => {
    expect(normalizeSeasonDivision("gold", d)).toBe(d);
  });

  it.each([[0], [4], [-1], [1.5], [NaN], [null], [undefined]])(
    "drops an out-of-range or absent division (%s)",
    (d) => {
      expect(normalizeSeasonDivision("gold", d as number | null | undefined)).toBeNull();
    },
  );

  it("drops any division on a single tier", () => {
    expect(normalizeSeasonDivision("master", 2)).toBeNull();
    expect(normalizeSeasonDivision("grandmaster", 1)).toBeNull();
  });
});

describe("seasonRankLabel", () => {
  afterEach(async () => {
    await i18n.changeLanguage("en");
  });

  it("reads tier plus division when there is one", () => {
    expect(seasonRankLabel(i18n.t, "gold", 2)).toBe("Gold 2");
  });

  it("reads the bare tier name without a division", () => {
    expect(seasonRankLabel(i18n.t, "master", null)).toBe("Master");
    // An ended pre-division season: stored tier, no division.
    expect(seasonRankLabel(i18n.t, "gold", null)).toBe("Gold");
  });

  it("localizes the tier name, keeping the numeral", async () => {
    await i18n.changeLanguage("mk");
    expect(seasonRankLabel(i18n.t, "gold", 2)).toBe("Злато 2");
    await i18n.changeLanguage("hr");
    expect(seasonRankLabel(i18n.t, "silver", 3)).toBe("Srebro 3");
  });
});

describe("formatSpChange", () => {
  it("signs a gain", () => {
    expect(formatSpChange(24)).toBe("+24");
  });

  it("signs a loss with U+2212, never a hyphen, and never clamps it", () => {
    expect(formatSpChange(-13)).toBe("\u221213");
    expect(formatSpChange(-120)).toBe("\u2212120");
  });

  it("renders no change as a bare 0", () => {
    expect(formatSpChange(0)).toBe("0");
  });

  it.each([[NaN], [Infinity], [undefined]])("renders a non-finite value (%s) as 0", (v) => {
    expect(formatSpChange(v as number)).toBe("0");
  });
});
