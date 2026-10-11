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
 * Mirror of the server's `legalCards`: the cards in a seat's hand that may be played into the current trick.
 * The led suit comes from the trick's first card, since `state.leadSuit` can lag behind a `card_played` event.
 */
export function legalCards(state: MatchState, seat: number): Card[] {
  const player = state.players[seat];
  if (!player) return [];
  const hand = player.hand ?? [];

  const currentTrick = state.currentTrick ?? [];
  if (currentTrick.length === 0) return hand;
  if (state.trumpSuit === null) return hand;

  const trumpSuit = state.trumpSuit;
  const first = currentTrick[0];
  if (!first) return hand;
  const ledSuit = first.card.suit;
  const suitCards = filterBySuit(hand, ledSuit);

  if (suitCards.length > 0) {
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
 * True when playing `card` would trigger Belote: it is the trump King or Queen and the hand also holds its partner.
 * Mirrors the server's `shouldPromptBelot` so the client can prompt before sending play_card.
 */
export function isBelotEligible(card: Card, hand: Card[], trump: Suit): boolean {
  if (card.suit !== trump) return false;
  if (card.rank !== "K" && card.rank !== "Q") return false;
  const partner: Rank = card.rank === "K" ? "Q" : "K";
  return hand.some((c) => c.suit === trump && c.rank === partner);
}
