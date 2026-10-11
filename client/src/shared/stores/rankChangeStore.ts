import { create } from "zustand";

import type { SeasonPointsAwardedPayload } from "@/shared/types/wsEvents";

/** The data a post-match rank-change dialog needs; only a promotion or demotion is stored. */
export interface RankChangeInfo {
  direction: "promoted" | "demoted";
  rankTier: string;
  rankDivision: number | null;
  spChange: number;
  newSeasonSp: number;
}

/**
 * Holds a pending rank change outside matchStore so the dialog can open after
 * post-match navigation. A later change replaces an unseen one.
 */
interface RankChangeState {
  pending: RankChangeInfo | null;
  setPending: (info: RankChangeInfo) => void;
  clear: () => void;
}

export const useRankChangeStore = create<RankChangeState>((set) => ({
  pending: null,
  setPending: (pending) => set({ pending }),
  clear: () => set({ pending: null }),
}));

/** Builds the pending info from a validated payload, or null for "none". */
export function rankChangeFromPayload(p: SeasonPointsAwardedPayload): RankChangeInfo | null {
  if (p.rankChange !== "promoted" && p.rankChange !== "demoted") return null;
  return {
    direction: p.rankChange,
    rankTier: p.rankTier,
    rankDivision: p.rankDivision,
    spChange: p.spChange,
    newSeasonSp: p.newSeasonSp,
  };
}
