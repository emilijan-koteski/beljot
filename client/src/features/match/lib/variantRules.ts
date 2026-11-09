import type { Variant } from "@/shared/types/matchTypes";

/**
 * Per-variant facts the UI needs: mirrors of the server's `game.VariantRules` plus UI-only naming,
 * derived from the variant string because the server never sends this config over the wire.
 */
interface VariantRuleFacts {
  readonly declarationOverlap: boolean;
  readonly declarationWindow: boolean;
  readonly kingIsRebelote: boolean;
}

const PRESETS: Record<Variant, VariantRuleFacts> = {
  bitola: Object.freeze({
    declarationOverlap: false,
    declarationWindow: true,
    kingIsRebelote: true,
  }),
  croatia: Object.freeze({
    declarationOverlap: true,
    declarationWindow: false,
    kingIsRebelote: false,
  }),
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
