import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import { BrandLockup } from "@/features/landing/components/BrandLockup";
import { ContactDialog } from "@/features/landing/components/ContactDialog";
import { SupportDialog } from "@/shared/components/support/SupportDialog";

// Felt footer — brand, a row of links, and the copyright. Rules reaches the
// public reference page; Privacy + Terms the public legal pages;
// Contact and Support each open a dialog.

export function LandingFooter() {
  const { t } = useTranslation();
  const [supportOpen, setSupportOpen] = useState(false);

  const links: Array<{ label: string; to: string }> = [
    { label: t("landing.foot.rules"), to: "/rules" },
    { label: t("landing.foot.privacy"), to: "/privacy" },
    { label: t("landing.foot.terms"), to: "/terms" },
  ];

  return (
    <footer className="felt-surface border-brass-soft border-t py-10">
      <div className="mx-auto flex max-w-7xl flex-col items-start gap-5 px-[clamp(28px,5vw,72px)] sm:flex-row sm:flex-wrap sm:items-center">
        <BrandLockup size={30} wordmarkSize={18} showSuffix />
        <nav className="text-ink-mute flex flex-wrap items-center gap-x-6 gap-y-2 text-[13px] sm:ml-auto">
          {links.map((l) => (
            <Link key={l.label} to={l.to} className="hover:text-ink transition-colors">
              {l.label}
            </Link>
          ))}
          <ContactDialog />
          {/* Labelled "Support Beljot.online" rather than a bare "Support":
              next to Contact, the short form would read as a help desk. */}
          <button
            type="button"
            onClick={() => setSupportOpen(true)}
            data-testid="support-footer-link"
            className="hover:text-ink cursor-pointer transition-colors"
          >
            {t("support.label")}
          </button>
        </nav>
        <span className="text-ink-off font-mono text-[11px]">{t("landing.foot.copyright")}</span>
      </div>

      <SupportDialog open={supportOpen} onOpenChange={setSupportOpen} />
    </footer>
  );
}
