import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import { BrandLockup } from "@/features/landing/components/BrandLockup";
import { LandingCta } from "@/features/landing/components/LandingCta";
import { LanguageSelector } from "@/shared/components/LanguageSelector";

// Marketing top bar that sits absolutely over the felt hero.
// It renders inside the hero's `.felt-surface`, so everything auto-themes to light/brass.
export function LandingNav() {
  const { t } = useTranslation();
  return (
    <header className="border-brass-soft absolute inset-x-0 top-0 z-10 h-18.5 border-b">
      <div className="mx-auto flex h-full max-w-7xl items-center px-[clamp(28px,5vw,72px)]">
        <BrandLockup showSuffix />
        <nav className="ml-auto flex items-center gap-3 sm:gap-4">
          <LanguageSelector persistToServer={false} testIdPrefix="landing-language" />
          <Link
            to="/login"
            data-testid="landing-login"
            className="text-ink-dim hover:text-ink text-sm font-semibold transition-colors"
          >
            {t("landing.nav.login")}
          </Link>
          <span className="hidden sm:inline-flex">
            <LandingCta to="/register" testId="landing-signup" className="px-4.5 py-2.5 text-sm">
              {t("landing.nav.signup")}
            </LandingCta>
          </span>
        </nav>
      </div>
    </header>
  );
}
