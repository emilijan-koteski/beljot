import { describe, expect, it } from "vitest";

import {
  HONOR_PRIOR_SCORE,
  HONOR_TIERS,
  honorBarFill,
  honorCountOrZero,
  honorIsNewPlayer,
  honorScoreOrPrior,
  honorTierForScore,
  normalizeHonorTier,
  normalizeHonorTrendDirection,
} from "./honor";

describe("honorTierForScore", () => {
  it("buckets every band edge the same way the server does", () => {
    const cases: Array<[number, string]> = [
      [100, "exemplary"],
      [95, "exemplary"],
      [94, "trusted"],
      [85, "trusted"],
      [84, "fair"],
      [80, "fair"],
      [70, "fair"],
      [69, "unreliable"],
      [50, "unreliable"],
      [49, "problematic"],
      [0, "problematic"],
    ];
    for (const [score, tier] of cases) {
      expect(honorTierForScore(score)).toBe(tier);
    }
  });

  it("clamps out-of-range scores to the nearest band", () => {
    expect(honorTierForScore(150)).toBe("exemplary");
    expect(honorTierForScore(-20)).toBe("problematic");
  });

  it("places the no-history prior in the fair band", () => {
    expect(honorTierForScore(HONOR_PRIOR_SCORE)).toBe("fair");
  });

  it("falls back to the prior's band for an absent score, not the worst band", () => {
    // Regression: an unguarded undefined produced NaN, every floor comparison
    // went false, and the loop fell through to "problematic" — a blank number in
    // danger red. A missing score must read as the neutral prior instead.
    expect(honorTierForScore(undefined as unknown as number)).toBe("fair");
    expect(honorTierForScore(NaN)).toBe("fair");
  });
});

describe("honorScoreOrPrior", () => {
  it("passes real scores through, including a genuine zero", () => {
    // 0 is a REAL score from Go ("problematic"), not a missing value. A
    // truthiness check here would silently rewrite it to 80.
    expect(honorScoreOrPrior(0)).toBe(0);
    expect(honorScoreOrPrior(1)).toBe(1);
    expect(honorScoreOrPrior(96)).toBe(96);
  });

  it("substitutes the prior for anything not finite", () => {
    expect(honorScoreOrPrior(undefined)).toBe(HONOR_PRIOR_SCORE);
    expect(honorScoreOrPrior(null)).toBe(HONOR_PRIOR_SCORE);
    expect(honorScoreOrPrior(NaN)).toBe(HONOR_PRIOR_SCORE);
    expect(honorScoreOrPrior(Infinity)).toBe(HONOR_PRIOR_SCORE);
  });
});

describe("honorCountOrZero", () => {
  it("passes real counts through, including zero and negatives", () => {
    expect(honorCountOrZero(0)).toBe(0);
    expect(honorCountOrZero(41)).toBe(41);
    // A trend delta is legitimately signed.
    expect(honorCountOrZero(-3)).toBe(-3);
  });

  it("substitutes 0 for anything not finite", () => {
    expect(honorCountOrZero(undefined)).toBe(0);
    expect(honorCountOrZero(null)).toBe(0);
    expect(honorCountOrZero(NaN)).toBe(0);
  });
});

describe("honorIsNewPlayer", () => {
  it("passes a real boolean through, including false", () => {
    expect(honorIsNewPlayer(false)).toBe(false);
    expect(honorIsNewPlayer(true)).toBe(true);
  });

  it("defaults to SUPPRESSED when absent", () => {
    // Unguarded, `undefined` was falsy and took the numeric branch, so a server
    // without the honor fields showed a confident 80/"Fair" for every account —
    // including the newcomers the flag exists to suppress. Hiding is the
    // conservative default when we cannot tell.
    expect(honorIsNewPlayer(undefined)).toBe(true);
    expect(honorIsNewPlayer(null)).toBe(true);
  });
});

describe("normalizeHonorTier", () => {
  it("passes every known token through unchanged", () => {
    for (const tier of HONOR_TIERS) {
      expect(normalizeHonorTier(tier, 50)).toBe(tier);
    }
  });

  it("falls back to the score's own band for an unknown token", () => {
    // Version skew: a newer server ships a tier this bundle has never heard of.
    // It must colour by score rather than render a missing i18n key.
    expect(normalizeHonorTier("legendary", 97)).toBe("exemplary");
    expect(normalizeHonorTier("", 30)).toBe("problematic");
  });
});

describe("honorBarFill", () => {
  it("maps the 0-100 scale onto a [0,1] fraction", () => {
    expect(honorBarFill(0)).toBe(0);
    expect(honorBarFill(50)).toBe(0.5);
    expect(honorBarFill(100)).toBe(1);
  });

  it("clamps out-of-range scores", () => {
    expect(honorBarFill(140)).toBe(1);
    expect(honorBarFill(-10)).toBe(0);
  });

  it("renders the prior's fill rather than NaN% for an absent score", () => {
    expect(honorBarFill(undefined as unknown as number)).toBe(HONOR_PRIOR_SCORE / 100);
  });
});

describe("normalizeHonorTrendDirection", () => {
  it("passes known directions through", () => {
    expect(normalizeHonorTrendDirection("up")).toBe("up");
    expect(normalizeHonorTrendDirection("flat")).toBe("flat");
    expect(normalizeHonorTrendDirection("down")).toBe("down");
  });

  it("defaults an unknown direction to flat", () => {
    expect(normalizeHonorTrendDirection("sideways")).toBe("flat");
  });
});
