import { useTranslation } from "react-i18next";

import { Bokeh } from "@/features/landing/components/Bokeh";
import { BrowserFrame } from "@/features/landing/components/BrowserFrame";
import { Dama } from "@/features/landing/components/Dama";
import { LandingCta } from "@/features/landing/components/LandingCta";
import { LandingNav } from "@/features/landing/components/LandingNav";
import { PhoneFrame } from "@/features/landing/components/PhoneFrame";
import { ScreenShot } from "@/features/landing/components/ScreenShot";
import { Eyebrow } from "@/shared/components/ui/eyebrow";

const SPOTLIGHT =
  "pointer-events-none absolute rounded-full bg-[radial-gradient(closest-side,rgba(243,217,160,.3),rgba(224,181,106,.08)_60%,transparent)]";

/**
 * The felt hero: headline and sign-up beside the real match table on a laptop and a phone, with Dama.
 * The stage scales with its width from tablet up; phones get a tighter phone-and-Dama pair.
 */
export function LandingHero() {
  const { t } = useTranslation();
  const phoneShot = (
    <ScreenShot name="match-phone-croatian" alt={t("landing.hero.phoneAlt")} eager />
  );
  return (
    <section
      className="felt-surface felt-hero-bg relative overflow-hidden"
      data-testid="landing-hero"
    >
      <div className="felt-rail absolute inset-x-0 top-0 h-2" aria-hidden="true" />
      <Bokeh />
      <LandingNav />

      <div className="relative mx-auto grid max-w-310 gap-6 px-4 pt-24 md:px-7 md:pt-32 xl:grid-cols-[470px_1fr] xl:gap-5 xl:pt-37.5">
        <div>
          <Eyebrow size="xl" className="text-brass">
            {t("landing.hero.eyebrow")}
          </Eyebrow>
          <h1 className="font-display text-ink mt-3.5 text-[60px] leading-[0.92] font-semibold tracking-[0.03em] uppercase md:text-[84px] xl:text-[96px]">
            {t("landing.hero.title")}{" "}
            <span className="bg-[linear-gradient(180deg,#f3dca6,#c9a876_60%,#a8864f)] bg-clip-text text-transparent">
              {t("landing.hero.titleAccent")}
            </span>
          </h1>
          <p className="text-ink-dim mt-4 max-w-120 text-base leading-relaxed md:text-[19px] xl:max-w-110">
            {t("landing.hero.subtitle")}
          </p>
          <div className="mt-7.5">
            <LandingCta to="/register" testId="landing-hero-cta">
              {t("landing.hero.cta")}
            </LandingCta>
          </div>
        </div>

        <div className="relative h-125 md:hidden">
          <div className={`${SPOTLIGHT} -top-5 left-10 size-105`} aria-hidden="true" />
          <PhoneFrame className="absolute top-4 left-2 z-10 w-50">{phoneShot}</PhoneFrame>
          <Dama pose="signature" eager className="absolute -bottom-37.5 left-37.5 z-20 h-155" />
        </div>

        <div className="relative mx-auto hidden aspect-[700/650] w-full max-w-160 md:block xl:mx-0 xl:max-w-175">
          <div
            className={`${SPOTLIGHT} -top-[6%] left-[40%] aspect-square w-[89%]`}
            aria-hidden="true"
          />
          <BrowserFrame
            tone="felt"
            url="beljot.online/match"
            className="absolute top-[6%] left-[4.3%] w-[80%]"
          >
            <ScreenShot name="match-desktop" alt={t("landing.hero.tableAlt")} />
          </BrowserFrame>
          <PhoneFrame className="absolute top-[38%] left-[-3%] z-10 w-[25.4%]">
            {phoneShot}
          </PhoneFrame>
          <Dama
            pose="signature"
            eager
            className="absolute -bottom-[23%] left-[45%] z-20 h-[114%]"
          />
        </div>
      </div>
    </section>
  );
}
