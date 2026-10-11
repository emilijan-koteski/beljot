import type { CSSProperties } from "react";

import { type CardSpec, PlayingCard } from "@/features/landing/components/PlayingCard";

type CardFanProps = {
  cards: CardSpec[];
  w?: number;
  overlap?: number;
  arc?: number;
  lift?: number;
  className?: string;
  style?: CSSProperties;
};

// Overlapping spread of cards; `overlap` pulls cards together, `arc` tilts them, `lift` raises the middle.
export function CardFan({
  cards,
  w = 96,
  overlap = 0.46,
  arc = 7,
  lift = 10,
  className,
  style,
}: CardFanProps) {
  const n = cards.length;
  return (
    <div className={className} style={{ display: "flex", alignItems: "flex-end", ...style }}>
      {cards.map((c, i) => {
        const t = i - (n - 1) / 2;
        return (
          <div key={i} style={{ marginLeft: i === 0 ? 0 : -(w * overlap) }}>
            <PlayingCard
              {...c}
              w={w}
              tilt={t * arc}
              raise={Math.round((1 - Math.abs(t) / n) * lift)}
              z={i + 1}
            />
          </div>
        );
      })}
    </div>
  );
}
