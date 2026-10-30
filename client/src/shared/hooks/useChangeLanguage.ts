import { useTranslation } from "react-i18next";

import { updatePreferences } from "@/shared/api/profile";
import { useAuthStore } from "@/shared/stores/authStore";

export const LANGUAGE_OPTIONS = [
  { code: "en", labelKey: "language.en" },
  { code: "hr", labelKey: "language.hr" },
  { code: "sr", labelKey: "language.sr" },
  { code: "mk", labelKey: "language.mk" },
] as const;

export type LanguageCode = (typeof LANGUAGE_OPTIONS)[number]["code"];

/**
 * Switches the UI language and, when asked to, saves it as the signed-in player's preference.
 * The saved preference is updated optimistically and reverted if the server rejects it.
 */
export function useChangeLanguage(persistToServer: boolean) {
  const { i18n } = useTranslation();

  return async function changeLanguage(lang: string) {
    if (lang === i18n.language) return;
    await i18n.changeLanguage(lang);

    if (!persistToServer) return;

    const user = useAuthStore.getState().user;
    if (!user) return;

    const previousPreference = user.languagePreference;
    useAuthStore.getState().setUser({ ...user, languagePreference: lang });

    try {
      await updatePreferences(user.id, { languagePreference: lang });
    } catch {
      const current = useAuthStore.getState().user;
      if (current?.id === user.id) {
        useAuthStore.getState().setUser({ ...current, languagePreference: previousPreference });
      }
    }
  };
}
