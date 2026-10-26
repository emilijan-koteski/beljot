import type { CSSProperties } from "react";

export type SuitName = "hearts" | "diamonds" | "spades" | "clubs";

const SUIT: Record<SuitName, { ch: string; red: boolean }> = {
  hearts: { ch: "♥", red: true },
  diamonds: { ch: "♦", red: true },
  spades: { ch: "♠", red: false },
  clubs: { ch: "♣", red: false },
};

const SUIT_FONT = "var(--font-suit)";

type SuitProps = {
  name: SuitName;
  size?: number;
  color?: string;
  className?: string;
};

export function Suit({ name, size = 14, color, className }: SuitProps) {
  const s = SUIT[name];
  return (
    <span
      className={className}
      aria-hidden="true"
      style={{
        fontSize: size,
        lineHeight: 1,
        fontFamily: SUIT_FONT,
        color: color ?? (s.red ? "var(--suit-red)" : "var(--suit-black)"),
      }}
    >
      {s.ch}
    </span>
  );
}

export type CardSpec = { rank: string; suit: SuitName };

type PlayingCardProps = CardSpec & {
  w?: number;
  tilt?: number;
  raise?: number;
  z?: number;
  faceDown?: boolean;
  className?: string;
  style?: CSSProperties;
};

// Marketing recreation of a Beljot playing card whose whole geometry scales off `w`.
// Colours come from scope-stable card/suit tokens, so the face looks the same on parchment or felt.
export function PlayingCard({
  rank = "A",
  suit = "hearts",
  w = 96,
  tilt = 0,
  raise = 0,
  z = 1,
  faceDown = false,
  className,
  style,
}: PlayingCardProps) {
  const h = Math.round(w * 1.4);
  const col = SUIT[suit].red ? "var(--suit-red)" : "var(--suit-black)";

  const base: CSSProperties = {
    position: "relative",
    width: w,
    height: h,
    flexShrink: 0,
    borderRadius: Math.round(w * 0.11),
    transform: `rotate(${tilt}deg) translateY(${-raise}px)`,
    transformOrigin: "bottom center",
    zIndex: z,
    ...style,
  };

  if (faceDown) {
    return (
      <div
        className={className}
        style={{
          ...base,
          background:
            "repeating-linear-gradient(45deg, var(--accent-deep) 0 6px, var(--accent) 6px 12px)",
          border: "2px solid var(--brass)",
          boxShadow: "0 10px 26px -12px rgba(0,0,0,0.55)",
        }}
      >
        <div
          style={{
            position: "absolute",
            inset: 5,
            borderRadius: Math.round(w * 0.08),
            border: "1px solid var(--brass)",
            opacity: 0.5,
          }}
        />
      </div>
    );
  }

  const corner = (rotate: boolean): CSSProperties => ({
    position: "absolute",
    display: "flex",
    flexDirection: "column",
    alignItems: "center",
    lineHeight: 1,
    ...(rotate
      ? { bottom: w * 0.07, right: w * 0.09, transform: "rotate(180deg)" }
      : { top: w * 0.07, left: w * 0.09 }),
  });

  return (
    <div
      className={className}
      style={{
        ...base,
        background: "var(--card-face)",
        border: "1px solid rgba(0,0,0,0.14)",
        boxShadow: "0 12px 30px -14px rgba(0,0,0,0.5), inset 0 1px 0 rgba(255,255,255,0.7)",
      }}
    >
      <span style={corner(false)}>
        <span className="" style={{ fontWeight: 700, fontSize: w * 0.24, color: col }}>
          {rank}
        </span>
        <Suit name={suit} size={w * 0.17} />
      </span>
      <span
        style={{
          position: "absolute",
          inset: 0,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <Suit name={suit} size={w * 0.5} />
      </span>
      <span style={corner(true)}>
        <span className="" style={{ fontWeight: 700, fontSize: w * 0.24, color: col }}>
          {rank}
        </span>
        <Suit name={suit} size={w * 0.17} />
      </span>
    </div>
  );
}
