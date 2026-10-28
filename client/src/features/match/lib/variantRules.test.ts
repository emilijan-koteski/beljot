import { describe, expect, it } from "vitest";

import { rulesForVariant } from "./variantRules";

describe("rulesForVariant", () => {
  it("returns each variant's facts", () => {
    expect(rulesForVariant("bitola")).toEqual({ declarationOverlap: false, kingIsRebelote: true });
    expect(rulesForVariant("croatia")).toEqual({ declarationOverlap: true, kingIsRebelote: false });
  });

  it("falls back to Bitola for an unknown or missing variant", () => {
    expect(rulesForVariant(null)).toEqual(rulesForVariant("bitola"));
    expect(rulesForVariant(undefined)).toEqual(rulesForVariant("bitola"));
    expect(rulesForVariant("belgian" as never)).toEqual(rulesForVariant("bitola"));
  });

  it("falls back to Bitola for Object.prototype keys", () => {
    for (const key of ["constructor", "__proto__", "toString", "hasOwnProperty"]) {
      const facts = rulesForVariant(key as never);
      expect(facts).toEqual(rulesForVariant("bitola"));
      expect(facts.declarationOverlap).toBe(false);
    }
  });

  it("hands out a preset that cannot be mutated", () => {
    const facts = rulesForVariant("bitola");
    expect(() => {
      (facts as { declarationOverlap: boolean }).declarationOverlap = true;
    }).toThrow();
    expect(rulesForVariant("bitola").declarationOverlap).toBe(false);
  });
});
