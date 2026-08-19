// Rendering-time merge of the viewer's own revealed face-down cards.
//
// Under the Croatian deal a seat physically holds eight cards while only six sit
// in the authoritative snapshot: the other two are dealt face-down and reach
// their owner alone, on `event:face_down_revealed`, because `match_state` is
// serialized once and the identical bytes go to all four seats. The store holds
// that per-seat payload in its own slice; this function folds it into the hand
// for DISPLAY only. The server stays the single authority on what a hand
// actually contains.

import { isCardId, splitCardId } from "@/shared/lib/cardId";
import type { Card } from "@/shared/types/matchTypes";

/**
 * Appends the viewer's own revealed face-down cards to their rendered hand.
 *
 * Returns the original array (same reference) when there is nothing to merge —
 * the Bitola path, and every phase after bidding resolves — so renders in the
 * common case are unaffected.
 *
 * Guards, each load-bearing:
 * - `revealedSeat !== myPlayerSeat` merges nothing. A payload naming another
 *   seat is a server bug, and this is the last line that stops it reaching the
 *   viewer's hand.
 * - Ids already present are skipped, so a duplicated or replayed event (the
 *   reconnect path re-sends it) cannot double a card.
 * - Malformed ids are skipped rather than sliced into a wrong card.
 */
export function mergeRevealedFaceDownCards(
  hand: Card[],
  myPlayerSeat: number | null,
  revealedSeat: number | null,
  revealedCardIds: readonly string[] | undefined,
): Card[] {
  if (
    myPlayerSeat === null ||
    revealedSeat === null ||
    revealedSeat !== myPlayerSeat ||
    revealedCardIds === undefined ||
    revealedCardIds.length === 0
  ) {
    return hand;
  }
  const present = new Set(hand.map((c) => `${c.rank}${c.suit}`));
  const extra: Card[] = [];
  for (const id of revealedCardIds) {
    if (!isCardId(id) || present.has(id)) continue;
    present.add(id);
    extra.push(splitCardId(id));
  }
  return extra.length === 0 ? hand : [...hand, ...extra];
}
