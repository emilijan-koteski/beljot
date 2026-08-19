import type { Variant } from "@/shared/types/matchTypes";

/**
 * Client mirror of the rule facts the UI needs from the server's
 * `game.VariantRules` preset. Only the fields the client actually reads live
 * here; the server config is `json:"-"` by design and is never sent over the
 * wire, so the client re-derives these facts from the variant string.
 *
 * Fields are `readonly` and the presets are frozen, so a consumer cannot
 * mutate the shared preset out from under every later caller.
 */
export interface VariantRuleFacts {
  /**
   * True when one card may count toward more than one declaration. False keeps
   * the one-card-one-group dedup by higher value (four-of-a-kind wins an
   * equal-value clash).
   */
  readonly declarationOverlap: boolean;
}

/**
 * Mirrors `RulesFor` in server/internal/game/types.go. Every preset is fully
 * populated — no field is left to a default.
 */
const PRESETS: Record<Variant, VariantRuleFacts> = {
  bitola: Object.freeze({ declarationOverlap: false }),
  croatia: Object.freeze({ declarationOverlap: true }),
};

/**
 * Resolves a variant to its rule facts. This is the ONLY place on the client
 * that maps a variant name to behaviour — every consumer reads through it, so
 * later variant rules are added here rather than as scattered comparisons.
 *
 * An unrecognised or missing variant falls back to Bitola, matching the Go
 * resolver's explicit fallback (a zero-value config is not Bitola). The lookup
 * goes through `Object.hasOwn` rather than a bare index: the wire type for
 * `variant` is an unconstrained string, so a payload carrying "constructor" or
 * "__proto__" would otherwise resolve to an `Object.prototype` member — truthy,
 * therefore surviving a `??` fallback, and yielding `declarationOverlap:
 * undefined`.
 */
export function rulesForVariant(variant: Variant | null | undefined): VariantRuleFacts {
  if (variant !== null && variant !== undefined && Object.hasOwn(PRESETS, variant)) {
    return PRESETS[variant];
  }
  return PRESETS.bitola;
}
