import { Moon, Sun } from "lucide-react";
import { useTranslation } from "react-i18next";

import { useActiveTheme } from "@/shared/hooks/useActiveTheme";
import { persistTheme } from "@/shared/lib/themePreference";
import { cn } from "@/shared/lib/utils";
import { useAuthStore } from "@/shared/stores/authStore";
import { useGuestThemeStore } from "@/shared/stores/themeStore";
import type { ThemePreference } from "@/shared/types/apiTypes";

type ThemeToggleProps = {
  testId?: string;
  className?: string;
};

/**
 * Switches between parchment and brass mode, styled like the language selector beside it.
 * A signed-in player's choice is saved to the account; a visitor's stays in this browser until they sign in.
 */
export function ThemeToggle({ testId = "theme-toggle", className }: ThemeToggleProps) {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const pickGuestTheme = useGuestThemeStore((s) => s.pick);
  const theme = useActiveTheme();
  const brass = theme === "brass";

  function handleClick() {
    const next: ThemePreference = brass ? "parchment" : "brass";
    if (user) {
      void persistTheme(next, theme);
      return;
    }
    pickGuestTheme(next);
  }

  return (
    <button
      type="button"
      onClick={handleClick}
      aria-pressed={brass}
      aria-label={t("settings.theme.label")}
      title={t("settings.theme.label")}
      data-testid={testId}
      className={cn(
        "text-ink-dim hover:bg-surface-sunken hover:text-ink inline-flex size-8 shrink-0 cursor-pointer items-center justify-center rounded-lg border border-border bg-transparent transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/70",
        className,
      )}
    >
      {brass ? (
        <Sun className="size-4" aria-hidden="true" />
      ) : (
        <Moon className="size-4" aria-hidden="true" />
      )}
    </button>
  );
}
