import { useTranslation } from "react-i18next";

import { LandingCta } from "@/features/landing/components/LandingCta";
import { Eyebrow } from "@/shared/components/ui/eyebrow";

// Closing felt CTA band with a spotlight, big headline and sign-up CTA.
export function ClosingCta() {
  const { t } = useTranslation();
  return (
    <section className="felt-surface felt-cta-bg relative overflow-hidden py-[clamp(64px,10vw,100px)] text-center">
      <div className="mx-auto max-w-7xl px-[clamp(28px,5vw,72px)]">
        <div className="mb-4.5 flex justify-center">
          <Eyebrow size="xl" className="text-brass">
            {t("landing.cta.eyebrow")}
          </Eyebrow>
        </div>
        <h2 className="font-display text-ink mx-auto mb-5 max-w-190 text-[clamp(38px,4.6vw,54px)] font-bold tracking-[-1.4px]">
          {t("landing.cta.title")}
        </h2>
        <p className="text-ink-dim mx-auto mb-8 max-w-130 text-[18px] leading-[1.6]">
          {t("landing.cta.body")}
        </p>
        <LandingCta to="/register" size="lg" testId="landing-cta">
          {t("landing.cta.cta")}
        </LandingCta>
      </div>
    </section>
  );
}
