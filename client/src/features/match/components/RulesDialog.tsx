import { BookOpen } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";

import { Chapter } from "@/features/rules/components/Chapter";
import { ChapterIndex } from "@/features/rules/components/ChapterIndex";
import { getRulesContent } from "@/features/rules/content/rulesContent";
import { RulesProvider } from "@/features/rules/RulesContext";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/shared/components/ui/tabs";
import { useFocusTrap } from "@/shared/hooks/useFocusTrap";
import { variantLabel } from "@/shared/lib/roomLabels";
import { Z } from "@/shared/lib/zLayers";
import { normalizeVariant, type Variant, VARIANTS } from "@/shared/types/matchTypes";

import { ClassicButton } from "./overlay/ClassicButton";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";

interface RulesDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  variant?: Variant;
  stopAtTarget?: boolean;
  declarationsTransfer?: boolean;
  clockwise?: boolean;
}

const RULES_PANEL_HEIGHT = 620;
/**
 * In-game rules reference in the dark felt chrome, sharing its content with the standalone /rules page.
 * A sticky chapter index scroll-spies a single scrolling body.
 */
export function RulesDialog({
  open,
  onOpenChange,
  variant,
  stopAtTarget,
  declarationsTransfer,
  clockwise,
}: RulesDialogProps) {
  const { t, i18n } = useTranslation();
  const content = getRulesContent(i18n.language);
  const { sections, ui } = content;

  const matchVariant = normalizeVariant(variant);
  const [activeVariant, setActiveVariant] = useState<Variant>(matchVariant);

  useEffect(() => {
    if (open) setActiveVariant(matchVariant);
  }, [open, matchVariant]);

  const dialogRef = useFocusTrap<HTMLDivElement>({ onEscape: () => onOpenChange(false) });
  const scrollRef = useRef<HTMLDivElement>(null);
  const chapterRefs = useRef<Record<string, HTMLElement | null>>({});
  const [activeId, setActiveId] = useState(sections[0]?.id ?? "");

  useEffect(() => {
    if (!open) return;
    const el = scrollRef.current;
    if (!el) return;
    const onScroll = () => {
      const top = el.getBoundingClientRect().top;
      let best = sections[0]?.id ?? "";
      for (const s of sections) {
        const node = chapterRefs.current[s.id];
        if (!node) continue;
        if (node.getBoundingClientRect().top - top - 120 <= 0) best = s.id;
      }
      setActiveId(best);
    };
    el.addEventListener("scroll", onScroll, { passive: true });
    onScroll();
    return () => el.removeEventListener("scroll", onScroll);
  }, [open, sections, activeVariant]);

  if (!open) return null;

  const jump = (id: string) => {
    chapterRefs.current[id]?.scrollIntoView({ behavior: "smooth", block: "start" });
    setActiveId(id);
  };

  const dialog = (
    <RulesProvider value={{ content, variant: activeVariant, dense: true }}>
      <div
        className="felt-surface fixed inset-0"
        style={{ zIndex: Z.UTIL }}
        data-testid="rules-dialog"
      >
        <OverlayBackdrop dim="prompt">
          <div
            ref={dialogRef}
            role="dialog"
            aria-modal="true"
            aria-labelledby="rules-dialog-title"
            className="motion-safe:animate-in motion-safe:zoom-in-95 motion-safe:duration-150"
          >
            <ClassicPanel
              size="reference"
              bodyPadding={0}
              style={{ height: RULES_PANEL_HEIGHT }}
              title={
                <span id="rules-dialog-title" className="inline-flex items-center gap-2.5">
                  <BookOpen size={20} className="text-brass" aria-hidden="true" />
                  {ui.ovTitle}
                </span>
              }
            >
              <div className="flex h-full min-h-0 flex-col">
                <div className="flex min-h-0 flex-1">
                  <ChapterIndex
                    activeId={activeId}
                    onJump={jump}
                    title={ui.ovChapters}
                    testIdPrefix="rules-toc"
                    className="felt-controls border-brass-25 hidden w-50 shrink-0 border-r bg-black/30 px-2.5 py-3.5 md:flex"
                  />

                  <div
                    ref={scrollRef}
                    className="min-w-0 flex-1 overflow-y-auto px-4 pt-1 pb-5 md:px-6.5 md:pb-5.5"
                  >
                    <Tabs
                      value={activeVariant}
                      onValueChange={(value) => setActiveVariant(normalizeVariant(value))}
                      className="gap-0"
                    >
                      <div className="flex items-center gap-2 pt-3">
                        <span className="text-brass text-2xs font-semibold tracking-caps uppercase opacity-70">
                          {ui.variantLabel}
                        </span>
                        <TabsList
                          activateOnFocus
                          aria-label={ui.variantLabel}
                          className="felt-controls"
                          data-testid="rules-dialog-variant-tabs"
                        >
                          {VARIANTS.map((v) => (
                            <TabsTrigger
                              key={v}
                              value={v}
                              className="px-3"
                              data-testid={`rules-dialog-variant-${v}`}
                            >
                              {variantLabel(t, v)}
                            </TabsTrigger>
                          ))}
                        </TabsList>
                      </div>
                      {stopAtTarget === true && (
                        <TableRuleNote testId="rules-dialog-stop-at-target-note">
                          {t("match.rules.stopAtTargetNote")}
                        </TableRuleNote>
                      )}
                      {declarationsTransfer === false && (
                        <TableRuleNote testId="rules-dialog-no-declarations-transfer-note">
                          {t("match.rules.noDeclarationsTransferNote")}
                        </TableRuleNote>
                      )}
                      {clockwise === true && (
                        <TableRuleNote testId="rules-dialog-clockwise-note">
                          {t("match.rules.clockwiseNote")}
                        </TableRuleNote>
                      )}
                      {VARIANTS.map((v) => (
                        <TabsContent key={v} value={v}>
                          <RulesProvider value={{ content, variant: v, dense: true }}>
                            {sections.map((sec, i) => (
                              <Chapter
                                key={sec.id}
                                idx={i}
                                section={sec}
                                registerRef={(id, el) => {
                                  chapterRefs.current[id] = el;
                                }}
                              />
                            ))}
                          </RulesProvider>
                        </TabsContent>
                      ))}
                    </Tabs>
                  </div>
                </div>

                <div className="border-brass-25 flex shrink-0 items-center justify-end border-t bg-black/30 px-5 py-3">
                  <ClassicButton
                    variant="primary"
                    onClick={() => onOpenChange(false)}
                    data-testid="rules-dialog-close"
                  >
                    {ui.ovClose}
                  </ClassicButton>
                </div>
              </div>
            </ClassicPanel>
          </div>
        </OverlayBackdrop>
      </div>
    </RulesProvider>
  );

  return createPortal(dialog, document.body);
}

/** A callout naming a room rule this table plays differently from the chapters below. */
function TableRuleNote({ testId, children }: { testId: string; children: React.ReactNode }) {
  return (
    <div
      className="bg-brass/10 border-brass-40 text-ink-light mt-3 rounded-lg border px-3 py-2.5 text-xs leading-normal"
      data-testid={testId}
    >
      {children}
    </div>
  );
}
