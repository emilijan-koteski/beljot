import { useTranslation } from "react-i18next";

import { Step } from "@/features/landing/components/Step";
import { Eyebrow } from "@/shared/components/ui/eyebrow";

// "How it works" parchment band with three numbered steps.
export function HowItWorks() {
  const { t } = useTranslation();
  return (
    <section id="how" className="py-[clamp(60px,9vw,92px)]">
      <div className="mx-auto max-w-7xl px-[clamp(28px,5vw,72px)]">
        <div className="mb-4.5">
          <Eyebrow size="xl">{t("landing.how.eyebrow")}</Eyebrow>
        </div>
        <h2 className="text-ink mb-[clamp(36px,5vw,52px)] max-w-190 text-[clamp(30px,3.4vw,42px)] font-bold tracking-tight">
          {t("landing.how.title")}
        </h2>
        <div className="grid gap-12 md:grid-cols-3">
          <Step n="1" title={t("landing.how.s1t")} body={t("landing.how.s1b")} />
          <Step n="2" title={t("landing.how.s2t")} body={t("landing.how.s2b")} />
          <Step n="3" title={t("landing.how.s3t")} body={t("landing.how.s3b")} />
        </div>
      </div>
    </section>
  );
}
