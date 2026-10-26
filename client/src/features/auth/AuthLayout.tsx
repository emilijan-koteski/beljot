import { Trans } from "react-i18next";
import { Outlet } from "react-router";

import { SupportNote } from "@/shared/components/support/SupportNote";
import { TopBar } from "@/shared/components/TopBar";

const LINKEDIN_URL = "https://www.linkedin.com/in/emilijan-koteski/";

/**
 * Parchment-themed layout route for the pre-authentication pages.
 * Hosts the shared TopBar and footer so only the page's AuthCard swaps on navigation.
 */
export function AuthLayout() {
  return (
    <div className="text-ink relative flex min-h-screen flex-col overflow-hidden">
      <TopBar languageTestIdPrefix="auth-language" showFullBrand />

      <main className="flex flex-1 items-center justify-center px-6 py-8">
        <Outlet />
      </main>

      <footer className="text-ink-mute pt-4 pb-5 text-center text-xs" data-testid="auth-footer">
        <Trans
          i18nKey="auth.footer.credit"
          components={{
            suit: (
              <span
                className="text-accent"
                style={{ fontFamily: "var(--font-suit)" }}
                aria-hidden="true"
              />
            ),
            name: (
              <a
                href={LINKEDIN_URL}
                target="_blank"
                rel="noopener noreferrer"
                className="text-ink-dim hover:text-accent transition-colors"
                data-testid="auth-footer-link"
              />
            ),
          }}
        />

        <SupportNote variant="auth" />
      </footer>
    </div>
  );
}
