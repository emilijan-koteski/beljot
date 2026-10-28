import { createContext, useContext } from "react";

import type { Variant } from "@/shared/types/matchTypes";

import type { RulesContent } from "./content/types";

/**
 * Localized rules content plus the active variant tab, shared by every rules sub-component.
 */
type RulesContextValue = {
  content: RulesContent;
  variant: Variant;
  dense?: boolean;
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

/** True inside the in-match rules dialog, which sets the same chapters a size smaller. */
export function useRulesDense(): boolean {
  return useRulesContext().dense === true;
}
