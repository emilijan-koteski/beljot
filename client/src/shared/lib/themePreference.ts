import { updatePreferences } from "@/shared/api/profile";
import { useAuthStore } from "@/shared/stores/authStore";
import type { ThemePreference } from "@/shared/types/apiTypes";

export const THEME_STORAGE_KEY = "beljot.theme";
export const BRASS_MODE_CLASS = "brass-mode";
export const THEME_PICKED_KEY = "beljot.theme.picked";

let latestRequest = 0;

/** The theme for anyone who has not chosen one. */
export const DEFAULT_THEME: ThemePreference = "brass";

/**
 * Puts the theme on <html>, so portalled dialogs and toasts follow it too.
 * It is mirrored to localStorage so index.html can paint it before the session restores.
 */
export function applyTheme(theme: ThemePreference): void {
  document.documentElement.classList.toggle(BRASS_MODE_CLASS, theme === "brass");
  try {
    window.localStorage.setItem(THEME_STORAGE_KEY, theme);
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

/** The theme last shown in this browser, or the default on a first visit. */
export function readStoredTheme(): ThemePreference {
  try {
    const stored = window.localStorage.getItem(THEME_STORAGE_KEY);
    return stored === "parchment" || stored === "brass" ? stored : DEFAULT_THEME;
  } catch {
    return DEFAULT_THEME;
  }
}

/** Remembers a theme a signed-out visitor chose, so signing in can save it to the account. */
export function markThemePicked(theme: ThemePreference): void {
  try {
    window.sessionStorage.setItem(THEME_PICKED_KEY, theme);
  } catch {
    return;
  }
}

function takeThemePick(): ThemePreference | null {
  try {
    const picked = window.sessionStorage.getItem(THEME_PICKED_KEY);
    window.sessionStorage.removeItem(THEME_PICKED_KEY);
    return picked === "brass" || picked === "parchment" ? picked : null;
  } catch {
    return null;
  }
}

function setUserTheme(id: number, theme: ThemePreference): void {
  const current = useAuthStore.getState().user;
  if (current?.id === id) {
    useAuthStore.getState().setUser({ ...current, themePreference: theme });
  }
}

interface ThemeAuthResult {
  id: number;
  themePreference: ThemePreference;
}

/**
 * Saves the theme a visitor picked before signing in when it differs from the account's.
 * The auth store updates first so the page keeps the picked theme; a failed PATCH restores the account's.
 */
export async function reconcileThemePreference(res: ThemeAuthResult): Promise<void> {
  const picked = takeThemePick();
  if (picked === null || picked === res.themePreference) return;
  setUserTheme(res.id, picked);
  try {
    await updatePreferences(res.id, { themePreference: picked });
  } catch {
    setUserTheme(res.id, res.themePreference);
  }
}

/** Test seam: reset the in-flight sequence between cases. */
export function resetThemeRequestSequence(): void {
  latestRequest = 0;
}
