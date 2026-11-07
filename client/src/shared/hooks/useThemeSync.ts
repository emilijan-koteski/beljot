import { useEffect } from "react";

import { applyTheme } from "@/shared/lib/themePreference";
import { useAuthStore } from "@/shared/stores/authStore";
import { useGuestThemeStore } from "@/shared/stores/themeStore";

/**
 * Keeps <html> on the signed-in player's theme once the session has restored, or on the visitor's own.
 * The account theme is copied to the visitor's, so signing out keeps the page as it was.
 */
export function useThemeSync(): void {
  const isLoading = useAuthStore((s) => s.isLoading);
  const userTheme = useAuthStore((s) => s.user?.themePreference ?? null);
  const guestTheme = useGuestThemeStore((s) => s.theme);
  const syncGuestTheme = useGuestThemeStore((s) => s.sync);

  useEffect(() => {
    if (isLoading) return;
    if (userTheme !== null) syncGuestTheme(userTheme);
    applyTheme(userTheme ?? guestTheme);
  }, [isLoading, userTheme, guestTheme, syncGuestTheme]);
}
