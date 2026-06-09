import { describe, expect, it } from "vitest";

import type { Card, MatchState, PlayerState, Suit, TrickCard } from "@/shared/types/matchTypes";

import { isBelotEligible, legalCardIds, legalCards } from "./legalCards";

function card(id: string): Card {
  return { rank: id[0] as Card["rank"], suit: id[1] as Card["suit"] };
}

function hand(ids: string[]): Card[] {
  return ids.map(card);
}

function trickCard(id: string, seat: number): TrickCard {
  return { card: card(id), playerSeat: seat };
}

function player(seat: number, cards: Card[]): PlayerState {
  return {
    seat,
    hand: cards,
    userId: seat + 1,
    username: `p${seat}`,
    team: seat % 2 === 0 ? "teamA" : "teamB",
    declarations: [],
    connected: true,
  };
}

function makeState(args: {
  mySeat: number;
  myHand: Card[];
  trumpSuit: Suit | null;
  leadSuit: Suit | null;
  currentTrick: TrickCard[];
}): MatchState {
  const hands: Record<number, Card[]> = {
    0: [],
    1: [],
    2: [],
    3: [],
  };
  hands[args.mySeat] = args.myHand;
  return {
    id: 1,
    roomId: 1,
    variant: "bitola",
    matchMode: "1001",
    phase: "playing",
    ownerSeat: 0,
    handNumber: 1,
    dealerSeat: 0,
    trumpSuit: args.trumpSuit,
    trumpCallerSeat: null,
    trumpCandidate: null,
    biddingRound: 1,
    biddingPassCount: 0,
    deck: [],
    activePlayerSeat: args.mySeat,
    trickNumber: 1,
    currentTrick: args.currentTrick,
    leadSuit: args.leadSuit,
    trickWinnerSeat: null,
    awaitingDeclaration: false,
    declarationsResolved: false,
    players: [
      player(0, hands[0] ?? []),
      player(1, hands[1] ?? []),
      player(2, hands[2] ?? []),
      player(3, hands[3] ?? []),
    ],
    teamScores: [0, 0],
    handPoints: [0, 0],
    declarationPoints: [0, 0],
    belotPoints: [0, 0],
    tricksWon: [0, 0],
    pendingBelotSeat: null,
    belotAnnounced: false,
    winnerTeam: null,
    lastHandResult: null,
    turnExpiresAt: null,
    timerDurationSec: 30,
    previousPhase: "",
    pausedPlayers: [false, false, false, false],
    pauseUsed: [false, false, false, false],
    surrenderProposerSeat: null,
    surrenderUsed: [false, false, false, false],
    turnTimeRemaining: 30,
    disconnectedSeat: -1,
    reconnectExpiresAt: null,
    playerReconnectExpiresAt: [null, null, null, null],
  };
}

