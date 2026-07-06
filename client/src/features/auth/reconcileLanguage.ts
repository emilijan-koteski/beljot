import { updatePreferences } from "@/shared/api/profile";
import { normalizeLanguage } from "@/shared/i18n/i18n";
import { useAuthStore } from "@/shared/stores/authStore";

interface AuthResult {
  id: number;
  languagePreference: string;
}

/**
 * Post-auth language reconciliation, shared by the password and SSO success
 * paths on both auth pages: if the visitor picked a different language on the
 * auth page than the one stored on their profile, push the picked language to
 * the server and reconcile the auth store. Mirrors LanguageSelector's
 * optimistic-with-rollback pattern; the UI language stays as picked either
 * way, and a PATCH failure is silent (rolls back only the stored preference).
 *
 * currentLanguage is i18n.language — normalized to the short code here so
 * region-tagged values like "en-US" don't trigger a futile PATCH the server
 * would reject.
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
