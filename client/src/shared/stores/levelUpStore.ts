import { create } from "zustand";

/** The data a post-match level-up dialog needs. */
interface LevelUpInfo {
  newLevel: number;
  newTotalXp: number;
  xpEarned: number;
}

/** Holds a pending level-up outside matchStore so the dialog can open after post-match navigation. */
interface LevelUpState {
  pending: LevelUpInfo | null;
  setPending: (info: LevelUpInfo) => void;
  clear: () => void;
}

export const useLevelUpStore = create<LevelUpState>((set) => ({
  pending: null,
  setPending: (pending) => set({ pending }),
  clear: () => set({ pending: null }),
}));
