import { create } from "zustand";

import type { SeasonPointsAwardedPayload } from "@/shared/types/wsEvents";

/**
 * The data a post-match rank-change dialog needs: the direction and the rank the
 * match landed on, taken from event:season_points_awarded. Only a promotion or
 * a demotion is ever stored — a match that kept the rank shows no dialog.
 */
export interface RankChangeInfo {
  direction: "promoted" | "demoted";
  /** Server tier token (normalized at render, like every other tier token). */
  rankTier: string;
  /** 1–3, or null for Master and Grandmaster. */
  rankDivision: number | null;
  /** The signed, applied SP change of the match. */
  spChange: number;
  /** The season total after the match. */
  newSeasonSp: number;
}

interface RankChangeState {
  /**
   * A pending rank change to announce, or null. Set by the
   * event:season_points_awarded handler on a promotion or demotion. It lives
   * OUTSIDE the game-scoped matchStore (which clearGame() wipes on the way back
   * to the lobby/room) so the dialog can open AFTER navigation — the same reason
   * levelUpStore exists. A later match's change REPLACES an unseen one: the
   * dialog always announces the rank the player holds now. Cleared only when
   * the dialog is dismissed.
   */
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
