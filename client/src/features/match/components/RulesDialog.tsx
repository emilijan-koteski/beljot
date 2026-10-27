import { BookOpen } from "lucide-react";
import { createContext, useContext, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";

import { DiffMarker } from "@/features/rules/components/DiffMarker";
import { getRulesContent } from "@/features/rules/content/rulesContent";
import type {
  CardRow,
  Declaration,
  RuleBlock,
  RulesContent,
  RuleSection,
} from "@/features/rules/content/types";
import { isVisibleFor, visibleFor } from "@/features/rules/content/visibility";
import { TooltipProvider } from "@/shared/components/ui/tooltip";
import { useFocusTrap } from "@/shared/hooks/useFocusTrap";
import { variantLabel } from "@/shared/lib/roomLabels";
import { Z } from "@/shared/lib/zLayers";
import { normalizeVariant, type Variant, VARIANTS } from "@/shared/types/matchTypes";

import { ClassicButton } from "./overlay/ClassicButton";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";

interface RulesDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  variant?: Variant;
  stopAtTarget?: boolean;
  declarationsTransfer?: boolean;
  clockwise?: boolean;
}

const PANEL_ID = "rules-dialog-panel";
const tabId = (v: Variant) => `rules-dialog-tab-${v}`;

const PANEL_BG = "linear-gradient(180deg, rgba(30,60,40,0.98) 0%, rgba(14,40,24,0.98) 100%)";
const BRASS = "var(--brass)";
const INK = "var(--ink-light)";
const TEXT = "rgba(245,242,232,0.88)";
const TEXT_DIM = "rgba(245,242,232,0.66)";
const GREEN = "var(--accent-light)";

