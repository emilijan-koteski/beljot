import { useEffect, useState } from "react";

import type { Card, Rank, Suit } from "@/shared/types/matchTypes";

import type { CardState } from "./PlayingCard";
import { PlayingCard } from "./PlayingCard";

interface HandCardsProps {
  hand: Card[];
  isMyTurn: boolean;
  playableCardIds: string[];
  onPlayCard: (cardId: string) => void;
  /**
   * Phone layout: the fan overlaps more so all 8 cards fit the viewport width
   * (rather than running off the edges). Card size is unchanged — only the
   * lateral spread compresses to fit.
   */
  compact?: boolean;
  /**
   * Card id (e.g. `"KS"`) currently being thrown into the trick. Hides the
   * matching card via `visibility: hidden` so the `CardFlight` overlay is
   * the only painter of the in-flight card. The card stays laid out (so the
   * rest of the fan doesn't reflow mid-flight) and the source rect we
   * measured at click time stays valid.
   */
  flyingId?: string | null;
}

// Display sizing matches the `lg` PlayingCard variant — wider than the previous
// medium fan so the table-edge presentation reads at glance.
const CARD_WIDTH = 88; // matches SIZE_DIMENSIONS.lg.width in PlayingCard
const CARD_HEIGHT = 128;

// Maximum lateral spread between adjacent cards before they start to compress.
const MAX_SPREAD_PX = 54;
// At a normal 8-card hand this lands at ~52 px; for fewer cards we widen so
// the fan still feels balanced.
const SPREAD_BUDGET_PX = 480;

// Fan rotation: the leftmost card tilts slightly counter-clockwise, the
// rightmost slightly clockwise, mid-cards stay almost vertical.
const PER_OFFSET_DEG = 2.2;

// Vertical sag per offset — pure arc visual without an SVG path.
const PER_OFFSET_DROP = 3;

// Alternating-color display order so neighboring suits are visually distinct.
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

/**
 * Bottom-of-table fan of the local player's hand.
 *
 * Layout: cards are absolutely positioned around a horizontal centerline. Each
 * card pivots from its bottom-center via `transform-origin: 50% 120%` so the
 * rotation reads as a fan rather than a tilt. Outer cards drop a few pixels so
 * the top edge traces a gentle arc.
 *
 * State channel:
 *  • `playable`   — picked up (lifted) with a lime halo (handled inside the
 *                   `PlayingCard`).
 *  • `unplayable` — stays at full opacity per the design's "visible but not
 *                   playable" rule, sits a few pixels lower.
 *  • `default`    — when it's not my turn, every card renders in default state
 *                   so the legality hint never bleeds across turns.
 */
export function HandCards({
  hand,
  isMyTurn,
  playableCardIds,
  onPlayCard,
  flyingId = null,
  compact = false,
}: HandCardsProps) {
  // Track viewport width so the phone fan can compress to whatever's available
  // (and re-fit on rotation). Desktop ignores this — spread stays budget-based.
  const [viewportWidth, setViewportWidth] = useState<number>(() =>
    typeof window === "undefined" ? 1280 : window.innerWidth,
  );
  useEffect(() => {
    const onResize = () => setViewportWidth(window.innerWidth);
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

  if (hand.length === 0) {
    return <div className="relative h-36 w-px" data-testid="hand-cards" />;
  }

  const sortedHand = sortHand(hand);
  const total = sortedHand.length;
  const budgetSpread = Math.min(MAX_SPREAD_PX, total > 1 ? SPREAD_BUDGET_PX / total : 0);
  // On phones, cap the spread so the whole fan fits within the viewport; cards
  // keep their full `lg` size and just overlap more. The margin also absorbs the
  // ~17 px the outermost (rotated) cards' corners swing past their nominal box.
  const COMPACT_SIDE_MARGIN = 28;
  const fitSpread =
    total > 1 ? (viewportWidth - 2 * COMPACT_SIDE_MARGIN - CARD_WIDTH) / (total - 1) : budgetSpread;
  const spread =
    compact && total > 1 ? Math.max(8, Math.min(budgetSpread, fitSpread)) : budgetSpread;
  // Container width needs to fit the outermost card's centre offset + half a
  // card width on either side. Add safety for the lift transform halo.
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
              // While flying, hide the hand card's pixels — the CardFlight
              // overlay paints the moving card in screen-fixed coordinates.
              // Visibility (rather than display:none) preserves the layout so
              // adjacent cards don't reflow during the flight, which would
              // otherwise mis-position the source rect we measured.
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
    </div>
  );
}
