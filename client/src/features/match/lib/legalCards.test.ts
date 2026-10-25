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
    avatarUrl: null,
    team: seat % 2 === 0 ? "teamA" : "teamB",
    declarations: [],
    connected: true,
    isBot: false,
    level: 1,
    faceDownCount: 0,
    handCount: cards.length,
    declarationAnswered: false,
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
    mustPickTrump: false,
    declarationsEnabled: true,
    stopAtTarget: false,
    declarationsTransfer: true,
    clockwise: false,
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

  it("led non-trump cut by trump → any heart legal, off-suit and trump excluded", () => {
    const myHand = hand(["8H", "AH", "KC", "QD"]);
    const state = makeState({
      mySeat: 2,
      myHand,
      trumpSuit: "D",
      leadSuit: "H",
      currentTrick: [trickCard("KH", 1), trickCard("7D", 3)],
    });
    expect(
      legalCards(state, 2)
        .map((c) => `${c.rank}${c.suit}`)
        .sort(),
    ).toEqual(["8H", "AH"]);
  });

  it("multiple higher led-suit cards — all strictly higher are legal", () => {
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