type DarkRulesValue = { content: RulesContent; variant: Variant };
const DarkRulesCtx = createContext<DarkRulesValue | null>(null);
const useDarkRules = () => (useContext(DarkRulesCtx) as DarkRulesValue).content;
const useDarkVariant = () => (useContext(DarkRulesCtx) as DarkRulesValue).variant;

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
    <TooltipProvider delay={0}>
      <div className="fixed inset-0" style={{ zIndex: Z.UTIL }} data-testid="rules-dialog">
        <OverlayBackdrop dim={0.55}>
          <div
            ref={dialogRef}
            role="dialog"
            aria-modal="true"
            aria-labelledby="rules-dialog-title"
            className="motion-safe:animate-in motion-safe:zoom-in-95 motion-safe:duration-150 flex flex-col overflow-hidden"
            style={{
              width: 860,
              maxWidth: "92vw",
              height: 620,
              maxHeight: "88vh",
              borderRadius: 14,
              background: PANEL_BG,
              border: "1px solid var(--brass-55)",
              boxShadow:
                "var(--shadow-felt-xl), 0 0 0 4px var(--brass-10), inset 0 1px 0 var(--brass-25)",
              color: INK,
            }}
          >
            <div
              className="flex shrink-0 items-center gap-3"
              style={{
                padding: "16px 20px 14px",
                borderBottom: "1px solid var(--brass-25)",
              }}
            >
              <BookOpen size={20} style={{ color: BRASS }} aria-hidden="true" />
              <div className="flex flex-col">
                <span
                  style={{
                    fontSize: 10,
                    letterSpacing: "0.18em",
                    textTransform: "uppercase",
                    color: BRASS,
                    opacity: 0.85,
                  }}
                >
                  {ui.ovReference}
                </span>
                <span
                  id="rules-dialog-title"
                  style={{
                    fontSize: 18,
                    fontWeight: 600,
                    color: INK,
                    marginTop: 1,
                  }}
                >
                  {ui.ovTitle}
                </span>
              </div>
            </div>

            <div className="flex min-h-0 flex-1">
              <nav
                className="hidden shrink-0 flex-col gap-0.5 md:flex"
                style={{
                  width: 200,
                  borderRight: "1px solid var(--brass-25)",
                  padding: "14px 10px",
                  background: "rgba(10,22,15,0.30)",
                }}
              >
                <div
                  style={{
                    padding: "0 10px 10px",
                    fontSize: 10,
                    letterSpacing: "0.18em",
                    textTransform: "uppercase",
                    color: BRASS,
                    opacity: 0.7,
                    fontWeight: 600,
                  }}
                >
                  {ui.ovChapters}
                </div>
                {sections.map((s, i) => {
                  const on = s.id === activeId;
                  return (
                    <button
                      key={s.id}
                      type="button"
                      onClick={() => jump(s.id)}
                      data-testid={`rules-toc-${s.id}`}
                      className="grid cursor-pointer grid-cols-[26px_1fr] items-center rounded-sm text-left transition-colors"
                      style={{
                        padding: "8px 10px",
                        background: on ? "var(--brass-soft)" : "transparent",
                        border: on ? "1px solid var(--brass-40)" : "1px solid transparent",
                        color: on ? INK : TEXT_DIM,
                      }}
                    >
                      <span
                        style={{
                          fontSize: 10,
                          color: on ? BRASS : TEXT_DIM,
                          letterSpacing: "0.05em",
                          fontWeight: 600,
                        }}
                      >
                        {String(i + 1).padStart(2, "0")}
                      </span>
                      <span style={{ fontSize: 14, fontWeight: on ? 600 : 500 }}>{s.label}</span>
                    </button>
                  );
                })}
              </nav>

              <div
                ref={scrollRef}
                className="min-w-0 flex-1 overflow-y-auto px-4 pt-1 pb-5 md:px-6.5 md:pb-5.5"
              >
                <div
                  className="flex items-center gap-2 pt-3"
                  data-testid="rules-dialog-variant-tabs"
                  role="tablist"
                  aria-label={ui.variantLabel}
                >
                  <span
                    style={{
                      fontSize: 10,
                      letterSpacing: "0.18em",
                      textTransform: "uppercase",
                      color: BRASS,
                      opacity: 0.7,
                      fontWeight: 600,
                    }}
                  >
                    {ui.variantLabel}
                  </span>
                  {VARIANTS.map((v, i) => {
                    const on = v === activeVariant;
                    return (
                      <button
                        key={v}
                        type="button"
                        role="tab"
                        id={tabId(v)}
                        aria-selected={on}
                        aria-controls={PANEL_ID}
                        tabIndex={on ? 0 : -1}
                        onKeyDown={(e) => {
                          const step = e.key === "ArrowRight" ? 1 : e.key === "ArrowLeft" ? -1 : 0;
                          if (step === 0) return;
                          e.preventDefault();
                          const next = normalizeVariant(
                            VARIANTS[(i + step + VARIANTS.length) % VARIANTS.length],
                          );
                          setActiveVariant(next);
                          e.currentTarget.parentElement
                            ?.querySelector<HTMLButtonElement>(
                              `[data-testid="rules-dialog-variant-${next}"]`,
                            )
                            ?.focus();
                        }}
                        onClick={() => setActiveVariant(v)}
                        data-testid={`rules-dialog-variant-${v}`}
                        className="cursor-pointer rounded-sm transition-colors"
                        style={{
                          padding: "5px 11px",
                          fontSize: 12,
                          fontWeight: on ? 600 : 500,
                          background: on ? "var(--brass-soft)" : "transparent",
                          border: on ? "1px solid var(--brass-40)" : "1px solid var(--brass-soft)",
                          color: on ? INK : TEXT_DIM,
                        }}
                      >
                        {variantLabel(t, v)}
                      </button>
                    );
                  })}
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
                <DarkRulesCtx.Provider value={{ content, variant: activeVariant }}>
                  <div id={PANEL_ID} role="tabpanel" aria-labelledby={tabId(activeVariant)}>
                    {sections.map((s, i) => (
                      <DarkChapter
                        key={s.id}
                        idx={i}
                        section={s}
                        registerRef={(el) => {
                          chapterRefs.current[s.id] = el;
                        }}
                      />
                    ))}
                  </div>
                </DarkRulesCtx.Provider>
              </div>
            </div>

            <div
              className="flex shrink-0 items-center justify-end"
              style={{
                padding: "12px 20px",
                borderTop: "1px solid var(--brass-25)",
                background: "rgba(10,22,15,0.30)",
              }}
            >
              <ClassicButton
                variant="primary"
                onClick={() => onOpenChange(false)}
                data-testid="rules-dialog-close"
              >
                {ui.ovClose}
              </ClassicButton>
            </div>
          </div>
        </OverlayBackdrop>
      </div>
    </TooltipProvider>
  );

  return createPortal(dialog, document.body);
}

