import { Trans } from "react-i18next";
import { Outlet } from "react-router";

import { TopBar } from "@/shared/components/TopBar";

const LINKEDIN_URL = "https://www.linkedin.com/in/emilijan-koteski/";

/**
 * Parchment-themed shell for the pre-authentication routes (Login + Register).
 * Hosts the shared <TopBar /> (no nav, no user menu, no server persistence
 * for the language picker) and a centered stage for the page's <AuthCard />.
 *
 * Mounted once via App.tsx as a layout route around `/login` and `/register`,
 * so navigating between the two pages doesn't remount the top bar or the
 * footer credit — only the card swaps.
 */
export function AuthLayout() {
  return (
    <div className="text-ink font-body relative flex min-h-screen flex-col overflow-hidden">
      <TopBar languageTestIdPrefix="auth-language" showFullBrand />

      <main className="flex flex-1 items-center justify-center px-6 py-8">
        <Outlet />
      </main>

      <footer
        className="text-ink-mute pt-4 pb-5 text-center text-[11.5px] tracking-[0.1px]"
        data-testid="auth-footer"
      >
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
      </footer>
    </div>
  );
}
