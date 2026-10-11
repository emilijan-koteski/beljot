import { useEffect, useState } from "react";

import type { Card, Rank, Suit } from "@/shared/types/matchTypes";

import { CARD_SIZES } from "../lib/cardFace";
import type { CardState } from "./PlayingCard";
import { PlayingCard } from "./PlayingCard";

interface HandCardsProps {
  hand: Card[];
  isMyTurn: boolean;
  playableCardIds: string[];
  onPlayCard: (cardId: string) => void;
  compact?: boolean;
  faceDownCount?: number;
  flyingId?: string | null;
}

const { width: CARD_WIDTH, height: CARD_HEIGHT } = CARD_SIZES.lg;

const MAX_SPREAD_PX = 54;
const SPREAD_BUDGET_PX = 480;

const PER_OFFSET_DEG = 2.2;

const PER_OFFSET_DROP = 3;

const SUIT_ORDER: Record<Suit, number> = { S: 0, H: 1, C: 2, D: 3 };
const RANK_ORDER: Record<Rank, number> = { "7": 0, "8": 1, "9": 2, T: 3, J: 4, Q: 5, K: 6, A: 7 };

function cardId(card: Card): string {
  return `${card.rank}${card.suit}`;
}

function sortHand(hand: Card[]): Card[] {
  return [...hand].sort((a, b) => {
    const suitDiff = SUIT_ORDER[a.suit] - SUIT_ORDER[b.suit];
    return suitDiff !== 0 ? suitDiff : RANK_ORDER[a.rank] - RANK_ORDER[b.rank];
  });
}

/** Bottom-of-table fan of the local player's hand, with lifted playable cards and a gentle arc. */
export function HandCards({
  hand,
  isMyTurn,
  playableCardIds,
  onPlayCard,
  flyingId = null,
  compact = false,
  faceDownCount = 0,
}: HandCardsProps) {
  const [viewportWidth, setViewportWidth] = useState<number>(() =>
    typeof window === "undefined" ? 1280 : window.innerWidth,
  );
  useEffect(() => {
    const onResize = () => setViewportWidth(window.innerWidth);
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

  if (hand.length === 0 && faceDownCount === 0) {
    return (
      <div
        className="relative w-px"
        style={{ height: CARD_HEIGHT + 32 }}
        data-testid="hand-cards"
      />
    );
  }

  const sortedHand = sortHand(hand);
  const backs = Math.max(0, faceDownCount);
  const total = sortedHand.length + backs;
  const budgetSpread = Math.min(MAX_SPREAD_PX, total > 1 ? SPREAD_BUDGET_PX / total : 0);
  const COMPACT_SIDE_MARGIN = 28;
  const fitSpread =
    total > 1 ? (viewportWidth - 2 * COMPACT_SIDE_MARGIN - CARD_WIDTH) / (total - 1) : budgetSpread;
  const spread =
    compact && total > 1 ? Math.max(8, Math.min(budgetSpread, fitSpread)) : budgetSpread;
  const containerWidth = spread * Math.max(0, total - 1) + CARD_WIDTH + (compact ? 8 : 24);

  return (
    <div
      className="relative flex items-end justify-center"
      style={{ width: containerWidth, height: CARD_HEIGHT + 32 }}
      data-testid="hand-cards"
    >
      {sortedHand.map((card, index) => {
        const id = cardId(card);
        const offset = index - (total - 1) / 2;
        const rotateDeg = offset * PER_OFFSET_DEG;
        const dropPx = Math.abs(offset) * PER_OFFSET_DROP;
        const isFlying = flyingId === id;

        let state: CardState = "default";
        if (isMyTurn && !isFlying) {
          state = playableCardIds.includes(id) ? "playable" : "unplayable";
        }

        return (
          <div
            key={id}
            className="absolute"
            data-testid={`hand-card-${id}`}
            style={{
              left: `calc(50% + ${offset * spread}px - ${CARD_WIDTH / 2}px)`,
              bottom: dropPx,
              transform: `rotate(${rotateDeg}deg)`,
              transformOrigin: "50% 120%",
              zIndex: index,
              visibility: isFlying ? "hidden" : "visible",
              pointerEvents: isFlying ? "none" : "auto",
            }}
          >
            <PlayingCard
              card={card}
              state={state}
              size="lg"
              onClick={state === "playable" ? () => onPlayCard(id) : undefined}
            />
          </div>
        );
      })}

      {Array.from({ length: backs }, (_, i) => {
        const index = sortedHand.length + i;
        const offset = index - (total - 1) / 2;
        return (
          <div
            key={`face-down-${i}`}
            className="absolute"
            data-testid="hand-card-face-down"
            style={{
              left: `calc(50% + ${offset * spread}px - ${CARD_WIDTH / 2}px)`,
              bottom: Math.abs(offset) * PER_OFFSET_DROP,
              transform: `rotate(${offset * PER_OFFSET_DEG}deg)`,
              transformOrigin: "50% 120%",
              zIndex: index,
              pointerEvents: "none",
            }}
          >
            <PlayingCard card={null} state="face-down" size="lg" />
          </div>
        );
      })}
    </div>
  );
}
