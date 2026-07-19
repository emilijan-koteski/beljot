import type { CardRow } from "@/features/rules/content/types";
import { useRules } from "@/features/rules/RulesContext";

// Accent palette for one ladder (trump = felt-green, off-trump = brass).
export type LadderAccent = {
  eyebrow: string;
  ink: string;
  edge: string;
  tint: string;
  softTint: string;
};

type SuitKey = "S" | "H" | "D" | "C";

const SUIT_GLYPH: Record<SuitKey, string> = { S: "♠", H: "♥", D: "♦", C: "♣" };
const SUIT_COLOR: Record<SuitKey, string> = {
  S: "var(--ink)",
  C: "var(--ink)",
  H: "var(--danger)",
  D: "var(--danger)",
};

function SuitGlyph({ suit, size = 13 }: { suit: SuitKey; size?: number }) {
  return (
    <span style={{ color: SUIT_COLOR[suit], fontSize: size, lineHeight: 1 }}>
      {/* U+FE0E forces text (not emoji) rendering so the glyph stays small. */}
      {SUIT_GLYPH[suit]}
      {"︎"}
    </span>
  );
}

// Mini strength bar — 8 cells, fill (9 - strength) so strongest = full.
function PowerBar({ strength, accent }: { strength: number; accent: LadderAccent }) {
  const filled = 9 - strength;
  return (
    <span style={{ display: "inline-flex", gap: 2 }}>
      {Array.from({ length: 8 }).map((_, i) => (
        <span
          key={i}
          style={{
            width: 4,
            height: 12,
            borderRadius: 1,
            background: i < filled ? accent.edge : "var(--surface-3)",
          }}
        />
      ))}
    </span>
  );
}

type Props = {
  title: string;
  accent: LadderAccent;
  rows: CardRow[];
  suit: SuitKey;
  testId?: string;
};

/** Card-value table for one suit category (trump or off-trump). */
export function CardLadder({ title, accent, rows, suit, testId }: Props) {
  const { ui } = useRules();
  return (
    <div
      data-testid={testId}
      style={{
        flex: 1,
        minWidth: 0,
        background: "var(--surface)",
        border: "1px solid var(--border)",
        borderRadius: "var(--radius-lg)",
        overflow: "hidden",
      }}
    >
      <header
        style={{
          padding: "14px 18px 12px",
          borderBottom: "1px solid var(--border)",
          background: "var(--surface-2)",
          display: "flex",
          alignItems: "center",
          gap: 10,
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
            background: accent.tint,
            color: accent.ink,
            border: `1px solid ${accent.edge}`,
          }}
        >
          <SuitGlyph suit={suit} />
        </span>
        <div style={{ display: "flex", flexDirection: "column" }}>
          <span
            className="font-mono"
            style={{
              fontSize: 10.5,
              letterSpacing: 2.2,
              textTransform: "uppercase",
              color: accent.ink,
              fontWeight: 700,
            }}
          >
            {accent.eyebrow}
          </span>
          <span
            className="font-display"
            style={{
              fontSize: 15,
              fontWeight: 600,
              color: "var(--ink)",
              letterSpacing: -0.1,
              marginTop: 1,
            }}
          >
            {title}
          </span>
        </div>
      </header>

      <div style={{ padding: "6px 0" }}>
        <div
          className="font-mono"
          style={{
            display: "grid",
            gridTemplateColumns: "34px 1fr 56px 64px",
            padding: "8px 18px 6px",
            fontSize: 10,
            letterSpacing: 1.8,
            textTransform: "uppercase",
            color: "var(--ink-mute)",
            fontWeight: 600,
          }}
        >
          <span>#</span>
          <span>{ui.colCard}</span>
          <span style={{ textAlign: "right" }}>{ui.colPoints}</span>
          <span style={{ textAlign: "right" }}>{ui.colPower}</span>
        </div>
        {rows.map((r) => {
          const top = r.strength === 1;
          return (
            <div
              key={r.rank}
              style={{
                display: "grid",
                gridTemplateColumns: "34px 1fr 56px 64px",
                alignItems: "center",
                padding: "8px 18px",
                borderTop: "1px solid var(--border)",
                background: top ? accent.softTint : "transparent",
              }}
            >
              <span
                className="font-mono"
                style={{
                  fontSize: 11.5,
                  color: top ? accent.ink : "var(--ink-mute)",
                  fontWeight: 600,
                }}
              >
                {String(r.strength).padStart(2, "0")}
              </span>
              <span style={{ display: "flex", alignItems: "center", gap: 10 }}>
                <span
                  className="font-display"
                  style={{
                    display: "inline-flex",
                    alignItems: "center",
                    justifyContent: "center",
                    minWidth: 28,
                    height: 32,
                    padding: "0 6px",
                    borderRadius: 6,
                    background: "var(--surface-2)",
                    border: `1px solid ${top ? accent.edge : "var(--border)"}`,
                    fontWeight: 700,
                    fontSize: 13.5,
                    color: top ? accent.ink : "var(--ink)",
                    letterSpacing: -0.2,
                  }}
                >
                  {r.rank}
                </span>
                <span style={{ fontSize: 13.5, color: "var(--ink)", fontWeight: top ? 600 : 500 }}>
                  {r.name}
                </span>
                {r.note && (
                  <span
                    // The strongest/weakest annotations wrap awkwardly beside
                    // the card name on phones — hidden below sm (the tinted
                    // top row and the rank order carry the meaning there).
                    className="hidden sm:inline"
                    style={{
                      fontSize: 11.5,
                      color: top ? accent.ink : "var(--ink-mute)",
                      fontStyle: "italic",
                    }}
                  >
                    · {r.note}
                  </span>
                )}
              </span>
              <span
                className="font-mono tabular-nums"
                style={{
                  textAlign: "right",
                  fontSize: 13,
                  color: r.pts > 0 ? "var(--ink)" : "var(--ink-off)",
                  fontWeight: r.pts > 0 ? 600 : 500,
                }}
              >
                {r.pts}
              </span>
              <span
                style={{
                  textAlign: "right",
                  display: "inline-flex",
                  justifyContent: "flex-end",
                  alignItems: "center",
                }}
              >
                <PowerBar strength={r.strength} accent={accent} />
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
