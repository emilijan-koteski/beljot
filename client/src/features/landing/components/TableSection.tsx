import { Clock, Shield } from "lucide-react";
import type { CSSProperties, ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { PhoneFrame } from "@/features/landing/components/PhoneFrame";
import { ScreenShot } from "@/features/landing/components/ScreenShot";
import { SectionHeading } from "@/features/landing/components/SectionHeading";
import { cn } from "@/shared/lib/utils";

const FAIR_PLAY = [
  { key: "clock", icon: Clock },
  { key: "fair", icon: Shield },
] as const;

/** A label beside the 300px phone with a brass leader line pointing at one spot on the real screen. */
function Callout({
  side,
  style,
  children,
}: {
  side: "left" | "right";
  style: CSSProperties;
  children: ReactNode;
}) {
  return (
    <div
      className={cn(
        "bg-surface-elevated ring-border text-ink absolute hidden items-center gap-2 rounded-xl px-3 py-2 text-[13px] font-semibold whitespace-nowrap shadow-md ring-1 md:flex",
        "after:bg-brass after:absolute after:top-1/2 after:h-px after:w-10.5",
        side === "left" ? "after:left-full" : "after:right-full",
      )}
      style={style}
    >
      {children}
    </div>
  );
}

/** The real table on a phone with callouts, beside the partnership and fair-play lines. */
export function TableSection() {
  const { t } = useTranslation();
  return (
    <section className="py-18 md:py-30" data-testid="landing-table">
      <div className="mx-auto grid max-w-310 items-center gap-12 px-4 md:px-7 lg:grid-cols-[1fr_420px]">
        <div className="relative order-2 mx-auto w-67.5 md:w-75 lg:order-none">
          <PhoneFrame>
            <ScreenShot name="match-phone" alt={t("landing.table.alt")} />
          </PhoneFrame>
          <Callout side="left" style={{ right: 319, top: 17 }}>
            {t("landing.table.score")}
            <small className="text-ink-dim font-medium">462 · 318</small>
          </Callout>
          <Callout side="right" style={{ left: 204, top: 18 }}>
            {t("landing.table.trump")}
          </Callout>
          <Callout side="left" style={{ right: 217, top: 422 }}>
            {t("landing.table.turn")}
            <small className="text-ink-dim font-medium">11s</small>
          </Callout>
          <Callout side="right" style={{ left: 293, top: 541 }}>
            {t("landing.table.legal")}
          </Callout>
        </div>
        <div className="order-1 lg:order-none">
          <SectionHeading
            eyebrow={t("landing.table.eyebrow")}
            title={t("landing.table.title")}
            body={t("landing.table.body")}
          />
          <ul className="mt-7 grid gap-3.5">
            {FAIR_PLAY.map(({ key, icon: Icon }) => (
              <li key={key} className="flex items-start gap-3.5">
                <span className="bg-accent-soft text-accent grid size-9 shrink-0 place-items-center rounded-[10px]">
                  <Icon className="size-4.25" aria-hidden="true" />
                </span>
                <span>
                  <b className="text-ink block text-[15px] font-semibold">
                    {t(`landing.table.${key}Title`)}
                  </b>
                  <span className="text-ink-dim text-sm">{t(`landing.table.${key}Body`)}</span>
                </span>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </section>
  );
}
