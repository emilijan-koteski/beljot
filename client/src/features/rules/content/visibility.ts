import type { Variant } from "@/shared/types/matchTypes";

import type { VariantScope } from "./types";

/**
 * Whether a scoped thing — a block or a single step item — renders under the
 * given variant tab. No `variant` means shared, so it renders under both.
 */
export function isVisibleFor(scope: VariantScope, variant: Variant): boolean {
  return !scope.variant || scope.variant === variant;
}

/**
 * The subset of blocks / step items one variant's tab renders.
 *
 * ONE definition, imported by the light renderer, the dark renderer and the
 * parity gate, so "what this tab shows" cannot mean three slightly different
 * things. Step numbering is derived from the RESULT of this call, never from the
 * unfiltered array — numbering over the source would render 01, 02, 03, 05 on a
 * tab whose fourth item belongs to the other variant.
 */
export function visibleFor<T extends VariantScope>(items: readonly T[], variant: Variant): T[] {
  return items.filter((item) => isVisibleFor(item, variant));
}
