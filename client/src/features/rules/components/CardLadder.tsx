import type { CardRow } from "@/features/rules/content/types";
import { useRules } from "@/features/rules/RulesContext";
import { cn } from "@/shared/lib/utils";

type SuitKey = "S" | "H" | "D" | "C";
type LadderTone = "trump" | "plain";

const SUIT_GLYPH: Record<SuitKey, string> = { S: "♠", H: "♥", D: "♦", C: "♣" };
const TEXT_PRESENTATION_SELECTOR = "︎";
const SUIT_INK: Record<SuitKey, string> = {
  S: "text-ink",
  C: "text-ink",
  H: "text-danger",
  D: "text-danger",
};

const TONE: Record<
  LadderTone,
  { ink: string; edge: string; fill: string; tint: string; softTint: string }
> = {
  trump: {
    ink: "text-accent",
    edge: "border-accent/45",
    fill: "bg-accent/45",
    tint: "bg-accent/10",
    softTint: "bg-accent/5",
  },
  plain: {
    ink: "text-brass-deep",
    edge: "border-brass",
    fill: "bg-brass",
    tint: "bg-brass-soft",
    softTint: "bg-brass/10",
  },
};

const COLUMNS = "grid grid-cols-[34px_1fr_56px_64px] items-center px-4.5";

function PowerBar({ strength, tone }: { strength: number; tone: LadderTone }) {
  const filled = 9 - strength;
  return (
    <span className="inline-flex gap-0.5">
      {Array.from({ length: 8 }).map((_, i) => (
        <span
          key={i}
          className={cn("h-3 w-1 rounded-xs", i < filled ? TONE[tone].fill : "bg-surface-3")}
        />
      ))}
    </span>
  );
}

type Props = {
  title: string;
  eyebrow: string;
  tone: LadderTone;
  rows: CardRow[];
  suit: SuitKey;
  testId?: string;
};

/** Card-value table for one suit category (trump or off-trump), with an eight-cell strength bar per card. */
export function CardLadder({ title, eyebrow, tone, rows, suit, testId }: Props) {
  const { ui } = useRules();
  const t = TONE[tone];
  return (
    <div
      data-testid={testId}
      className="bg-surface border-border min-w-0 flex-1 overflow-hidden rounded-lg border"
    >
      <header className="bg-surface-2 border-border flex items-center gap-2.5 border-b px-4.5 pt-3.5 pb-3">
        <span
          className={cn(
            "inline-flex size-6 items-center justify-center rounded-sm border text-[13px] leading-none",
            t.tint,
            t.edge,
            SUIT_INK[suit],
          )}
        >
          {SUIT_GLYPH[suit]}
          {TEXT_PRESENTATION_SELECTOR}
        </span>
        <div className="flex flex-col">
          <span className={cn("font-mono text-2xs font-bold tracking-caps uppercase", t.ink)}>
            {eyebrow}
          </span>
          <span className="text-ink mt-px text-base font-semibold">{title}</span>
        </div>
      </header>

      <div className="py-1.5">
        <div
          className={cn(
            COLUMNS,
            "text-ink-mute pt-2 pb-1.5 font-mono text-2xs font-semibold tracking-caps uppercase",
          )}
        >
          <span>#</span>
          <span>{ui.colCard}</span>
          <span className="text-right">{ui.colPoints}</span>
          <span className="text-right">{ui.colPower}</span>
        </div>
        {rows.map((r) => {
          const top = r.strength === 1;
          return (
            <div
              key={r.rank}
              className={cn(COLUMNS, "border-border border-t py-2", top && t.softTint)}
            >
              <span
                className={cn("font-mono text-xs font-semibold", top ? t.ink : "text-ink-mute")}
              >
                {String(r.strength).padStart(2, "0")}
              </span>
              <span className="flex min-w-0 items-center gap-2.5">
                <span
                  className={cn(
                    "bg-surface-2 inline-flex h-8 min-w-7 items-center justify-center rounded-sm border px-1.5 text-sm font-bold tracking-tight",
                    top ? cn(t.edge, t.ink) : "border-border text-ink",
                  )}
                >
                  {r.rank}
                </span>
                <span className={cn("text-ink text-sm", top ? "font-semibold" : "font-medium")}>
                  {r.name}
                </span>
                {r.note && (
                  <span
                    className={cn("hidden text-xs italic sm:inline", top ? t.ink : "text-ink-mute")}
                  >
                    · {r.note}
                  </span>
                )}
              </span>
              <span
                className={cn(
                  "text-right font-mono text-sm tabular-nums",
                  r.pts > 0 ? "text-ink font-semibold" : "text-ink-mute font-medium",
                )}
              >
                {r.pts}
              </span>
              <span className="inline-flex items-center justify-end">
                <PowerBar strength={r.strength} tone={tone} />
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
