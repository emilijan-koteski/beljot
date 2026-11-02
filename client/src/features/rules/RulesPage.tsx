import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { Chapter } from "@/features/rules/components/Chapter";
import { ChapterChips } from "@/features/rules/components/ChapterChips";
import { ChapterNav } from "@/features/rules/components/ChapterNav";
import { RulesFooter } from "@/features/rules/components/RulesFooter";
import { RulesHero } from "@/features/rules/components/RulesHero";
import { getRulesContent } from "@/features/rules/content/rulesContent";
import { RulesProvider } from "@/features/rules/RulesContext";
import { Segmented } from "@/shared/components/ui/segmented";
import { useLayoutBand } from "@/shared/hooks/useLayoutBand";
import { variantLabel } from "@/shared/lib/roomLabels";
import { cn } from "@/shared/lib/utils";
import { type Variant, VARIANTS } from "@/shared/types/matchTypes";

const SCROLL_SPY_OFFSET_PX = 140;

/**
 * The rules of the card game in six short chapters. Phones and tablets pin chapter chips under the top bar;
 * desktops keep the chapters and the variant switch in a sticky left column, like Settings.
 */
export function RulesPage() {
  const { t, i18n } = useTranslation();
  const band = useLayoutBand();
  const content = getRulesContent(i18n.language);
  const { sections, ui } = content;
  const firstId = sections[0]?.id ?? "";

  const [variant, setVariant] = useState<Variant>("bitola");
  const [activeId, setActiveId] = useState<string>(firstId);
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

  const variantSwitch = (
    <Segmented<Variant>
      value={variant}
      onValueChange={setVariant}
      ariaLabel={ui.variantLabel}
      testId="rules-variant"
      className={band === "tablet" ? "w-72" : undefined}
      options={VARIANTS.map((v) => ({ value: v, label: variantLabel(t, v) }))}
    />
  );

  const chapters = (
    <>
      {sections.map((s) => (
        <Chapter key={s.id} section={s} registerRef={registerRef} />
      ))}
      <RulesFooter />
    </>
  );

  if (band === "desktop") {
    return (
      <RulesProvider value={{ content, variant }}>
        <div
          className="mx-auto grid w-full max-w-256 grid-cols-[13.75rem_minmax(0,1fr)] items-start gap-10 px-7 pt-8 pb-16"
          data-testid="rules-page"
        >
          <aside className="sticky top-21 flex flex-col">
            <ChapterNav activeId={activeId} onJump={jump} />
            <div className="border-border my-3 border-t" />
            <span className="text-ink-mute px-3 pb-1.5 text-xs font-medium">{ui.variantLabel}</span>
            {variantSwitch}
          </aside>
          <div className="flex max-w-176 flex-col gap-10">
            <RulesHero />
            {chapters}
          </div>
        </div>
      </RulesProvider>
    );
  }

  const column = band === "tablet" ? "mx-auto w-full max-w-160 px-6" : "px-4";
  return (
    <RulesProvider value={{ content, variant }}>
      <div className="pb-10" data-testid="rules-page">
        <div className={cn(column, "flex flex-col gap-4 pt-5 pb-4 md:gap-5 md:pt-6")}>
          <RulesHero />
          {variantSwitch}
        </div>
        <ChapterChips activeId={activeId} onJump={jump} innerClassName={column} />
        <div className={cn(column, "flex flex-col gap-8 pt-5 md:gap-9 md:pt-6")}>{chapters}</div>
      </div>
    </RulesProvider>
  );
}
