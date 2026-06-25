import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import { LanguageSelector } from "@/shared/components/LanguageSelector";
import { buttonVariants } from "@/shared/components/ui/button";
import { cn } from "@/shared/lib/utils";

// Parchment sticky header for public content pages (Rules) when
// viewed by a logged-out visitor. Mirrors the app TopBar's chrome but offers
// sign-in CTAs instead of the authed nav/user menu. The logo links home.

export function PublicTopBar() {
  const { t } = useTranslation();
  return (
    <nav
      className="border-border sticky top-0 z-50 flex h-15 items-center border-b bg-[rgba(245,242,232,0.85)] px-7 backdrop-blur-md"
      data-testid="public-nav"
    >
      <Link to="/" className="flex items-center gap-2.5" data-testid="public-logo">
        <img src="/beljot-logo.svg" alt="" aria-hidden="true" className="size-7 shrink-0" />
        <span className="font-display text-ink text-xl font-semibold tracking-tight">
          Beljot
          <span className="text-brass font-medium">.online</span>
        </span>
      </Link>

      <div className="ml-auto flex items-center gap-2.5 sm:gap-3.5">
        <LanguageSelector persistToServer={false} testIdPrefix="public-language" />
        <Link
          to="/login"
          data-testid="public-login"
          className="text-ink-dim hover:text-ink text-sm font-semibold transition-colors"
        >
          {t("landing.nav.login")}
        </Link>
        {/* Sign-up CTA — hidden on phones, where it overflowed the narrow bar.
            The Rules body still routes visitors to register; the mobile header
            just needs the brand, language picker and a way back in (log in).
            Wrapped in a span (rather than toggling the button's own display)
            so `hidden` isn't fighting buttonVariants' `inline-flex`. */}
        <span className="hidden sm:inline-flex">
          <Link
            to="/register"
            data-testid="public-signup"
            className={cn(buttonVariants(), "h-9 px-4")}
          >
            {t("landing.nav.signup")}
          </Link>
        </span>
      </div>
    </nav>
  );
}
