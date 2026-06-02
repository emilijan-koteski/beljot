import type { TrickCard } from "@/shared/types/matchTypes";

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
  /**
   * Snapshot of the just-resolved trick captured by the WS dispatcher (see
   * `matchStore.pendingResolvedTrick`). When non-null, this owns the trick
   * display until cleared — `currentTrick` may already be `[]` (server
   * cleared it on resolve), and without the snapshot the four cards would
   * vanish before the collect animation could run.
   */
  pendingResolvedTrick?: PendingResolvedSnapshot | null;
  /**
   * Set of `${rank}${suit}` ids whose cards are currently being painted by
   * the `CardFlight` overlay. TrickArea filters these out of the slot
   * rendering so the overlay's flying card and the static slot card never
   * double-paint. The overlay's last frame and TrickArea's static slot card
   * land at the same viewport position, so removing the id from this set
   * (when the flight completes) is a pixel-equivalent handoff.
   */
  suppressedCardIds?: ReadonlySet<string>;
  /** Phone layout: smaller placeholders pulled into a tighter diamond. */
  compact?: boolean;
}

/** PlayingCard `md` dimensions — used for both placeholders and slot cards.
 *  Exported so the CardFlight wiring in `MatchPage` can compute the slot's
 *  destination rect without measuring the slot DOM. */
export const TRICK_SLOT_W = 72;
export const TRICK_SLOT_H = 104;

const PLACEHOLDER_W = TRICK_SLOT_W;
const PLACEHOLDER_H = TRICK_SLOT_H;

const TRICK_AREA_W = 280;
const TRICK_AREA_H = 240;

/**
 * Trick area — renders the 1–4 cards currently on the table at their compass
 * positions. The component is presentation-only: all motion (cards flying in
 * from the player who threw them, cards collecting toward the winner) lives
 * in the `CardFlight` overlay. TrickArea is the static painter that shows the
 * "settled" state of each slot.
 *
 * Rendering source priority:
 * 1. `pendingResolvedTrick` (when set) — the captured snapshot of the
 *    just-resolved 4-card trick. Owns the display through the resolve-glow
 *    phase and while the collect flights are in transit.
 * 2. `trick` (the live `currentTrick` from the server) — used during normal
 *    play, between resolves.
 *
 * Suppression: any cardId present in `suppressedCardIds` is removed from
 * rendering — that card is currently being animated by `CardFlight` and the
 * slot must stay empty (placeholder visible) so the overlay doesn't double-
 * paint with this static card.
 */
export function TrickArea({
  trick: rawTrick,
  winnerSeat,
  myPlayerSeat,
  pendingResolvedTrick = null,
  suppressedCardIds = EMPTY_SET,
  compact = false,
}: TrickAreaProps) {
  // Phone: keep the placeholders (and thrown cards) at full card size but pull
  // the diamond in a little so it fits the narrower felt. Placeholders and cards
  // share the same size + scaled offsets so a thrown card lands exactly on its
  // slot.
  const phW = PLACEHOLDER_W;
  const phH = PLACEHOLDER_H;
  const slotScale = compact ? 0.78 : 1;

  const liveTrick = rawTrick ?? EMPTY_TRICK;
  // Resolve which trick the slots paint from. When the snapshot is set the
  // server may already have cleared `currentTrick` — using the snapshot
  // keeps the four cards on screen for the resolve-glow + collect-flight
  // window.
  const displayTrick = pendingResolvedTrick !== null ? pendingResolvedTrick.trick : liveTrick;
  const effectiveWinnerSeat =
    pendingResolvedTrick !== null ? pendingResolvedTrick.winnerSeat : winnerSeat;
  const showWinnerGlow = pendingResolvedTrick !== null;

  const winnerCompass =
    effectiveWinnerSeat !== null ? compassOffset(effectiveWinnerSeat, myPlayerSeat) : null;

  const renderableTrick = displayTrick.filter((tc) => {
    const cardId = `${tc.card.rank}${tc.card.suit}`;
    return !suppressedCardIds.has(cardId);
  });

  // Build from the post-suppression set so a compass whose card is mid-flight
  // still shows the dashed placeholder — otherwise the slot reads as a black
  // hole during the flight (no card, no border).
  const playedByCompass = new Set(
    renderableTrick.map((tc) => compassOffset(tc.playerSeat, myPlayerSeat)),
  );

  return (
    <div
      className="relative pointer-events-none"
      style={{ width: TRICK_AREA_W, height: TRICK_AREA_H }}
      data-testid="trick-area"
    >
      {/* Slot anchors at every compass. These are always rendered (visibility
          flips to hidden when a real card occupies the slot) so the CardFlight
          overlay can always measure the slot's viewport rect via
          `getBoundingClientRect()` — without this, a slot with a settled card
          would have no `data-testid="trick-slot-{compass}"` element to
          measure against during a take/collect flight. */}
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
              borderRadius: 6,
              border: occupied ? "1.5px solid transparent" : "1.5px dashed rgba(255,255,255,0.14)",
              background: occupied ? "transparent" : "rgba(255,255,255,0.02)",
              // Keep the anchor measurable even when a card sits over it.
              // Pointer events stay off (the parent already disables them).
              visibility: "visible",
              zIndex: 0,
            }}
            data-testid={`trick-slot-${compass}`}
            aria-hidden="true"
          />
        );
      })}

      {renderableTrick.map((tc) => {
        const compass = compassOffset(tc.playerSeat, myPlayerSeat);
        const slot = SLOT_POSITIONS[compass];
        const isWinner = showWinnerGlow && compass === winnerCompass;

        return (
          <div
            key={`${tc.card.rank}${tc.card.suit}`}
            className={`absolute ${isWinner ? "shadow-[0_0_20px_var(--color-accent)]" : ""}`}
            data-testid={`trick-slot-card-${compass}`}
            style={{
              left: "50%",
              top: "50%",
              transform: `translate(calc(-50% + ${slot.offsetX * slotScale}px), calc(-50% + ${slot.offsetY * slotScale}px)) rotate(${slot.rotation}deg)`,
            }}
          >
            <PlayingCard card={tc.card} state="default" size="md" withTransition={false} />
          </div>
        );
      })}
    </div>
  );
}
