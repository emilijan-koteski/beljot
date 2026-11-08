import { useTranslation } from "react-i18next";

import { Bokeh } from "@/features/landing/components/Bokeh";
import { Dama } from "@/features/landing/components/Dama";
import { LandingCta } from "@/features/landing/components/LandingCta";

/** The closing felt band: Dama cheering beside the last call to sign up. */
export function ClosingCta() {
  const { t } = useTranslation();
  return (
    <section
      className="felt-surface felt-cta-bg relative overflow-hidden"
      data-testid="landing-closing"
    >
      <Bokeh />
      <div
        className="pointer-events-none absolute -bottom-30 -left-15 size-130 rounded-full bg-[radial-gradient(closest-side,rgba(243,217,160,.3),rgba(224,181,106,.08)_60%,transparent)] lg:-top-15 lg:-right-10 lg:bottom-auto lg:left-auto lg:size-160"
        aria-hidden="true"
      />
      <Dama
        pose="win-half"
        className="absolute -bottom-50 left-1/2 h-140 -translate-x-[46%] lg:right-15 lg:-bottom-40 lg:left-auto lg:h-175 lg:translate-x-0"
      />
      <div className="relative mx-auto max-w-310 px-4 pt-18 pb-95 md:px-7 lg:py-30">
        <h2 className="font-display text-ink m-0 text-[44px] leading-[1.02] font-semibold tracking-[0.03em] uppercase md:text-[68px]">
          {t("landing.cta.title1")}
          <br />
          {t("landing.cta.title2")}
        </h2>
        <p className="text-ink-dim mt-4 text-base md:text-lg">{t("landing.cta.body")}</p>
        <div className="mt-7.5">
          <LandingCta to="/register" testId="landing-cta">
            {t("landing.cta.cta")}
          </LandingCta>
        </div>
      </div>
    </section>
  );
}