describe("legalCards", () => {
  it("returns full hand when leading", () => {
    const myHand = hand(["7S", "KD", "AC"]);
    const state = makeState({
      mySeat: 0,
      myHand,
      trumpSuit: "C",
      leadSuit: null,
      currentTrick: [],
    });
    expect(legalCards(state, 0)).toEqual(myHand);
  });

  it("must follow led suit and overplay highest non-trump led-suit card on the table", () => {
    // Trump is clubs; spades led with QS, then opponent KS (non-trump rank
    // order: K=5, Q=4). Hand has TS (6), 7S (0). Must overplay → only TS.
    const myHand = hand(["7S", "TS", "KD", "AC"]);
    const state = makeState({
      mySeat: 3,
      myHand,
      trumpSuit: "C",
      leadSuit: "S",
      currentTrick: [trickCard("QS", 0), trickCard("KS", 1), trickCard("7H", 2)],
    });
    expect(legalCards(state, 3).map((c) => `${c.rank}${c.suit}`)).toEqual(["TS"]);
  });

  it("falls back to all led non-trump suit cards when no overplay exists", () => {
    // Trump is diamonds; hearts led with KH. Hand: 7H (0), 8H (1) — both
    // below KH (5). No overplay possible → both 7H and 8H are legal (rule 2).
    const myHand = hand(["7H", "8H"]);
    const state = makeState({
      mySeat: 2,
      myHand,
      trumpSuit: "D",
      leadSuit: "H",
      currentTrick: [trickCard("KH", 1)],
    });
    expect(legalCards(state, 2).map((c) => `${c.rank}${c.suit}`)).toEqual(["7H", "8H"]);
  });

  it("must overplay led non-trump even when an opponent already trumped", () => {
    // Trump is diamonds. Hearts led with KH; seat 3 (opponent of seat 2)
    // trumped with 7D. Hand has 8H, AH. The trump in trick does not change
    // the led-suit overplay obligation: highest hearts on table is still KH,
    // so AH (7) must be played; 8H (1) is excluded.
    const myHand = hand(["8H", "AH", "KC"]);
    const state = makeState({
      mySeat: 2,
      myHand,
      trumpSuit: "D",
      leadSuit: "H",
      currentTrick: [trickCard("KH", 1), trickCard("7D", 3)],
    });
    expect(legalCards(state, 2).map((c) => `${c.rank}${c.suit}`)).toEqual(["AH"]);
  });

  it("multiple higher led-suit cards — all strictly higher are legal", () => {
    // Trump=diamonds. Hearts led with JH (non-trump rank 3). Hand has QH (4),
    // KH (5), AH (7) — all strictly higher than JH. The helper must return
    // every overplay, not just the maximum.
    const myHand = hand(["QH", "KH", "AH", "8C"]);
    const state = makeState({
      mySeat: 1,
      myHand,
      trumpSuit: "D",
      leadSuit: "H",
      currentTrick: [trickCard("JH", 0)],
    });
    expect(legalCards(state, 1).map((c) => `${c.rank}${c.suit}`)).toEqual(["QH", "KH", "AH"]);
  });

  it("must overplay led non-trump even when highest card on table came from partner", () => {
    // Partner exemption applies only to void-in-led-suit (rule 3). When the
    // player has led-suit cards, the overplay rule still applies regardless
    // of who played the current high card.
    // Trump=diamonds. Seat 2 (partner of seat 0) led KH; seat 1 (opp) plays 7H.
    // Seat 0's hand: 8H, AH. Must overplay → only AH.
    const myHand = hand(["8H", "AH"]);
    const state = makeState({
      mySeat: 0,
      myHand,
      trumpSuit: "D",
      leadSuit: "H",
      currentTrick: [trickCard("KH", 2), trickCard("7H", 1)],
    });
    expect(legalCards(state, 0).map((c) => `${c.rank}${c.suit}`)).toEqual(["AH"]);
  });

  it("must over-trump when led suit is trump and over-trump exists", () => {
    // Trump is spades, led spades with 9S (order 6). My hand has JS (7) and 7S (0).
    // Must play JS (only over-trump).
    const myHand = hand(["7S", "JS", "AH"]);
    const state = makeState({
      mySeat: 1,
      myHand,
      trumpSuit: "S",
      leadSuit: "S",
      currentTrick: [trickCard("9S", 0)],
    });
    expect(legalCards(state, 1).map((c) => `${c.rank}${c.suit}`)).toEqual(["JS"]);
  });

  it("falls back to any card of led trump suit when no over-trump", () => {
    // Trump is spades, led spades with JS (order 7, top). No over-trump possible.
    // Can play any spade.
    const myHand = hand(["7S", "9S", "AH"]);
    const state = makeState({
      mySeat: 1,
      myHand,
      trumpSuit: "S",
      leadSuit: "S",
      currentTrick: [trickCard("JS", 0)],
    });
    expect(legalCards(state, 1).map((c) => `${c.rank}${c.suit}`)).toEqual(["7S", "9S"]);
  });

  it("must trump when void in led suit and opponent is winning", () => {
    // Opponent (seat 0, team A) led 10D and is winning the trick.
    // I'm seat 1 (team B), void in diamonds, holding two clubs (trump) and a heart.
    // Must trump.
    const myHand = hand(["8C", "9C", "AH"]);
    const state = makeState({
      mySeat: 1,
      myHand,
      trumpSuit: "C",
      leadSuit: "D",
      currentTrick: [trickCard("TD", 0)],
    });
    const ids = legalCards(state, 1).map((c) => `${c.rank}${c.suit}`);
    expect(ids.sort()).toEqual(["8C", "9C"]);
  });

  it("must over-trump when void and opponent has already trumped", () => {
    // Led diamonds. Seat 2 (opponent of seat 1) trumped with 8C (order 1).
    // I'm seat 1, void in diamonds. My trumps: 7C (order 0), JC (order 7).
    // Must over-trump → only JC.
    const myHand = hand(["7C", "JC", "AH"]);
    const state = makeState({
      mySeat: 1,
      myHand,
      trumpSuit: "C",
      leadSuit: "D",
      currentTrick: [trickCard("TD", 0), trickCard("8C", 2)],
    });
    expect(legalCards(state, 1).map((c) => `${c.rank}${c.suit}`)).toEqual(["JC"]);
  });

  it("void in led suit with trump in hand → must cut even when partner is winning", () => {
    // Bitola variant has no partner-winning exemption: a void player who holds
    // any trump must cut. Seat 2 leads TD; seat 3 plays 7D. Seat 0 (partner of
    // seat 2 — team A) is void in D and holds trump clubs 8C/9C plus AH.
    // No trump is on the table yet, so the over-trump filter falls through and
    // both trumps are legal; AH (non-trump) must be excluded.
    const myHand = hand(["8C", "9C", "AH"]);
    const state = makeState({
      mySeat: 0,
      myHand,
      trumpSuit: "C",
      leadSuit: "D",
      currentTrick: [trickCard("TD", 2), trickCard("7D", 3)],
    });
    const ids = legalCards(state, 0).map((c) => `${c.rank}${c.suit}`);
    expect(ids.sort()).toEqual(["8C", "9C"]);
  });

  it("img_two: void with trump, partner already trumped — must over-trump, diamonds illegal", () => {
    // Trump = Hearts. Trick: cvetanka(seat 1) 9C, emilijan(seat 2) QH (trump),
    // irena(seat 3) 7C. Kiro(seat 0) is void in clubs and holds 9H + TH (over-
    // trump QH(2)) plus diamonds. Only the over-trumps are legal.
    const myHand = hand(["9H", "TH", "8D", "TD", "JD", "QD", "AD"]);
    const state = makeState({
      mySeat: 0,
      myHand,
      trumpSuit: "H",
      leadSuit: "C",
      currentTrick: [trickCard("9C", 1), trickCard("QH", 2), trickCard("7C", 3)],
    });
    const ids = legalCards(state, 0).map((c) => `${c.rank}${c.suit}`);
    expect(ids.sort()).toEqual(["9H", "TH"]);
  });

  it("img_three: void with one trump, partner winning non-trump — only that trump", () => {
    // Trump = Spades. Trick: cvetanka(seat 1) 9C, kiro(seat 2) AC (partner of
    // seat 0 — team A), irena(seat 3) TC. Emilijan(seat 0) is void in clubs and
    // holds the single trump 8S plus J♦ + K♦. With no trump on the table yet,
    // the only legal play is 8S; both diamonds must be excluded.
    const myHand = hand(["8S", "JD", "KD"]);
    const state = makeState({
      mySeat: 0,
      myHand,
      trumpSuit: "S",
      leadSuit: "C",
      currentTrick: [trickCard("9C", 1), trickCard("AC", 2), trickCard("TC", 3)],
    });
    const ids = legalCards(state, 0).map((c) => `${c.rank}${c.suit}`);
    expect(ids).toEqual(["8S"]);
  });

  it("img_four: void in non-trump, no trump on table yet — every trump legal, no non-trumps", () => {
    // Trump = Hearts. Trick: irena(seat 1) 7S, cvetanka(seat 2) 9S, partner
    // emilijan(seat 3) plays a club (no spades, no trumps). Kiro(seat 0) is
    // void in spades and holds four hearts plus three diamonds. With no trump
    // on the table, every heart in hand is legal and every diamond is illegal.
    const myHand = hand(["7H", "8H", "TH", "KH", "7D", "8D", "TD"]);
    const state = makeState({
      mySeat: 0,
      myHand,
      trumpSuit: "H",
      leadSuit: "S",
      currentTrick: [trickCard("7S", 1), trickCard("9S", 2), trickCard("KC", 3)],
    });
    const ids = legalCards(state, 0).map((c) => `${c.rank}${c.suit}`);
    expect(ids.sort()).toEqual(["7H", "8H", "KH", "TH"]);
  });

  it("void with trump, partner already winning with a trump — falls through to any trump", () => {
    // Branch fallback: void in led suit, has trumps, but none over-trumps the
    // highest trump on the table (which happens to belong to partner). Must
    // still cut — non-trump card stays illegal.
    const myHand = hand(["QH", "KH", "7C"]);
    const state = makeState({
      mySeat: 0,
      myHand,
      trumpSuit: "H",
      leadSuit: "S",
      currentTrick: [trickCard("AS", 1), trickCard("9H", 2)],
    });
    const ids = legalCards(state, 0).map((c) => `${c.rank}${c.suit}`);
    expect(ids.sort()).toEqual(["KH", "QH"]);
  });

  it("void in led suit, no trump held → any card", () => {
    const myHand = hand(["AH", "KH", "7S"]);
    const state = makeState({
      mySeat: 1,
      myHand,
      trumpSuit: "C",
      leadSuit: "D",
      currentTrick: [trickCard("TD", 0)],
    });
    const ids = legalCards(state, 1).map((c) => `${c.rank}${c.suit}`);
    expect(ids.sort()).toEqual(["7S", "AH", "KH"]);
  });

  it("derives led suit from currentTrick[0] when leadSuit is stale/null (race between card_played and match_state)", () => {
    // Simulates the window after event:card_played has appended the opener to
    // currentTrick but the follow-up event:match_state hasn't synced leadSuit yet.
    // Without deriving from currentTrick, the leadSuit===null fallback would
    // return the full hand and all cards would flash as legal.
    const myHand = hand(["7S", "KS", "AH", "9D"]);
    const state = makeState({
      mySeat: 0,
      myHand,
      trumpSuit: "C",
      leadSuit: null,
      currentTrick: [trickCard("KD", 3)],
    });
    expect(legalCards(state, 0).map((c) => `${c.rank}${c.suit}`)).toEqual(["9D"]);
  });

  it("tolerates currentTrick === null on the wire (server emits nil between tricks)", () => {
    const myHand = hand(["7S", "KD", "AC"]);
    const state = makeState({
      mySeat: 0,
      myHand,
      trumpSuit: "C",
      leadSuit: null,
      currentTrick: [],
    });
    // Simulate server sending null instead of [] — cast to bypass the type lie.
    (state as unknown as { currentTrick: TrickCard[] | null }).currentTrick = null;
    expect(legalCards(state, 0)).toEqual(myHand);
  });

  it("legalCardIds returns the string-id form", () => {
    const myHand = hand(["7S", "KD"]);
    const state = makeState({
      mySeat: 0,
      myHand,
      trumpSuit: "C",
      leadSuit: null,
      currentTrick: [],
    });
    expect(legalCardIds(state, 0).sort()).toEqual(["7S", "KD"]);
  });
});

describe("isBelotEligible", () => {
  const trump: Suit = "H";

  it("is true for the trump King when the hand also holds the trump Queen", () => {
    expect(isBelotEligible(card("KH"), hand(["KH", "QH", "7S"]), trump)).toBe(true);
  });

  it("is true for the trump Queen when the hand also holds the trump King", () => {
    expect(isBelotEligible(card("QH"), hand(["QH", "KH", "AD"]), trump)).toBe(true);
  });

  it("is false for a trump King without the trump Queen in hand", () => {
    expect(isBelotEligible(card("KH"), hand(["KH", "9H", "7S"]), trump)).toBe(false);
  });

  it("is false for a non-trump King even with the same-suit Queen", () => {
    expect(isBelotEligible(card("KS"), hand(["KS", "QS"]), trump)).toBe(false);
  });

  it("is false for trump ranks other than King or Queen", () => {
    expect(isBelotEligible(card("AH"), hand(["AH", "KH", "QH"]), trump)).toBe(false);
  });
});
