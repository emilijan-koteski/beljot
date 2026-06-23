import { create } from "zustand";

/** The data a post-match level-up dialog needs (Story 9.5 follow-up). */
export interface LevelUpInfo {
  /** The level the player just reached (server-authoritative). */
  newLevel: number;
  /** Lifetime XP after the match — anchors the dialog's progress-bar band. */
  newTotalXp: number;
  /** XP earned in the match that triggered the level-up. */
  xpEarned: number;
}

interface LevelUpState {
  /**
   * A pending level-up to celebrate, or null. Set by the event:xp_awarded
   * handler ONLY when the award crossed a level boundary. It deliberately lives
   * outside the game-scoped matchStore (which clearGame() wipes on the way back
   * to the lobby/room) so the dialog can open AFTER navigation. Cleared only
   * when the dialog is dismissed.
   */
  pending: LevelUpInfo | null;
  setPending: (info: LevelUpInfo) => void;
  clear: () => void;
}

export const useLevelUpStore = create<LevelUpState>((set) => ({
  pending: null,
  setPending: (pending) => set({ pending }),
  clear: () => set({ pending: null }),
}));
