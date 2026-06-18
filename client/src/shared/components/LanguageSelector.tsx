import { Check, ChevronDown, Globe } from "lucide-react";
import { useTranslation } from "react-i18next";

import { updatePreferences } from "@/shared/api/profile";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/shared/components/ui/dropdown-menu";
import { cn } from "@/shared/lib/utils";
import { useAuthStore } from "@/shared/stores/authStore";

// Order: Latin-script entries sorted ASC by native name (English, Hrvatski,
// Srpski) then Cyrillic-script entries sorted ASC (Македонски). The native
// names in `language.<code>` are immutable, so this ordering is stable.
const languages = [
  { code: "en", labelKey: "language.en" },
  { code: "hr", labelKey: "language.hr" },
  { code: "sr", labelKey: "language.sr" },
  { code: "mk", labelKey: "language.mk" },
] as const;

type LanguageSelectorProps = {
  /**
   * When true, the picked language is also pushed to the server via
   * `updatePreferences` so it persists across devices. The authenticated
   * `AppLayout` enables this; the pre-auth `AuthLayout` leaves it off
   * (localStorage persistence is handled centrally by the
   * `languageChanged` listener in i18n.ts).
   */
  persistToServer?: boolean;
  /**
   * Override the data-testid of the trigger + items. Preserves the
   * pre-existing test ids for `auth-language-selector` and
   * `language-selector` flows without forking the component.
   */
  testIdPrefix?: string;
};

export function LanguageSelector({
  persistToServer = false,
  testIdPrefix = "language",
}: LanguageSelectorProps = {}) {
  const { t, i18n } = useTranslation();

  async function handleLanguageChange(lang: string) {
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
      // Revert the optimistic auth-store update so it stays in sync with the
      // server. The visible UI language stays put (i18n already changed) — a
      // refresh would re-pull the persisted preference and reconcile.
      const current = useAuthStore.getState().user;
      if (current?.id === user.id) {
        useAuthStore.getState().setUser({ ...current, languagePreference: previousPreference });
      }
    }
  }

  const currentLang = i18n.language.slice(0, 2).toUpperCase();
  const triggerTestId = `${testIdPrefix}-selector`;
  const optionTestId = (code: string) => `${testIdPrefix}-option-${code}`;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className="text-ink-dim hover:bg-surface-sunken hover:text-ink aria-expanded:bg-surface-sunken aria-expanded:text-ink inline-flex h-8 w-8 items-center justify-center rounded-lg border border-border bg-transparent transition-colors md:w-auto md:justify-start md:gap-1.5 md:px-2.5 md:text-sm md:font-medium"
        aria-label={t("auth.languageSelector.label")}
        data-testid={triggerTestId}
      >
        {/* Phones (<md): icon-only globe button, matching the hamburger menu —
            the language code + chevron only appear from md up. */}
        <Globe className="size-4.5 md:size-3.5" />
        <span className="text-ink hidden tracking-wider md:inline">{currentLang}</span>
        <ChevronDown className="hidden size-3 opacity-70 md:block" />
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        className="bg-surface-elevated min-w-40 border border-border p-1 shadow-[0_14px_36px_-18px_rgba(14,58,36,0.30)]"
      >
        {languages.map((lang) => {
          const active = lang.code === i18n.language;
          return (
            <DropdownMenuItem
              key={lang.code}
              onClick={() => handleLanguageChange(lang.code)}
              data-testid={optionTestId(lang.code)}
              className={cn(
                "flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-sm transition-colors",
                active
                  ? "bg-accent-soft text-ink font-semibold"
                  : "text-ink hover:bg-surface-sunken font-medium",
              )}
            >
              <span className="text-ink-mute w-4 font-mono text-[10px] font-bold tracking-[1px]">
                {lang.code.toUpperCase()}
              </span>
              <span className="flex-1 text-left">{t(lang.labelKey)}</span>
              {active && <Check className="text-accent size-3.5" strokeWidth={2.4} />}
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
