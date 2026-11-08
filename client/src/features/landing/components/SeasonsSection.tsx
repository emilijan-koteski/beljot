import { useTranslation } from "react-i18next";

import { PhoneFrame } from "@/features/landing/components/PhoneFrame";
import { ScreenShot } from "@/features/landing/components/ScreenShot";
import { SectionHeading } from "@/features/landing/components/SectionHeading";

const TIERS = [
  "iron",
  "bronze",
  "silver",
  "gold",
  "platinum",
  "diamond",
  "master",
  "grandmaster",
] as const;

/** The season ladder beside the real profile and leaderboard, fanned on two phones. */
export function SeasonsSection() {
  const { t } = useTranslation();
  return (
    <section className="bg-surface py-18 md:py-30" data-testid="landing-seasons">
      <div className="mx-auto grid max-w-310 items-center gap-10 px-4 md:px-7 lg:grid-cols-[400px_1fr]">
        <div>
          <SectionHeading
            eyebrow={t("landing.seasons.eyebrow")}
            title={t("landing.seasons.title")}
            body={t("landing.seasons.body")}
          />
          <ul className="mt-5.5 flex flex-wrap gap-1.5">
            {TIERS.map((tier) => (
              <li
                key={tier}
                className="bg-surface-elevated ring-border text-ink-dim last:text-brass-deep last:ring-brass rounded-lg px-2.25 py-1.75 font-mono text-[11px] leading-none font-semibold tracking-[0.08em] uppercase ring-1 ring-inset"
              >
                {t(`season.tier.${tier}`)}
              </li>
            ))}
          </ul>
        </div>
        <div className="relative mx-auto h-130 w-full max-w-90 md:h-160 md:max-w-140">
          <PhoneFrame className="absolute top-7.5 left-0 w-55 -rotate-5 md:w-72.5">
            <ScreenShot name="profile-phone" themed alt={t("landing.seasons.profileAlt")} />
          </PhoneFrame>
          <PhoneFrame className="absolute top-0 left-32.5 z-10 w-55 rotate-4 md:right-0 md:left-auto md:w-72.5">
            <ScreenShot name="leaderboard-phone" themed alt={t("landing.seasons.leaderboardAlt")} />
          </PhoneFrame>
        </div>
      </div>
    </section>
  );
}
