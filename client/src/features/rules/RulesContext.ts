import { createContext, useContext } from "react";

import type { Variant } from "@/shared/types/matchTypes";

import type { RulesContent } from "./content/types";

/**
 * Localized, render-ready rules content plus the variant tab that is currently
 * active, threaded to every rules sub-component (chapters, card ladders,
 * declarations grid) so a language swap OR a tab flip lands in one place.
 *
 * The active variant rides this existing context rather than a prop chain
 * through `Chapter` -> `RuleBlock`: only the leaf block renderer needs it, and
 * every level in between would otherwise have to forward a prop it never reads.
 */
type RulesContextValue = {
  content: RulesContent;
  variant: Variant;
};

const RulesContext = createContext<RulesContextValue | null>(null);

export const RulesProvider = RulesContext.Provider;

function useRulesContext(): RulesContextValue {
  const ctx = useContext(RulesContext);
  if (!ctx) throw new Error("useRules must be used within <RulesProvider>");
  return ctx;
}

export function useRules(): RulesContent {
  return useRulesContext().content;
}

/** The variant tab whose scoped blocks are rendering right now. */
export function useRulesVariant(): Variant {
  return useRulesContext().variant;
}
