import { useAuthStore } from "@/shared/stores/authStore";
import { useGuestThemeStore } from "@/shared/stores/themeStore";
import type { ThemePreference } from "@/shared/types/apiTypes";

/** The theme on screen: the signed-in player's, or the visitor's own. */
export function useActiveTheme(): ThemePreference {
  const userTheme = useAuthStore((s) => s.user?.themePreference ?? null);
  const guestTheme = useGuestThemeStore((s) => s.theme);
  return userTheme ?? guestTheme;
}
