import { BookOpen, Languages, Layers, MonitorSmartphone } from "lucide-react";
import { useTranslation } from "react-i18next";

const FACTS = [
  { key: "variants", icon: BookOpen },
  { key: "decks", icon: Layers },
  { key: "languages", icon: Languages },
  { key: "download", icon: MonitorSmartphone },
] as const;

/** Four one-line facts under the hero. */
export function FactStrip() {
  const { t } = useTranslation();
  return (
    <section className="bg-surface border-border border-b" data-testid="landing-facts">
      <ul className="mx-auto grid max-w-310 grid-cols-2 px-4 md:grid-cols-4 md:px-7">
        {FACTS.map(({ key, icon: Icon }) => (
          <li
            key={key}
            className="border-border flex items-center gap-3 py-4 md:border-l md:px-6 md:py-5.5 md:first:border-l-0 md:first:pl-0"
          >
            <span className="bg-accent-soft text-accent grid size-10 shrink-0 place-items-center rounded-xl">
              <Icon className="size-4.5" aria-hidden="true" />
            </span>
            <span className="text-ink text-sm font-semibold md:text-[15px]">
              {t(`landing.facts.${key}`)}
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}
