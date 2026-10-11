import { updatePreferences } from "@/shared/api/profile";
import { normalizeLanguage } from "@/shared/i18n/i18n";
import { useAuthStore } from "@/shared/stores/authStore";

interface AuthResult {
  id: number;
  languagePreference: string;
}

/**
 * Saves the language picked on an auth page to the profile when it differs from the stored one.
 * A failed PATCH is silent and rolls back only the stored preference in the auth store.
 */
export async function reconcileLanguagePreference(
  res: AuthResult,
  currentLanguage: string,
): Promise<void> {
  const picked = normalizeLanguage(currentLanguage);
  if (!picked || picked === res.languagePreference) return;
  try {
    await updatePreferences(res.id, { languagePreference: picked });
    const current = useAuthStore.getState().user;
    if (current?.id === res.id) {
      useAuthStore.getState().setUser({ ...current, languagePreference: picked });
    }
  } catch {
    const current = useAuthStore.getState().user;
    if (current?.id === res.id) {
      useAuthStore.getState().setUser({ ...current, languagePreference: res.languagePreference });
    }
  }
}
