import { Gamepad2, KeyRound, LogOut, ShieldCheck, UserRound, Volume2 } from "lucide-react";
import { type ReactNode, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";

import { PageShell } from "@/shared/components/layout/PageShell";
import { Button } from "@/shared/components/ui/button";
import { useLayoutBand } from "@/shared/hooks/useLayoutBand";
import { cn } from "@/shared/lib/utils";
import { useAuthStore } from "@/shared/stores/authStore";

import { AccountSection } from "./components/AccountSection";
import { GameSection } from "./components/GameSection";
import { PrivacySection } from "./components/PrivacySection";
import type { SettingsSectionId } from "./components/SettingsGroup";
import { SignInSection } from "./components/SignInSection";
import { SoundSection } from "./components/SoundSection";

const SECTIONS: Array<{ id: SettingsSectionId; icon: ReactNode }> = [
  { id: "account", icon: <UserRound /> },
  { id: "signIn", icon: <KeyRound /> },
  { id: "game", icon: <Gamepad2 /> },
  { id: "sound", icon: <Volume2 /> },
  { id: "privacy", icon: <ShieldCheck /> },
];

/** Follows which settings group sits at the top of the window, so the desktop section list can mark it. */
function useActiveSection(enabled: boolean) {
  const [active, setActive] = useState<SettingsSectionId>("account");
  useEffect(() => {
    if (!enabled || typeof IntersectionObserver === "undefined") return;
    const observer = new IntersectionObserver(
      (entries) => {
        const top = entries.find((e) => e.isIntersecting);
        if (top) setActive(top.target.id.replace("settings-", "") as SettingsSectionId);
      },
      { rootMargin: "-20% 0px -70% 0px" },
    );
    for (const s of SECTIONS) {
      const el = document.getElementById(`settings-${s.id}`);
      if (el) observer.observe(el);
    }
    return () => observer.disconnect();
  }, [enabled]);
  return [active, setActive] as const;
}

/**
 * Account settings at /settings: account, sign-in, game, sound and privacy, then Log out.
 * One column on a phone, two on a tablet, and a sticky section list beside the groups on a desktop.
 */
export function SettingsPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const logout = useAuthStore((s) => s.logout);
  const band = useLayoutBand();
  const desktop = band === "desktop";
  const [active, setActive] = useActiveSection(desktop);

  function handleLogout() {
    logout();
    navigate("/");
  }

  function jumpTo(id: SettingsSectionId) {
    setActive(id);
    document
      .getElementById(`settings-${id}`)
      ?.scrollIntoView({ behavior: "smooth", block: "start" });
  }

  const title = (
    <h1 className="text-ink m-0 text-2xl font-semibold tracking-tight md:text-3xl">
      {t("settings.title")}
    </h1>
  );

  const logoutButton = (
    <Button
      variant="outline"
      onClick={handleLogout}
      className={cn(
        "text-danger hover:text-danger gap-2",
        band === "phone" ? "h-11 w-full" : "self-start",
      )}
      data-testid="settings-logout"
    >
      <LogOut className="size-4" />
      {t("nav.logout")}
    </Button>
  );

  if (desktop) {
    return (
      <PageShell testId="settings-page">
        <div className="mx-auto grid w-full max-w-256 grid-cols-[13.75rem_minmax(0,1fr)] items-start gap-10">
          <nav
            className="sticky top-21 flex flex-col gap-1"
            aria-label={t("settings.sectionsLabel")}
            data-testid="settings-nav"
          >
            <div className="mb-3 px-3">{title}</div>
            {SECTIONS.map((s) => (
              <button
                key={s.id}
                type="button"
                onClick={() => jumpTo(s.id)}
                aria-current={active === s.id ? "true" : undefined}
                data-testid={`settings-nav-${s.id}`}
                className={cn(
                  "flex cursor-pointer items-center gap-2.5 rounded-lg px-3 py-2 text-left text-sm font-medium transition-colors [&>svg]:size-4",
                  active === s.id
                    ? "bg-accent-soft text-ink"
                    : "text-ink-dim hover:bg-surface-sunken hover:text-ink",
                )}
              >
                {s.icon}
                {t(`settings.sections.${s.id}`)}
              </button>
            ))}
            <div className="border-border my-2 border-t" />
            <button
              type="button"
              onClick={handleLogout}
              data-testid="settings-logout"
              className="text-danger hover:bg-surface-sunken flex cursor-pointer items-center gap-2.5 rounded-lg px-3 py-2 text-left text-sm font-medium [&>svg]:size-4"
            >
              <LogOut />
              {t("nav.logout")}
            </button>
          </nav>
          <div className="flex flex-col gap-6 pt-14">
            <AccountSection />
            <SignInSection />
            <GameSection wide />
            <SoundSection />
            <PrivacySection />
          </div>
        </div>
      </PageShell>
    );
  }

  if (band === "tablet") {
    return (
      <PageShell testId="settings-page">
        {title}
        <div className="grid grid-cols-2 items-start gap-5">
          <div className="flex flex-col gap-5">
            <AccountSection />
            <SignInSection />
            <PrivacySection />
            {logoutButton}
          </div>
          <div className="flex flex-col gap-5">
            <GameSection />
            <SoundSection />
          </div>
        </div>
      </PageShell>
    );
  }

  return (
    <PageShell testId="settings-page">
      {title}
      <AccountSection />
      <SignInSection />
      <GameSection />
      <SoundSection />
      <PrivacySection />
      {logoutButton}
    </PageShell>
  );
}
