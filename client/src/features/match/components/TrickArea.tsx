import type { TrickCard } from "@/shared/types/matchTypes";

import { CARD_SIZES } from "../lib/cardFace";
import { compassOffset, SLOT_POSITIONS } from "../lib/trickLayout";
import { PlayingCard } from "./PlayingCard";

const EMPTY_TRICK: TrickCard[] = [];
const EMPTY_SET: ReadonlySet<string> = new Set();

export interface PendingResolvedSnapshot {
  trick: TrickCard[];
  winnerSeat: number;
}

interface TrickAreaProps {
  trick: TrickCard[] | null;
  winnerSeat: number | null;
  myPlayerSeat: number;
  pendingResolvedTrick?: PendingResolvedSnapshot | null;
  suppressedCardIds?: ReadonlySet<string>;
  compact?: boolean;
}

export const TRICK_SLOT_W = CARD_SIZES.md.width;
export const TRICK_SLOT_H = CARD_SIZES.md.height;

const PLACEHOLDER_W = TRICK_SLOT_W;
const PLACEHOLDER_H = TRICK_SLOT_H;

const TRICK_AREA_W = 280;
const TRICK_AREA_H = 240;

/**
 * Static painter for the cards on the table, drawing the resolved-trick snapshot and the live trick together.
 * Cards in `suppressedCardIds` are skipped because the `CardFlight` overlay is animating them.
 */
export function TrickArea({
  trick: rawTrick,
  winnerSeat,
  myPlayerSeat,
  pendingResolvedTrick = null,
  suppressedCardIds = EMPTY_SET,
  compact = false,
}: TrickAreaProps) {
  const phW = PLACEHOLDER_W;
  const phH = PLACEHOLDER_H;
  const slotScale = compact ? 0.78 : 1;

  const liveTrick = rawTrick ?? EMPTY_TRICK;
  const snapshotTrick = pendingResolvedTrick !== null ? pendingResolvedTrick.trick : null;
  const effectiveWinnerSeat =
    pendingResolvedTrick !== null ? pendingResolvedTrick.winnerSeat : winnerSeat;
  const showWinnerGlow = pendingResolvedTrick !== null;

  const winnerCompass =
    effectiveWinnerSeat !== null ? compassOffset(effectiveWinnerSeat, myPlayerSeat) : null;

  const snapshotIds = new Set((snapshotTrick ?? []).map((tc) => `${tc.card.rank}${tc.card.suit}`));
  const displayEntries = [
    ...(snapshotTrick ?? []).map((tc) => ({ tc, fromSnapshot: true })),
    ...liveTrick
      .filter((tc) => !snapshotIds.has(`${tc.card.rank}${tc.card.suit}`))
      .map((tc) => ({ tc, fromSnapshot: false })),
  ];

  const renderableEntries = displayEntries.filter(({ tc }) => {
    const cardId = `${tc.card.rank}${tc.card.suit}`;
    return !suppressedCardIds.has(cardId);
  });

  const playedByCompass = new Set(
    renderableEntries.map(({ tc }) => compassOffset(tc.playerSeat, myPlayerSeat)),
  );

  return (
    <div
      className="relative pointer-events-none"
      style={{ width: TRICK_AREA_W, height: TRICK_AREA_H }}
      data-testid="trick-area"
    >
      {([0, 1, 2, 3] as const).map((compass) => {
        const slot = SLOT_POSITIONS[compass];
        const occupied = playedByCompass.has(compass);
        return (
          <div
            key={`placeholder-${compass}`}
            className="absolute"
            style={{
              left: "50%",
              top: "50%",
              width: phW,
              height: phH,
              transform: `translate(calc(-50% + ${slot.offsetX * slotScale}px), calc(-50% + ${slot.offsetY * slotScale}px)) rotate(${slot.rotation}deg)`,
              borderRadius: CARD_SIZES.md.radius,
              border: occupied ? "1.5px solid transparent" : "1.5px dashed rgba(255,255,255,0.14)",
              background: occupied ? "transparent" : "rgba(255,255,255,0.02)",
              visibility: "visible",
              zIndex: 0,
            }}
            data-testid={`trick-slot-${compass}`}
            aria-hidden="true"
          />
        );
      })}

      {renderableEntries.map(({ tc, fromSnapshot }) => {
        const compass = compassOffset(tc.playerSeat, myPlayerSeat);
        const slot = SLOT_POSITIONS[compass];
        const isWinner = showWinnerGlow && fromSnapshot && compass === winnerCompass;

        return (
          <div
            key={`${tc.card.rank}${tc.card.suit}`}
            className={`absolute ${isWinner ? "shadow-[0_0_20px_var(--color-accent)]" : ""}`}
            data-testid={`trick-slot-card-${compass}${fromSnapshot ? "-resolved" : ""}`}
            style={{
              left: "50%",
              top: "50%",
              transform: `translate(calc(-50% + ${slot.offsetX * slotScale}px), calc(-50% + ${slot.offsetY * slotScale}px)) rotate(${slot.rotation}deg)`,
              zIndex: fromSnapshot ? 1 : 2,
            }}
          >
            <PlayingCard card={tc.card} state="default" size="md" withTransition={false} />
          </div>
        );
      })}
    </div>
  );
}