/** A callout naming a room rule this table plays differently from the chapters below. */
function TableRuleNote({ testId, children }: { testId: string; children: React.ReactNode }) {
  return (
    <div
      className="mt-3 rounded-lg px-3 py-2.5"
      style={{
        background: "var(--brass-10)",
        border: "1px solid var(--brass-40)",
        fontSize: 12,
        lineHeight: 1.5,
        color: INK,
      }}
      data-testid={testId}
    >
      {children}
    </div>
  );
}

function DarkChapter({
  idx,
  section,
  registerRef,
}: {
  idx: number;
  section: RuleSection;
  registerRef: (el: HTMLElement | null) => void;
}) {
  return (
    <section
      ref={registerRef}
      style={{
        scrollMarginTop: 8,
        display: "flex",
        flexDirection: "column",
        gap: 14,
        padding: "22px 0 20px",
        borderTop: idx === 0 ? "none" : "1px solid var(--brass-soft)",
      }}
    >
      <header style={{ display: "flex", flexDirection: "column", gap: 4 }}>
        <span
          style={{
            fontSize: 10,
            letterSpacing: "0.18em",
            textTransform: "uppercase",
            color: BRASS,
            fontWeight: 600,
          }}
        >
          {String(idx + 1).padStart(2, "0")} &nbsp; {section.label}
        </span>
        <h3
          style={{
            margin: 0,
            fontWeight: 600,
            fontSize: 24,
            color: INK,
          }}
        >
          {section.title}
        </h3>
        <p style={{ margin: "2px 0 0", fontSize: 14, lineHeight: 1.6, color: TEXT_DIM }}>
          {section.lede}
        </p>
      </header>
      <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
        {section.blocks.map((b, i) => (
          <DarkBlock key={i} block={b} />
        ))}
      </div>
    </section>
  );
}

/**
 * The dark-felt twin of `RuleBlock`. The variant filter and the difference
 * marker have to live in BOTH switches: added to only one of them, they would
 * silently vanish in whichever theme was missed.
 */
