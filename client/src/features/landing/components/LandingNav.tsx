import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import { BrandLockup } from "@/features/landing/components/BrandLockup";
import { LandingCta } from "@/features/landing/components/LandingCta";
import { LanguageSelector } from "@/shared/components/LanguageSelector";
import { ThemeToggle } from "@/shared/components/ThemeToggle";

/** Marketing top bar over the felt hero; it sits inside `.felt-surface`, so its controls read light on felt. */
export function LandingNav() {
  const { t } = useTranslation();
  return (
    <header className="border-brass/20 absolute inset-x-0 top-0 z-30 border-b">
      <div className="mx-auto flex h-15 max-w-310 items-center gap-6 px-4 md:h-17 md:px-7">
        <BrandLockup showSuffix />
        <Link
          to="/rules"
          data-testid="landing-rules-link"
          className="text-ink-dim hover:text-ink hidden text-sm font-medium transition-colors md:inline"
        >
          {t("landing.nav.rules")}
        </Link>
        <nav className="ml-auto flex items-center gap-1.5 sm:gap-2.5">
          <LanguageSelector persistToServer={false} testIdPrefix="landing-language" />
          <ThemeToggle testId="landing-theme-toggle" />
          <Link
            to="/login"
            data-testid="landing-login"
            className="text-ink-dim hover:text-ink px-2 text-sm font-semibold whitespace-nowrap transition-colors"
          >
            {t("landing.nav.login")}
          </Link>
          <span className="hidden sm:inline-flex">
            <LandingCta to="/register" testId="landing-signup" className="px-4 py-2 text-sm">
              {t("landing.nav.signup")}
            </LandingCta>
          </span>
        </nav>
      </div>
    </header>
  );
}
