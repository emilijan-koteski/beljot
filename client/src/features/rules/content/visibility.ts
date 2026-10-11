import type { Variant } from "@/shared/types/matchTypes";

import type { VariantScope } from "./types";

/** Whether a block or step item renders under the given variant tab; no `variant` means both. */
export function isVisibleFor(scope: VariantScope, variant: Variant): boolean {
  return !scope.variant || scope.variant === variant;
}

/**
 * The subset of blocks or step items one variant's tab renders.
 * Number steps from this result, never from the unfiltered array.
 */
export function visibleFor<T extends VariantScope>(items: readonly T[], variant: Variant): T[] {
  return items.filter((item) => isVisibleFor(item, variant));
}
