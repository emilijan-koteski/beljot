import { describe, expect, it } from "vitest";

import { rulesForVariant } from "./variantRules";

describe("rulesForVariant", () => {
  it("mirrors the Go presets", () => {
    expect(rulesForVariant("bitola")).toEqual({ declarationOverlap: false });
    expect(rulesForVariant("croatia")).toEqual({ declarationOverlap: true });
  });

  it("falls back to Bitola for an unknown or missing variant", () => {
    expect(rulesForVariant(null)).toEqual(rulesForVariant("bitola"));
    expect(rulesForVariant(undefined)).toEqual(rulesForVariant("bitola"));
    // A variant string the client does not know yet — e.g. an older client
    // against a newer server — must not land on a zero-value config.
    expect(rulesForVariant("belgian" as never)).toEqual(rulesForVariant("bitola"));
  });

  it("falls back to Bitola for Object.prototype keys", () => {
    // The wire type for variant is an unconstrained string. A bare index would
    // resolve these to prototype members — truthy, so a `??` fallback would not
    // catch them — leaving declarationOverlap undefined.
    for (const key of ["constructor", "__proto__", "toString", "hasOwnProperty"]) {
      const facts = rulesForVariant(key as never);
      expect(facts).toEqual({ declarationOverlap: false });
      expect(facts.declarationOverlap).toBe(false);
    }
  });

  it("hands out a preset that cannot be mutated", () => {
    const facts = rulesForVariant("bitola");
    expect(() => {
      (facts as { declarationOverlap: boolean }).declarationOverlap = true;
    }).toThrow();
    // The next caller still sees the real preset.
    expect(rulesForVariant("bitola").declarationOverlap).toBe(false);
  });
});
