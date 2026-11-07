import { create } from "zustand";

import { applyTheme, markThemePicked, readStoredTheme } from "@/shared/lib/themePreference";
import type { ThemePreference } from "@/shared/types/apiTypes";

interface GuestThemeState {
  theme: ThemePreference;
  pick: (theme: ThemePreference) => void;
  sync: (theme: ThemePreference) => void;
}

/**
 * The theme a signed-out visitor sees: the one they picked, or the last one shown in this browser.
 * `pick` is the visitor's own choice; `sync` follows the signed-in account so sign-out keeps its theme.
 */
export const useGuestThemeStore = create<GuestThemeState>((set) => ({
  theme: readStoredTheme(),
  pick: (theme) => {
    applyTheme(theme);
    markThemePicked(theme);
    set({ theme });
  },
  sync: (theme) => set({ theme }),
}));
