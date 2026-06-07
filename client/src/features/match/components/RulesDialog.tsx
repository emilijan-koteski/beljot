import { BookOpen } from "lucide-react";
import { createContext, useContext, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";

import { getRulesContent } from "@/features/rules/content/rulesContent";
import type {
  CardRow,
  Declaration,
  RuleBlock,
  RulesContent,
  RuleSection,
} from "@/features/rules/content/types";
import { useFocusTrap } from "@/shared/hooks/useFocusTrap";
import { Z } from "@/shared/lib/zLayers";

import { ClassicButton } from "./overlay/ClassicButton";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";

interface RulesDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

// Felt-dark palette — matches the bidding / belot / score overlays.
const PANEL_BG = "linear-gradient(180deg, rgba(30,60,40,0.98) 0%, rgba(14,40,24,0.98) 100%)";
const BRASS = "#c9a876";
const INK = "#f5f2e8";
const TEXT = "rgba(245,242,232,0.88)";
const TEXT_DIM = "rgba(245,242,232,0.66)";
const TEXT_FAINT = "rgba(245,242,232,0.5)";
const GREEN = "#00e5a0"; // --turn-lime — the in-game accent

const DarkRulesCtx = createContext<RulesContent | null>(null);
const useDarkRules = () => useContext(DarkRulesCtx) as RulesContent;

/**
 * In-game rules reference. Same content module as the standalone /rules page
 * (so the two never drift) but rendered in the dark felt chrome the other
 * in-game overlays use. A sticky chapter index scroll-spies a single scrolling
 * body. No language switch — the locale is the one chosen in game settings.
 */
export function RulesDialog({ open, onOpenChange }: RulesDialogProps) {
  const { i18n } = useTranslation();
  const content = getRulesContent(i18n.language);
  const { sections, ui } = content;

  const dialogRef = useFocusTrap<HTMLDivElement>({ onEscape: () => onOpenChange(false) });
  const scrollRef = useRef<HTMLDivElement>(null);
  const chapterRefs = useRef<Record<string, HTMLElement | null>>({});
  const [activeId, setActiveId] = useState(sections[0]?.id ?? "");

  // Scroll-spy within the dialog's own scroll container.
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
  }, [open, sections]);

  if (!open) return null;

  const jump = (id: string) => {
    chapterRefs.current[id]?.scrollIntoView({ behavior: "smooth", block: "start" });
    setActiveId(id);
  };

  const dialog = (
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
            border: "1px solid rgba(201,168,118,0.55)",
            boxShadow:
              "0 20px 60px rgba(0,0,0,0.7), 0 0 0 4px rgba(201,168,118,0.12), inset 0 1px 0 rgba(201,168,118,0.22)",
            color: INK,
          }}
        >
          {/* Header */}
          <div
            className="flex shrink-0 items-center gap-3"
            style={{ padding: "16px 20px 14px", borderBottom: "1px solid rgba(201,168,118,0.22)" }}
          >
            <BookOpen size={18} style={{ color: BRASS }} aria-hidden="true" />
            <div className="flex flex-col">
              <span
                style={{
                  fontFamily: "var(--font-body)",
                  fontSize: 11,
                  letterSpacing: 2,
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
                  fontFamily: "var(--font-body)",
                  fontSize: 19,
                  fontWeight: 600,
                  letterSpacing: 0.2,
                  color: INK,
                  marginTop: 1,
                }}
              >
                {ui.ovTitle}
              </span>
            </div>
          </div>

          {/* Body: TOC rail + scrolling content. On phones the rail is hidden
              and the content fills the full width (everything scrolls in one
              column). */}
          <div className="flex min-h-0 flex-1">
            <nav
              className="hidden shrink-0 flex-col gap-0.5 md:flex"
              style={{
                width: 200,
                borderRight: "1px solid rgba(201,168,118,0.22)",
                padding: "14px 10px",
                background: "rgba(10,22,15,0.30)",
              }}
            >
              <div
                style={{
                  padding: "0 10px 10px",
                  fontFamily: "var(--font-body)",
                  fontSize: 10,
                  letterSpacing: 2,
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
                    className="grid cursor-pointer grid-cols-[26px_1fr] items-center rounded-[7px] text-left transition-colors"
                    style={{
                      padding: "8px 10px",
                      background: on ? "rgba(201,168,118,0.18)" : "transparent",
                      border: on ? "1px solid rgba(201,168,118,0.4)" : "1px solid transparent",
                      color: on ? INK : "rgba(245,242,232,0.75)",
                    }}
                  >
                    <span
                      style={{
                        fontFamily: "var(--font-body)",
                        fontSize: 10.5,
                        color: on ? BRASS : TEXT_FAINT,
                        letterSpacing: 0.5,
                        fontWeight: 600,
                      }}
                    >
                      {String(i + 1).padStart(2, "0")}
                    </span>
                    <span style={{ fontSize: 13, fontWeight: on ? 600 : 500 }}>{s.label}</span>
                  </button>
                );
              })}
            </nav>

            <div
              ref={scrollRef}
              className="min-w-0 flex-1 overflow-y-auto px-4 pt-1 pb-5 md:px-6.5 md:pb-5.5"
            >
              <DarkRulesCtx.Provider value={content}>
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
              </DarkRulesCtx.Provider>
            </div>
          </div>

          {/* Footer */}
          <div
            className="flex shrink-0 items-center justify-end"
            style={{
              padding: "12px 20px",
              borderTop: "1px solid rgba(201,168,118,0.22)",
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
  );

  return createPortal(dialog, document.body);
}

