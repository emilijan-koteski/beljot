import { useTranslation } from "react-i18next";

import { FeltVignette } from "@/features/landing/components/FeltVignette";
import { LandingCta } from "@/features/landing/components/LandingCta";
import { LandingNav } from "@/features/landing/components/LandingNav";
import { Eyebrow } from "@/shared/components/ui/eyebrow";
import { usePublicStatsQuery } from "@/shared/hooks/queries/usePublicStats";

// Cinematic felt hero with the headline and CTA beside a live felt-table vignette.
// The section owns the `.felt-surface` scope, so everything inside reads as light-on-felt.
export function LandingHero() {
  const { t } = useTranslation();
  const { data: stats } = usePublicStatsQuery();
  return (
    <section className="felt-surface felt-hero-bg relative overflow-hidden">
      <div className="felt-rail absolute inset-x-0 top-0 h-2" aria-hidden="true" />

      <LandingNav />

      <div className="mx-auto grid max-w-7xl items-center gap-12 px-[clamp(28px,5vw,72px)] pt-[clamp(120px,16vw,150px)] pb-[clamp(64px,9vw,96px)] md:grid-cols-[1.02fr_0.98fr]">
        <div>
          <div className="mb-4.5">
            <Eyebrow size="xl" className="text-brass">
              {t("landing.hero.eyebrow")}
            </Eyebrow>
          </div>
          <h1 className="text-ink text-[clamp(44px,6.5vw,70px)] font-bold leading-none tracking-tight">
            {t("landing.hero.title1")}
            <br />
            {t("landing.hero.title2")}
          </h1>
          <p className="text-ink-dim mt-6 max-w-120 text-[clamp(16px,1.6vw,18px)] leading-relaxed">
            {t("landing.hero.subtitle")}
          </p>
          <div className="mt-8 flex flex-wrap items-center gap-3.5">
            <LandingCta to="/register" testId="landing-hero-cta">
              {t("landing.hero.cta")}
            </LandingCta>
          </div>
          {stats && (
            <div className="text-ink-mute mt-7 flex flex-wrap items-center gap-4 text-sm">
              <span className="inline-flex items-center gap-2">
                <span
                  className="bg-accent size-2 rounded-full"
                  style={{ animation: "pulse-dot 1.8s ease-in-out infinite" }}
                />
                <b className="text-ink tabular-nums">{stats.online}</b> {t("landing.hero.playing")}
              </span>
              <span className="text-brass/40">·</span>
              <span>
                <b className="text-ink tabular-nums">{stats.openRooms}</b>{" "}
                {t("landing.hero.openTables")}
              </span>
            </div>
          )}
        </div>

        <div className="flex justify-center">
          <FeltVignette
            width={580}
            height={400}
            labels={{
              trump: t("landing.vig.trump"),
              opponent: t("landing.vig.opponent"),
              partner: t("landing.vig.partner"),
            }}
          />
        </div>
      </div>
    </section>
  );
}
