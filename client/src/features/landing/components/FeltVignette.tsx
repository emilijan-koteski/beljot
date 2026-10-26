import { type CSSProperties, useLayoutEffect, useRef, useState } from "react";

import { CardFan } from "@/features/landing/components/CardFan";
import { PlayingCard, Suit } from "@/features/landing/components/PlayingCard";

type FeltLabels = { trump: string; opponent: string; partner: string };

type FeltVignetteProps = {
  width?: number;
  height?: number;
  labels: FeltLabels;
  className?: string;
  style?: CSSProperties;
};

type SeatProps = {
  name: string;
  sub: string;
  team: "a" | "b";
  pos: CSSProperties;
  flip?: boolean;
};

function Seat({ name, sub, team, pos, flip = false }: SeatProps) {
  return (
    <div
      style={{
        position: "absolute",
        display: "flex",
        flexDirection: flip ? "row-reverse" : "row",
        alignItems: "center",
        gap: 8,
        ...pos,
      }}
    >
      <div
        className="font-display"
        style={{
          width: 34,
          height: 34,
          borderRadius: "50%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          fontWeight: 700,
          fontSize: 14,
          color: "var(--brass-ink)",
          background: team === "a" ? "var(--team-a-fill)" : "var(--team-b-fill)",
          border: `2px solid ${team === "a" ? "var(--team-a)" : "var(--team-b)"}`,
        }}
      >
        {name[0]}
      </div>
      <div style={{ lineHeight: 1.25, textAlign: flip ? "right" : "left" }}>
        <div className="text-ink" style={{ fontSize: 12.5, fontWeight: 600 }}>
          {name}
        </div>
        <div className="text-ink-mute font-mono" style={{ fontSize: 10.5 }}>
          {sub}
        </div>
      </div>
    </div>
  );
}

// Static felt-table illustration authored at a fixed design size.
// The stage scales to the board's rendered width so it shrinks to fit narrow columns.
export function FeltVignette({
  width = 560,
  height = 360,
  labels,
  className,
  style,
}: FeltVignetteProps) {
  const boardRef = useRef<HTMLDivElement>(null);
  const [scale, setScale] = useState(1);

  useLayoutEffect(() => {
    const board = boardRef.current;
    if (!board) return;
    const measure = () => {
      const w = board.clientWidth;
      if (w > 0) setScale(w / width);
    };
    measure();
    if (typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver(measure);
    ro.observe(board);
    return () => ro.disconnect();
  }, [width]);

  return (
    <div
      ref={boardRef}
      className={`felt-surface ${className ?? ""}`}
      style={{
        position: "relative",
        width: "100%",
        maxWidth: width,
        aspectRatio: `${width} / ${height}`,
        borderRadius: 20,
        overflow: "hidden",
        background:
          "radial-gradient(ellipse 70% 60% at 50% 42%, var(--felt-scene-top) 0%, var(--surface) 60%, var(--bg) 100%)",
        border: "3px solid var(--wood-rim)",
        boxShadow: "inset 0 0 60px rgba(0,0,0,0.55), 0 30px 70px -30px rgba(0,0,0,0.6)",
        ...style,
      }}
    >
      <div
        style={{
          position: "absolute",
          inset: 14,
          borderRadius: 12,
          border: "1px solid var(--border)",
        }}
      />

      <div
        style={{
          position: "absolute",
          top: 0,
          left: 0,
          width,
          height,
          transformOrigin: "top left",
          transform: `scale(${scale})`,
        }}
      >
        <div style={{ position: "absolute", left: "50%", top: "46%", width: 0, height: 0 }}>
          <div style={{ position: "absolute", transform: "translate(-50%,-95%) rotate(-8deg)" }}>
            <PlayingCard rank="Q" suit="hearts" w={62} />
          </div>
          <div style={{ position: "absolute", transform: "translate(-95%,-50%) rotate(-4deg)" }}>
            <PlayingCard rank="10" suit="spades" w={62} />
          </div>
          <div style={{ position: "absolute", transform: "translate(-5%,-50%) rotate(6deg)" }}>
            <PlayingCard rank="K" suit="diamonds" w={62} />
          </div>
          <div style={{ position: "absolute", transform: "translate(-50%,-5%) rotate(3deg)" }}>
            <PlayingCard rank="J" suit="clubs" w={62} />
          </div>
        </div>

        <Seat
          name="Kiro"
          sub={`Lvl 32 · ${labels.opponent}`}
          team="b"
          pos={{ left: 22, top: "40%" }}
        />
        <Seat
          name="Irena"
          sub={`Lvl 28 · ${labels.opponent}`}
          team="b"
          pos={{ right: 22, top: "40%" }}
          flip
        />
        <Seat
          name="Emilijan"
          sub={`Lvl 35 · ${labels.partner}`}
          team="a"
          pos={{ left: "50%", top: 16, transform: "translateX(-50%)" }}
        />

        <div
          style={{ position: "absolute", left: "50%", bottom: -6, transform: "translateX(-50%)" }}
        >
          <CardFan
            w={52}
            overlap={0.4}
            arc={6}
            lift={8}
            cards={[
              { rank: "A", suit: "hearts" },
              { rank: "K", suit: "hearts" },
              { rank: "Q", suit: "spades" },
              { rank: "J", suit: "diamonds" },
              { rank: "9", suit: "clubs" },
              { rank: "A", suit: "clubs" },
            ]}
          />
        </div>

        <div
          style={{
            position: "absolute",
            right: 18,
            top: 16,
            display: "flex",
            alignItems: "center",
            gap: 7,
            background: "rgba(10,24,18,0.7)",
            border: "1px solid var(--brass)",
            padding: "6px 11px",
            borderRadius: 10,
          }}
        >
          <span
            className="text-brass font-mono"
            style={{ fontSize: 9, letterSpacing: 1.5, textTransform: "uppercase" }}
          >
            {labels.trump}
          </span>
          <Suit name="hearts" size={15} />
        </div>
      </div>
    </div>
  );
}