function DarkBlock({ block }: { block: RuleBlock }) {
  const { ui } = useDarkRules();
  const activeVariant = useDarkVariant();

  if (!isVisibleFor(block, activeVariant)) return null;

  const marker = block.otherVariantNote ? (
    <DiffMarker note={block.otherVariantNote} label={ui.diffLabel} tone="dark" />
  ) : null;

  switch (block.kind) {
    case "p":
      return (
        <p style={{ margin: 0, fontSize: 14, lineHeight: 1.6, color: TEXT }}>
          {block.text}
          {marker}
        </p>
      );

    case "rule":
      return (
        <div
          style={{
            padding: "11px 14px",
            background: "rgba(20,46,28,0.55)",
            border: "1px solid var(--brass-25)",
            borderLeft: `3px solid ${GREEN}`,
            borderRadius: 8,
          }}
        >
          <div
            style={{
              fontSize: 14,
              fontWeight: 600,
              color: INK,
              marginBottom: 3,
            }}
          >
            {block.title}
            {marker}
          </div>
          <div style={{ fontSize: 12, lineHeight: 1.6, color: TEXT_DIM }}>{block.text}</div>
        </div>
      );

    case "note":
      return (
        <div
          style={{
            padding: "10px 12px",
            background: "var(--brass-10)",
            border: "1px dashed var(--brass-40)",
            borderRadius: 8,
            display: "flex",
            gap: 10,
            alignItems: "flex-start",
          }}
        >
          <span
            style={{
              fontSize: 10,
              letterSpacing: "0.18em",
              textTransform: "uppercase",
              color: BRASS,
              fontWeight: 700,
              flexShrink: 0,
              paddingTop: 1,
            }}
          >
            {ui.noteLabel}
          </span>
          <span style={{ fontSize: 12, lineHeight: 1.6, color: TEXT_DIM }}>
            {block.text}
            {marker}
          </span>
        </div>
      );

    case "steps": {
      const items = visibleFor(block.items, activeVariant);
      return (
        <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
          {marker ? (
            <div style={{ display: "flex", justifyContent: "flex-end" }}>{marker}</div>
          ) : null}
          <ol
            style={{
              margin: 0,
              padding: 0,
              listStyle: "none",
              display: "flex",
              flexDirection: "column",
              gap: 8,
            }}
          >
            {items.map((it, i) => (
              <li
                key={i}
                style={{
                  display: "grid",
                  gridTemplateColumns: "32px 1fr",
                  gap: 12,
                  padding: "10px 12px",
                  background: "rgba(20,46,28,0.45)",
                  border: "1px solid var(--brass-soft)",
                  borderRadius: 8,
                }}
              >
                <span
                  style={{
                    display: "inline-flex",
                    alignItems: "center",
                    justifyContent: "center",
                    width: 24,
                    height: 24,
                    borderRadius: "var(--radius-sm)",
                    background: "var(--brass-soft)",
                    border: "1px solid var(--brass-40)",
                    fontSize: 12,
                    fontWeight: 700,
                    color: BRASS,
                    letterSpacing: "0.05em",
                  }}
                >
                  {String(i + 1).padStart(2, "0")}
                </span>
                <div>
                  <div
                    style={{
                      fontSize: 14,
                      fontWeight: 600,
                      color: INK,
                      marginBottom: 2,
                    }}
                  >
                    {it.t}
                    {it.otherVariantNote ? (
                      <DiffMarker note={it.otherVariantNote} label={ui.diffLabel} tone="dark" />
                    ) : null}
                  </div>
                  <div style={{ fontSize: 12, lineHeight: 1.6, color: TEXT_DIM }}>{it.d}</div>
                </div>
              </li>
            ))}
          </ol>
        </div>
      );
    }

    case "cards":
      return <DarkCardLadders />;

    case "melds":
      return <DarkMeldsGrid />;

    default:
      return null;
  }
}

function DarkCardLadders() {
  const { cardsTrump, cardsPlain, ui } = useDarkRules();
  return (
    <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
      <DarkCardLadder
        title={ui.ladderTrumpTitle}
        eyebrow={ui.ladderTrumpEyebrow}
        accent={GREEN}
        rows={cardsTrump}
        colPts={ui.colPoints}
      />
      <DarkCardLadder
        title={ui.ladderPlainTitle}
        eyebrow={ui.ladderPlainEyebrow}
        accent={BRASS}
        rows={cardsPlain}
        colPts={ui.colPoints}
      />
    </div>
  );
}

