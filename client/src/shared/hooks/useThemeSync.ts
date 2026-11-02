import { useEffect } from "react";

import { applyTheme } from "@/shared/lib/themePreference";
import { useAuthStore } from "@/shared/stores/authStore";

/**
 * Keeps <html> on the signed-in player's theme once the session has restored.
 * A signed-out visitor always gets parchment, which also clears the first-paint mirror.
 */
export function useThemeSync(): void {
  const isLoading = useAuthStore((s) => s.isLoading);
  const theme = useAuthStore((s) => s.user?.themePreference ?? "parchment");

  useEffect(() => {
    if (isLoading) return;
    applyTheme(theme);
  }, [isLoading, theme]);
}
