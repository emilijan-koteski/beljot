import { Coins, Shield } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Dama } from "@/features/landing/components/Dama";
import { DeckFan } from "@/features/landing/components/DeckFan";
import { SectionHeading } from "@/features/landing/components/SectionHeading";
import { RULE_ICONS } from "@/shared/components/roomRules/roomRuleLines";
import { COIN_GOLD } from "@/shared/lib/coinGold";

const VARIANTS = ["bitola", "croatian"] as const;
const ROOM_RULES = [
  { key: "target", icon: RULE_ICONS.stopAtTarget },
  { key: "timer", icon: RULE_ICONS.moveTimer },
  { key: "direction", icon: RULE_ICONS.counterClockwise },
  { key: "declarations", icon: RULE_ICONS.declarationsOn },
  { key: "buyIn", icon: Coins },
  { key: "honor", icon: Shield },
] as const;
const DECKS = [
  { deck: "french", label: "frenchDeck" },
  { deck: "croatian", label: "croatianDeck" },
] as const;

const PANEL = "bg-background ring-border rounded-[22px] ring-1 ring-inset";

/** Both rulebooks, the room options a host can set, and both decks, with Dama pointing at the title. */
export function RulesSection() {
  const { t } = useTranslation();
  return (
    <section className="bg-surface py-18 md:py-30" data-testid="landing-rules">
      <div className="mx-auto max-w-310 px-4 md:px-7">
        <div className="grid items-end gap-10 lg:grid-cols-[1fr_300px]">
          <SectionHeading
            eyebrow={t("landing.rules.eyebrow")}
            title={
              <>
                {t("landing.rules.title1")}
                <br />
                {t("landing.rules.title2")}
              </>
            }
            body={t("landing.rules.body")}
          />
          <div className="relative hidden h-62.5 lg:block">
            <Dama pose="pointing" className="absolute right-2.5 -bottom-11.5 h-82.5 -scale-x-100" />
          </div>
        </div>

        <div className="mt-11 grid gap-5 md:grid-cols-2">
          {VARIANTS.map((variant) => (
            <article key={variant} className={`${PANEL} p-6 md:p-7`}>
              <h3 className="font-display text-ink m-0 text-[30px] leading-none font-semibold tracking-[0.03em] uppercase">
                {t(`landing.rules.${variant}.title`)}
              </h3>
              <ul className="mt-4 grid gap-2.5">
                {(["b1", "b2", "b3"] as const).map((line) => (
                  <li
                    key={line}
                    className="text-ink-dim before:bg-brass flex gap-2.5 text-[15px] leading-normal before:mt-2.25 before:size-1.5 before:shrink-0 before:rotate-45 before:rounded-[2px]"
                  >
                    {t(`landing.rules.${variant}.${line}`)}
                  </li>
                ))}
              </ul>
            </article>
          ))}
        </div>

        <div
          className={`${PANEL} mt-5 grid items-center gap-3 p-6 md:grid-cols-[240px_1fr] md:px-7`}
        >
          <h3 className="text-ink m-0 text-base font-semibold">{t("landing.rules.room.title")}</h3>
          <ul className="flex flex-wrap gap-2">
            {ROOM_RULES.map(({ key, icon: Icon }) => (
              <li
                key={key}
                className="bg-surface ring-border text-ink inline-flex h-8.5 items-center gap-1.75 rounded-full px-3 text-[13px] font-medium ring-1 ring-inset"
              >
                <Icon
                  className="text-brass-deep size-3.75"
                  style={key === "buyIn" ? { color: COIN_GOLD } : undefined}
                  aria-hidden="true"
                />
                {t(`landing.rules.room.${key}`)}
              </li>
            ))}
          </ul>
        </div>

        <div className="mt-5 grid gap-5 md:grid-cols-2">
          {DECKS.map(({ deck, label }) => (
            <div key={deck} className={`${PANEL} overflow-hidden px-6 pt-6 md:px-7`}>
              <h3 className="text-ink m-0 text-base font-semibold">
                {t(`landing.rules.${label}`)}
              </h3>
              <DeckFan deck={deck} />
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
