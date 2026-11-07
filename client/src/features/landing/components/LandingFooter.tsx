import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import { BrandLockup } from "@/features/landing/components/BrandLockup";
import { ContactDialog } from "@/features/landing/components/ContactDialog";
import { LanguageSelector } from "@/shared/components/LanguageSelector";
import { SupportDialog } from "@/shared/components/support/SupportDialog";
import { ThemeToggle } from "@/shared/components/ThemeToggle";

/** Felt footer with the brand, rules, legal, contact and support links, language, theme and copyright. */
export function LandingFooter() {
  const { t } = useTranslation();
  const [supportOpen, setSupportOpen] = useState(false);

  const links: Array<{ label: string; to: string }> = [
    { label: t("landing.foot.rules"), to: "/rules" },
    { label: t("landing.foot.privacy"), to: "/privacy" },
    { label: t("landing.foot.terms"), to: "/terms" },
  ];

  return (
    <footer className="felt-surface border-brass/20 border-t py-7 md:py-8">
      <div className="mx-auto flex max-w-310 flex-wrap items-center gap-x-7 gap-y-4 px-4 md:px-7">
        <BrandLockup wordmarkSize={18} showSuffix />
        <nav className="text-ink-mute flex w-full flex-wrap items-center gap-x-5.5 gap-y-2 text-sm md:ml-auto md:w-auto">
          {links.map((l) => (
            <Link key={l.label} to={l.to} className="hover:text-ink transition-colors">
              {l.label}
            </Link>
          ))}
          <ContactDialog />
          <button
            type="button"
            onClick={() => setSupportOpen(true)}
            data-testid="support-footer-link"
            className="hover:text-ink cursor-pointer transition-colors"
          >
            {t("landing.foot.support")}
          </button>
        </nav>
        <div className="flex items-center gap-2">
          <LanguageSelector persistToServer={false} testIdPrefix="landing-footer-language" />
          <ThemeToggle testId="landing-footer-theme-toggle" />
        </div>
        <span className="text-ink-mute font-mono text-2xs">{t("landing.foot.copyright")}</span>
      </div>

      <SupportDialog open={supportOpen} onOpenChange={setSupportOpen} />
    </footer>
  );
}
