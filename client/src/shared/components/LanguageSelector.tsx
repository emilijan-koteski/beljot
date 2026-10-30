import { Check, ChevronDown, Globe } from "lucide-react";
import { useTranslation } from "react-i18next";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/shared/components/ui/dropdown-menu";
import { LANGUAGE_OPTIONS, useChangeLanguage } from "@/shared/hooks/useChangeLanguage";
import { cn } from "@/shared/lib/utils";

type LanguageSelectorProps = {
  persistToServer?: boolean;
  testIdPrefix?: string;
};

export function LanguageSelector({
  persistToServer = false,
  testIdPrefix = "language",
}: LanguageSelectorProps = {}) {
  const { t, i18n } = useTranslation();

  const handleLanguageChange = useChangeLanguage(persistToServer);

  const currentLang = i18n.language.slice(0, 2).toUpperCase();
  const triggerTestId = `${testIdPrefix}-selector`;
  const optionTestId = (code: string) => `${testIdPrefix}-option-${code}`;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className="text-ink-dim hover:bg-surface-sunken hover:text-ink aria-expanded:bg-surface-sunken aria-expanded:text-ink inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-border bg-transparent transition-colors md:w-auto md:justify-start md:gap-1.5 md:px-2.5 md:text-sm md:font-medium"
        aria-label={t("auth.languageSelector.label")}
        data-testid={triggerTestId}
      >
        <Globe className="size-5 md:size-3.5" />
        <span className="text-ink hidden tracking-wider md:inline">{currentLang}</span>
        <ChevronDown className="hidden size-3 opacity-70 md:block" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-40">
        {LANGUAGE_OPTIONS.map((lang) => {
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
              <span className="text-ink-mute w-4 font-mono text-2xs font-bold tracking-widest">
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
