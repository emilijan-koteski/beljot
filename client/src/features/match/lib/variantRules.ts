import type { Variant } from "@/shared/types/matchTypes";

/**
 * Client mirror of the server's `game.VariantRules` facts the UI needs, derived from the
 * variant string. The server never sends this config over the wire.
 */
export interface VariantRuleFacts {
  readonly declarationOverlap: boolean;
}

const PRESETS: Record<Variant, VariantRuleFacts> = {
  bitola: Object.freeze({ declarationOverlap: false }),
  croatia: Object.freeze({ declarationOverlap: true }),
};

/**
 * Resolves a variant to its rule facts, falling back to Bitola for unknown or missing variants.
 * Uses `Object.hasOwn` so strings like "__proto__" never resolve to prototype members.
 */
export function rulesForVariant(variant: Variant | null | undefined): VariantRuleFacts {
  if (variant !== null && variant !== undefined && Object.hasOwn(PRESETS, variant)) {
    return PRESETS[variant];
  }
  return PRESETS.bitola;
}