// ── Dark-themed renderers ──────────────────────────────────────────────────

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
        borderTop: idx === 0 ? "none" : "1px solid rgba(201,168,118,0.16)",
      }}
    >
      <header style={{ display: "flex", flexDirection: "column", gap: 4 }}>
        <span
          style={{
            fontFamily: "var(--font-body)",
            fontSize: 10,
            letterSpacing: 2,
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
            fontFamily: "var(--font-body)",
            fontWeight: 600,
            fontSize: 22,
            letterSpacing: 0.2,
            color: INK,
          }}
        >
          {section.title}
        </h3>
        <p style={{ margin: "2px 0 0", fontSize: 13.5, lineHeight: 1.55, color: TEXT_DIM }}>
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

function DarkBlock({ block }: { block: RuleBlock }) {
  const { ui } = useDarkRules();

  switch (block.kind) {
    case "p":
      return (
        <p style={{ margin: 0, fontSize: 13.5, lineHeight: 1.6, color: TEXT }}>{block.text}</p>
      );

    case "rule":
      return (
        <div
          style={{
            padding: "11px 14px",
            background: "rgba(20,46,28,0.55)",
            border: "1px solid rgba(201,168,118,0.25)",
            borderLeft: `3px solid ${GREEN}`,
            borderRadius: 8,
          }}
        >
          <div
            style={{
              fontFamily: "var(--font-body)",
              fontSize: 13.5,
              fontWeight: 600,
              color: INK,
              marginBottom: 3,
            }}
          >
            {block.title}
          </div>
          <div style={{ fontSize: 12.5, lineHeight: 1.55, color: TEXT_DIM }}>{block.text}</div>
        </div>
      );

    case "note":
      return (
        <div
          style={{
            padding: "10px 12px",
            background: "rgba(201,168,118,0.10)",
            border: "1px dashed rgba(201,168,118,0.40)",
            borderRadius: 8,
            display: "flex",
            gap: 10,
            alignItems: "flex-start",
          }}
        >
          <span
            style={{
              fontFamily: "var(--font-body)",
              fontSize: 9.5,
              letterSpacing: 2,
              textTransform: "uppercase",
              color: BRASS,
              fontWeight: 700,
              flexShrink: 0,
              paddingTop: 1,
            }}
          >
            {ui.noteLabel}
          </span>
          <span style={{ fontSize: 12.5, lineHeight: 1.55, color: TEXT_DIM }}>{block.text}</span>
        </div>
      );

    case "steps":
      return (
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
          {block.items.map((it, i) => (
            <li
              key={i}
              style={{
                display: "grid",
                gridTemplateColumns: "32px 1fr",
                gap: 12,
                padding: "10px 12px",
                background: "rgba(20,46,28,0.45)",
                border: "1px solid rgba(201,168,118,0.18)",
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
                  borderRadius: 6,
                  background: "rgba(201,168,118,0.18)",
                  border: "1px solid rgba(201,168,118,0.35)",
                  fontFamily: "var(--font-body)",
                  fontSize: 11,
                  fontWeight: 700,
                  color: BRASS,
                  letterSpacing: 0.5,
                }}
              >
                {String(i + 1).padStart(2, "0")}
              </span>
              <div>
                <div
                  style={{
                    fontFamily: "var(--font-body)",
                    fontSize: 13.5,
                    fontWeight: 600,
                    color: INK,
                    marginBottom: 2,
                  }}
                >
                  {it.t}
                </div>
                <div style={{ fontSize: 12.5, lineHeight: 1.55, color: TEXT_DIM }}>{it.d}</div>
              </div>
            </li>
          ))}
        </ol>
      );

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
        border: "1px solid rgba(201,168,118,0.22)",
        borderRadius: 10,
        overflow: "hidden",
      }}
    >
      <header
        style={{ padding: "10px 12px 8px", borderBottom: "1px solid rgba(201,168,118,0.18)" }}
      >
        <div
          style={{
            fontFamily: "var(--font-body)",
            fontSize: 9.5,
            letterSpacing: 1.8,
            textTransform: "uppercase",
            color: accent,
            fontWeight: 700,
          }}
        >
          {eyebrow}
        </div>
        <div
          style={{
            fontFamily: "var(--font-body)",
            fontSize: 13,
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
                borderTop: i === 0 ? "none" : "1px solid rgba(201,168,118,0.10)",
                background: top ? "rgba(0,229,160,0.06)" : "transparent",
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
                  borderRadius: 5,
                  background: top ? "rgba(0,229,160,0.10)" : "rgba(245,242,232,0.06)",
                  border: `1px solid ${top ? `${accent}88` : "rgba(245,242,232,0.18)"}`,
                  fontFamily: "var(--font-body)",
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
                  fontSize: 12.5,
                  color: top ? INK : "rgba(245,242,232,0.78)",
                  fontWeight: top ? 600 : 500,
                }}
              >
                {r.name}
                {r.note ? (
                  <span style={{ color: TEXT_FAINT, fontStyle: "italic" }}> · {r.note}</span>
                ) : null}
              </span>
              <span
                aria-label={colPts}
                className="tabular-nums"
                style={{
                  textAlign: "right",
                  fontFamily: "var(--font-body)",
                  fontSize: 12.5,
                  fontWeight: 600,
                  color: r.pts > 0 ? INK : "rgba(245,242,232,0.4)",
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
        background: jackpot ? "rgba(0,229,160,0.06)" : "rgba(10,22,15,0.45)",
        border: `1px solid ${jackpot ? "rgba(0,229,160,0.35)" : "rgba(201,168,118,0.22)"}`,
        borderRadius: 8,
        display: "flex",
        flexDirection: "column",
        gap: 4,
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <span
          style={{
            fontFamily: "var(--font-body)",
            fontSize: 13.5,
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
          <span
            className="tabular-nums"
            style={{ fontFamily: "var(--font-body)", fontSize: 17, fontWeight: 700, color: accent }}
          >
            +{meld.pts}
          </span>
          <span style={{ fontSize: 10, color: TEXT_FAINT }}>{ptsLabel}</span>
        </span>
      </div>
      <div
        style={{
          fontFamily: "var(--font-body)",
          fontSize: 9.5,
          letterSpacing: 1.6,
          textTransform: "uppercase",
          color: jackpot ? accent : BRASS,
          fontWeight: 600,
        }}
      >
        {kindLabel}
      </div>
      <div style={{ fontSize: 11.5, lineHeight: 1.45, color: "rgba(245,242,232,0.72)" }}>
        {meld.summary}
      </div>
    </div>
  );
}
