import type { Card, MatchState, Rank, Suit, TrickCard } from "@/shared/types/matchTypes";

const TRUMP_RANK_ORDER: Record<Rank, number> = {
  J: 7,
  "9": 6,
  A: 5,
  T: 4,
  K: 3,
  Q: 2,
  "8": 1,
  "7": 0,
};

const NON_TRUMP_RANK_ORDER: Record<Rank, number> = {
  A: 7,
  T: 6,
  K: 5,
  Q: 4,
  J: 3,
  "9": 2,
  "8": 1,
  "7": 0,
};

function filterBySuit(hand: Card[], suit: Suit): Card[] {
  return hand.filter((c) => c.suit === suit);
}

function highestTrumpInTrick(trick: TrickCard[], trumpSuit: Suit): Rank | null {
  let best: Rank | null = null;
  let bestOrder = -1;
  for (const tc of trick) {
    if (tc.card.suit === trumpSuit) {
      const order = TRUMP_RANK_ORDER[tc.card.rank];
      if (order > bestOrder) {
        bestOrder = order;
        best = tc.card.rank;
      }
    }
  }
  return best;
}

function applyOverTrump(trumpCards: Card[], trick: TrickCard[], trumpSuit: Suit): Card[] {
  const highest = highestTrumpInTrick(trick, trumpSuit);
  if (highest === null) return [];
  const highestOrder = TRUMP_RANK_ORDER[highest];
  return trumpCards.filter((c) => TRUMP_RANK_ORDER[c.rank] > highestOrder);
}

function applyMustOverplayLedSuit(
  suitCards: Card[],
  trick: TrickCard[],
  ledSuit: Suit,
  trumpSuit: Suit,
): Card[] {
  const rankOrder = ledSuit === trumpSuit ? TRUMP_RANK_ORDER : NON_TRUMP_RANK_ORDER;
  let bestOrder = -1;
  for (const tc of trick) {
    if (tc.card.suit === ledSuit) {
      const order = rankOrder[tc.card.rank];
      if (order > bestOrder) bestOrder = order;
    }
  }
  if (bestOrder < 0) return [];
  return suitCards.filter((c) => rankOrder[c.rank] > bestOrder);
}

/**
 * Mirror of server/internal/game/validation.go `legalCards`. Returns the subset
 * of the seat's hand that is legal to play given the current trick state.
 * Bitola variant: follow the led suit when held — the overplay obligation (play
 * a higher led-suit card) applies only while the led suit can still win (led
 * suit is trump, or no trump has cut the trick yet); once a trump cuts a
 * non-trump-led trick, any card of the led suit is legal. When void in the led
 * suit, must cut with trump if any is held (over-trump if possible, otherwise
 * any trump) — no partner-winning exemption; otherwise any card is legal.
 */
export function legalCards(state: MatchState, seat: number): Card[] {
  const player = state.players[seat];
  if (!player) return [];
  const hand = player.hand ?? [];

  // currentTrick can be null on the wire during trick resolution (server emits
  // nil between tricks, which JSON-serializes to null). Treat null/empty as
  // "leading the next trick" → any card is legal.
  const currentTrick = state.currentTrick ?? [];
  if (currentTrick.length === 0) return hand;
  if (state.trumpSuit === null) return hand;

  const trumpSuit = state.trumpSuit;
  // Derive the led suit from the first card in the trick rather than
  // state.leadSuit. The server sends event:card_played and event:match_state
  // as separate messages; between them the client briefly has currentTrick
  // populated but leadSuit still stale from the previous trick's cleanup.
  // currentTrick[0] is atomically consistent with the current trick, so
  // using it closes that race — no "all cards legal" flash.
  const first = currentTrick[0];
  if (!first) return hand;
  const ledSuit = first.card.suit;
  const suitCards = filterBySuit(hand, ledSuit);

  if (suitCards.length > 0) {
    // Overplay applies only while the led suit can still win: led suit is
    // trump, or no trump has cut the trick yet. Once a trump cuts a non-trump-
    // led trick, any card of the led suit is legal.
    const ledSuitCanWin =
      ledSuit === trumpSuit || highestTrumpInTrick(currentTrick, trumpSuit) === null;
    if (ledSuitCanWin) {
      const higher = applyMustOverplayLedSuit(suitCards, currentTrick, ledSuit, trumpSuit);
      if (higher.length > 0) return higher;
    }
    return suitCards;
  }

  const trumpCards = filterBySuit(hand, trumpSuit);
  if (trumpCards.length > 0) {
    const overTrumps = applyOverTrump(trumpCards, currentTrick, trumpSuit);
    if (overTrumps.length > 0) return overTrumps;
    return trumpCards;
  }

  return hand;
}

export function legalCardIds(state: MatchState, seat: number): string[] {
  return legalCards(state, seat).map((c) => `${c.rank}${c.suit}`);
}

/**
 * True when playing `card` would trigger a Belote/Rebelote announcement: it is
 * the trump King or Queen and the hand (before playing) also holds the partner
 * (trump Queen or King). Mirrors the server's `shouldPromptBelot` so the client
 * can prompt the player BEFORE sending play_card — the card must not be thrown
 * (or shown to opponents) until the player decides to announce or pass.
 */
export function isBelotEligible(card: Card, hand: Card[], trump: Suit): boolean {
  if (card.suit !== trump) return false;
  if (card.rank !== "K" && card.rank !== "Q") return false;
  const partner: Rank = card.rank === "K" ? "Q" : "K";
  return hand.some((c) => c.suit === trump && c.rank === partner);
}