function DarkCardLadder({
  title,
  eyebrow,
  accent,
  rows,
  colPts,
}: {
  title: string;
  eyebrow: string;
  accent: string;
  rows: CardRow[];
  colPts: string;
}) {
  return (
    <div
      style={{
        background: "rgba(10,22,15,0.45)",
        border: "1px solid var(--brass-25)",
        borderRadius: 10,
        overflow: "hidden",
      }}
    >
      <header style={{ padding: "10px 12px 8px", borderBottom: "1px solid var(--brass-soft)" }}>
        <div
          style={{
            fontSize: 10,
            letterSpacing: "0.18em",
            textTransform: "uppercase",
            color: accent,
            fontWeight: 700,
          }}
        >
          {eyebrow}
        </div>
        <div
          style={{
            fontSize: 14,
            fontWeight: 600,
            color: INK,
            marginTop: 1,
          }}
        >
          {title}
        </div>
      </header>
      <div style={{ padding: "4px 0" }}>
        {rows.map((r, i) => {
          const top = r.strength === 1;
          return (
            <div
              key={r.rank}
              style={{
                display: "grid",
                gridTemplateColumns: "34px 1fr 40px",
                alignItems: "center",
                padding: "6px 12px",
                borderTop: i === 0 ? "none" : "1px solid var(--brass-10)",
                background: top
                  ? "color-mix(in srgb, var(--accent-light) 6%, transparent)"
                  : "transparent",
              }}
            >
              <span
                style={{
                  display: "inline-flex",
                  alignItems: "center",
                  justifyContent: "center",
                  minWidth: 26,
                  height: 26,
                  padding: "0 4px",
                  borderRadius: 4,
                  background: top
                    ? "color-mix(in srgb, var(--accent-light) 10%, transparent)"
                    : "rgba(245,242,232,0.06)",
                  border: `1px solid ${top ? `color-mix(in srgb, ${accent} 53%, transparent)` : "rgba(245,242,232,0.18)"}`,
                  fontWeight: 700,
                  fontSize: 12,
                  color: top ? accent : INK,
                }}
              >
                {r.rank}
              </span>
              <span
                style={{
                  marginLeft: 10,
                  fontSize: 12,
                  color: top ? INK : TEXT,
                  fontWeight: top ? 600 : 500,
                }}
              >
                {r.name}
                {r.note ? (
                  <span style={{ color: TEXT_DIM, fontStyle: "italic" }}> · {r.note}</span>
                ) : null}
              </span>
              <span
                aria-label={colPts}
                className="tabular-nums"
                style={{
                  textAlign: "right",
                  fontSize: 12,
                  fontWeight: 600,
                  color: r.pts > 0 ? INK : TEXT_DIM,
                }}
              >
                {r.pts}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function DarkMeldsGrid() {
  const { declarations, ui } = useDarkRules();
  return (
    <div className="grid grid-cols-1 gap-2 md:grid-cols-2">
      {declarations.map((m) => (
        <DarkMeldChip key={m.id} meld={m} ptsLabel={ui.pts} kindLabel={ui.meldKinds[m.kind]} />
      ))}
    </div>
  );
}

function DarkMeldChip({
  meld,
  ptsLabel,
  kindLabel,
}: {
  meld: Declaration;
  ptsLabel: string;
  kindLabel: string;
}) {
  const jackpot = meld.tier === 2;
  const accent = jackpot ? GREEN : BRASS;
  return (
    <div
      data-testid={`rules-meld-${meld.id}`}
      style={{
        padding: "10px 12px",
        background: jackpot
          ? "color-mix(in srgb, var(--accent-light) 6%, transparent)"
          : "rgba(10,22,15,0.45)",
        border: `1px solid ${jackpot ? "color-mix(in srgb, var(--accent-light) 35%, transparent)" : "var(--brass-25)"}`,
        borderRadius: 8,
        display: "flex",
        flexDirection: "column",
        gap: 4,
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <span
          style={{
            fontSize: 14,
            fontWeight: 600,
            color: INK,
            flex: "1 1 auto",
            minWidth: 0,
            lineHeight: 1.2,
          }}
        >
          {meld.name}
        </span>
        <span className="inline-flex shrink-0 items-baseline gap-1">
          <span className="tabular-nums" style={{ fontSize: 18, fontWeight: 700, color: accent }}>
            +{meld.pts}
          </span>
          <span style={{ fontSize: 10, color: TEXT_DIM }}>{ptsLabel}</span>
        </span>
      </div>
      <div
        style={{
          fontSize: 10,
          letterSpacing: "0.18em",
          textTransform: "uppercase",
          color: jackpot ? accent : BRASS,
          fontWeight: 600,
        }}
      >
        {kindLabel}
      </div>
      <div style={{ fontSize: 12, lineHeight: 1.4, color: TEXT_DIM }}>{meld.summary}</div>
    </div>
  );
}
