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

const VARIANT_LABEL_ID = "rules-variant-label";
const SCROLL_SPY_OFFSET_PX = 140;

export function RulesPage() {
  const { t, i18n } = useTranslation();
  const content = getRulesContent(i18n.language);
  const sections = content.sections;
  const firstId = sections[0]?.id ?? "";

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

  useEffect(() => {
    const onScroll = () => {
      let best = firstId;
      for (const s of sections) {
        const el = refs.current[s.id];
        if (!el) continue;
        if (el.getBoundingClientRect().top - SCROLL_SPY_OFFSET_PX <= 0) best = s.id;
      }
      setActiveId(best);
    };
    window.addEventListener("scroll", onScroll, { passive: true });
    onScroll();
    return () => window.removeEventListener("scroll", onScroll);
  }, [sections, firstId, variant]);

  return (
    <RulesProvider value={{ content, variant }}>
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
                  fontSize: 10,
                  letterSpacing: "0.18em",
                  textTransform: "uppercase",
                  color: "var(--brass-deep)",
                  fontWeight: 600,
                }}
              >
                {content.ui.variantLabel}
              </span>
              <Tabs value={variant} onValueChange={(value) => setVariant(normalizeVariant(value))}>
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
