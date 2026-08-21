import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { Chapter } from "@/features/rules/components/Chapter";
import { ChapterIndex } from "@/features/rules/components/ChapterIndex";
import { RulesFooter } from "@/features/rules/components/RulesFooter";
import { RulesHero } from "@/features/rules/components/RulesHero";
import { getRulesContent } from "@/features/rules/content/rulesContent";
import { RulesProvider } from "@/features/rules/RulesContext";
import { Tabs, TabsList, TabsTrigger } from "@/shared/components/ui/tabs";
import { TooltipProvider } from "@/shared/components/ui/tooltip";
import { variantLabel } from "@/shared/lib/roomLabels";
import { normalizeVariant, type Variant, VARIANTS } from "@/shared/types/matchTypes";

// The eyebrow above the tab bar names the tablist through aria-labelledby, so
// the string is announced once rather than duplicated into an aria-label.
const VARIANT_LABEL_ID = "rules-variant-label";

export function RulesPage() {
  const { t, i18n } = useTranslation();
  const content = getRulesContent(i18n.language);
  const sections = content.sections;
  const firstId = sections[0]?.id ?? "";

  // Bitola is the default tab: it is the only variant Quick Play offers and the
  // one an unfamiliar reader is most likely to land in.
  const [variant, setVariant] = useState<Variant>("bitola");
  const [activeId, setActiveId] = useState(firstId);
  const refs = useRef<Record<string, HTMLElement | null>>({});
  const registerRef = useCallback((id: string, el: HTMLElement | null) => {
    refs.current[id] = el;
  }, []);

  const jump = (id: string) => {
    refs.current[id]?.scrollIntoView({ behavior: "smooth", block: "start" });
    setActiveId(id);
  };

  // Scroll-spy — highlight the chapter whose header sits closest above the
  // ~140px mark below the sticky top bar. Keyed on the active variant too:
  // flipping tabs swaps whole blocks in and out of `basics` and `melds`, so
  // every chapter below them moves and a stale `activeId` would highlight the
  // wrong entry until the next scroll event.
  useEffect(() => {
    const onScroll = () => {
      let best = firstId;
      for (const s of sections) {
        const el = refs.current[s.id];
        if (!el) continue;
        if (el.getBoundingClientRect().top - 140 <= 0) best = s.id;
      }
      setActiveId(best);
    };
    window.addEventListener("scroll", onScroll, { passive: true });
    onScroll();
    return () => window.removeEventListener("scroll", onScroll);
  }, [sections, firstId, variant]);

  return (
    <RulesProvider value={{ content, variant }}>
      {/* One provider for the whole page: every difference marker's tooltip
          shares its delay grouping instead of each mounting a provider root. */}
      <TooltipProvider delay={0}>
        <div
          className="mx-auto flex max-w-270 items-start gap-10 px-7 py-10"
          data-testid="rules-page"
        >
          <ChapterIndex activeId={activeId} onJump={jump} />

          <div className="min-w-0 flex-1">
            <RulesHero />

            <div
              className="flex flex-col gap-2 pb-2"
              data-testid="rules-variant-tabs"
              style={{ borderTop: "1px solid var(--border)", paddingTop: 20 }}
            >
              <span
                id={VARIANT_LABEL_ID}
                className="font-mono"
                style={{
                  fontSize: 11,
                  letterSpacing: 2.4,
                  textTransform: "uppercase",
                  color: "var(--brass-deep)",
                  fontWeight: 600,
                }}
              >
                {content.ui.variantLabel}
              </span>
              <Tabs
                value={variant}
                // Guarded, not cast: a value outside the union would hide every
                // scoped block and leave `basics` and `melds` with no steps at all.
                onValueChange={(value) => setVariant(normalizeVariant(value))}
              >
                <TabsList aria-labelledby={VARIANT_LABEL_ID}>
                  {VARIANTS.map((v) => (
                    <TabsTrigger key={v} value={v} data-testid={`rules-variant-${v}`}>
                      {variantLabel(t, v)}
                    </TabsTrigger>
                  ))}
                </TabsList>
              </Tabs>
            </div>

            {sections.map((s, i) => (
              <Chapter key={s.id} idx={i} section={s} registerRef={registerRef} />
            ))}
            <RulesFooter />
          </div>
        </div>
      </TooltipProvider>
    </RulesProvider>
  );
}
