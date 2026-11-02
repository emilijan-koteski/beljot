import { updatePreferences } from "@/shared/api/profile";
import { useAuthStore } from "@/shared/stores/authStore";
import type { ThemePreference } from "@/shared/types/apiTypes";

export const THEME_STORAGE_KEY = "beljot.theme";
export const BRASS_MODE_CLASS = "brass-mode";

let latestRequest = 0;

/**
 * Puts the theme on <html>, so portalled dialogs and toasts follow it too.
 * It is mirrored to localStorage so index.html can paint it before the session restores.
 */
export function applyTheme(theme: ThemePreference): void {
  document.documentElement.classList.toggle(BRASS_MODE_CLASS, theme === "brass");
  try {
    if (theme === "brass") {
      window.localStorage.setItem(THEME_STORAGE_KEY, theme);
    } else {
      window.localStorage.removeItem(THEME_STORAGE_KEY);
    }
  } catch {
    return;
  }
}

/**
 * Optimistically writes the theme to the auth store, then persists it; a failure
 * silently restores `previous` unless a later toggle superseded it.
 */
export async function persistTheme(
  next: ThemePreference,
  previous: ThemePreference,
): Promise<void> {
  const user = useAuthStore.getState().user;
  if (!user) return;

  const request = ++latestRequest;
  useAuthStore.getState().setUser({ ...user, themePreference: next });

  try {
    await updatePreferences(user.id, { themePreference: next });
  } catch {
    if (request !== latestRequest) return;
    const current = useAuthStore.getState().user;
    if (current?.id === user.id) {
      useAuthStore.getState().setUser({ ...current, themePreference: previous });
    }
  }
}

/** Test seam: reset the in-flight sequence between cases. */
export function resetThemeRequestSequence(): void {
  latestRequest = 0;
}
