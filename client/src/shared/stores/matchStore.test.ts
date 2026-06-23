import { beforeEach, describe, expect, it } from "vitest";

import type { MatchState } from "@/shared/types/matchTypes";

import { useMatchStore } from "./matchStore";

const mockMatchState: MatchState = {
  id: 1,
  roomId: 100,
  variant: "bitola",
  matchMode: "1001",
  phase: "bidding",
  handNumber: 1,
  dealerSeat: 0,
  trumpSuit: null,
  trumpCallerSeat: null,
  trumpCandidate: { rank: "K", suit: "S" },
  biddingRound: 1,
  biddingPassCount: 0,
  deck: [],
  activePlayerSeat: 1,
  trickNumber: 0,
  currentTrick: [],
  leadSuit: null,
  trickWinnerSeat: null,
  awaitingDeclaration: false,
  declarationsResolved: false,
  players: [
    {
      hand: [],
      seat: 0,
      userId: 10,
      username: "Alice",
      team: "teamA",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
    },
    {
      hand: [],
      seat: 1,
      userId: 20,
      username: "Bob",
      team: "teamB",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
    },
    {
      hand: [],
      seat: 2,
      userId: 30,
      username: "Carol",
      team: "teamA",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
    },
    {
      hand: [],
      seat: 3,
      userId: 40,
      username: "Dave",
      team: "teamB",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
    },
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
  timerDurationSec: 0,
  previousPhase: "" as const,
  pausedPlayers: [false, false, false, false] as [boolean, boolean, boolean, boolean],
  pauseUsed: [false, false, false, false] as [boolean, boolean, boolean, boolean],
  surrenderProposerSeat: null,
  surrenderUsed: [false, false, false, false] as [boolean, boolean, boolean, boolean],
  turnTimeRemaining: 0,
  ownerSeat: 0,
  disconnectedSeat: -1,
  reconnectExpiresAt: null,
  playerReconnectExpiresAt: [null, null, null, null] as [
    string | null,
    string | null,
    string | null,
    string | null,
  ],
};

describe("matchStore", () => {
  beforeEach(() => {
    useMatchStore.getState().reset();
  });

  it("initializes with null matchState", () => {
    const state = useMatchStore.getState();
    expect(state.matchState).toBeNull();
    expect(state.myPlayerSeat).toBeNull();
    expect(state.roomId).toBeNull();
  });

  it("sets full game state via setMatchState", () => {
    useMatchStore.getState().setMatchState(mockMatchState);

    const state = useMatchStore.getState();
    expect(state.matchState).toEqual(mockMatchState);
    expect(state.roomId).toBe(100);
  });

  it("sets myPlayerSeat", () => {
    useMatchStore.getState().setMyPlayerSeat(2);
    expect(useMatchStore.getState().myPlayerSeat).toBe(2);
  });

  it("clears all game data via clearGame including scoreRevealData and matchEndData", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(1);
    useMatchStore.getState().setScoreRevealData({
      teamACardPoints: 70,
      teamBCardPoints: 82,
      teamADeclPoints: 0,
      teamBDeclPoints: 0,
      lastTrickTeam: 1,
      lastTrickBonus: 10,
      capot: false,
      capotTeam: null,
      capotBonus: 0,
      failedContract: false,
      contractingTeam: 1,
      teamAHandTotal: 70,
      teamBHandTotal: 92,
      teamAMatchScore: 70,
      teamBMatchScore: 92,
    });
    useMatchStore.getState().setMatchEndData({
      winnerTeam: 0,
      teamAFinalScore: 1020,
      teamBFinalScore: 850,
      matchDurationSec: 300,
    });

    useMatchStore.getState().clearGame();

    const state = useMatchStore.getState();
    expect(state.matchState).toBeNull();
    expect(state.myPlayerSeat).toBeNull();
    expect(state.roomId).toBeNull();
    expect(state.scoreRevealData).toBeNull();
    expect(state.matchEndData).toBeNull();
  });

  it("resets to initial state via reset", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setLoading(true);

    useMatchStore.getState().reset();

    const state = useMatchStore.getState();
    expect(state.matchState).toBeNull();
    expect(state.isLoading).toBe(false);
  });

  it("initializes activeEmotes with all four slots null", () => {
    const state = useMatchStore.getState();
    expect(state.activeEmotes).toEqual({ 0: null, 1: null, 2: null, 3: null });
  });

  it("setActiveEmote writes a per-seat slot with a receivedAt stamp", () => {
    useMatchStore.getState().setActiveEmote(2, "thumbs_up");

    const slot = useMatchStore.getState().activeEmotes[2];
    expect(slot).not.toBeNull();
    expect(slot?.emote).toBe("thumbs_up");
    expect(typeof slot?.receivedAt).toBe("number");
    // Other seats untouched
    expect(useMatchStore.getState().activeEmotes[0]).toBeNull();
    expect(useMatchStore.getState().activeEmotes[1]).toBeNull();
    expect(useMatchStore.getState().activeEmotes[3]).toBeNull();
  });

  it("setActiveEmote replaces an existing slot without leaking the previous value", () => {
    useMatchStore.getState().setActiveEmote(1, "clap");
    const first = useMatchStore.getState().activeEmotes[1];

    useMatchStore.getState().setActiveEmote(1, "laugh");
    const second = useMatchStore.getState().activeEmotes[1];

    expect(second?.emote).toBe("laugh");
    // The slot must be a fresh object — mutating the previous one would
    // break React's reference-equality re-render guard.
    expect(second).not.toBe(first);
  });

  it("setActiveEmote(seat, null) clears the slot", () => {
    useMatchStore.getState().setActiveEmote(0, "heart");
    expect(useMatchStore.getState().activeEmotes[0]).not.toBeNull();

    useMatchStore.getState().setActiveEmote(0, null);
    expect(useMatchStore.getState().activeEmotes[0]).toBeNull();
  });

  it("clearGame zeroes all four emote slots", () => {
    useMatchStore.getState().setActiveEmote(0, "thumbs_up");
    useMatchStore.getState().setActiveEmote(1, "clap");
    useMatchStore.getState().setActiveEmote(2, "laugh");
    useMatchStore.getState().setActiveEmote(3, "heart");

    useMatchStore.getState().clearGame();

    expect(useMatchStore.getState().activeEmotes).toEqual({
      0: null,
      1: null,
      2: null,
      3: null,
    });
  });

  it("initializes activeDeclares with all four slots null", () => {
    const state = useMatchStore.getState();
    expect(state.activeDeclares).toEqual({ 0: null, 1: null, 2: null, 3: null });
  });

  it("setActiveDeclare writes a per-seat slot with a receivedAt stamp", () => {
    useMatchStore.getState().setActiveDeclare(2, true);

    const slot = useMatchStore.getState().activeDeclares[2];
    expect(slot).not.toBeNull();
    expect(typeof slot?.receivedAt).toBe("number");
    // Other seats untouched
    expect(useMatchStore.getState().activeDeclares[0]).toBeNull();
    expect(useMatchStore.getState().activeDeclares[1]).toBeNull();
    expect(useMatchStore.getState().activeDeclares[3]).toBeNull();
  });

  it("setActiveDeclare(seat, false) clears the slot", () => {
    useMatchStore.getState().setActiveDeclare(0, true);
    expect(useMatchStore.getState().activeDeclares[0]).not.toBeNull();

    useMatchStore.getState().setActiveDeclare(0, false);
    expect(useMatchStore.getState().activeDeclares[0]).toBeNull();
  });

  it("setActiveDeclare ignores out-of-range seats", () => {
    useMatchStore.getState().setActiveDeclare(7, true);
    expect(useMatchStore.getState().activeDeclares).toEqual({
      0: null,
      1: null,
      2: null,
      3: null,
    });
  });

  it("clearGame zeroes all four declare slots", () => {
    useMatchStore.getState().setActiveDeclare(1, true);
    useMatchStore.getState().setActiveDeclare(3, true);

    useMatchStore.getState().clearGame();

    expect(useMatchStore.getState().activeDeclares).toEqual({
      0: null,
      1: null,
      2: null,
      3: null,
    });
  });

  it("initializes lastEmoteSentAt to 0", () => {
    expect(useMatchStore.getState().lastEmoteSentAt).toBe(0);
  });

  it("setLastEmoteSentAt updates the value", () => {
    useMatchStore.getState().setLastEmoteSentAt(12345.6);
    expect(useMatchStore.getState().lastEmoteSentAt).toBe(12345.6);
  });

  it("clearGame resets lastEmoteSentAt to 0 (AC8)", () => {
    useMatchStore.getState().setLastEmoteSentAt(12345.6);
    useMatchStore.getState().clearGame();
    expect(useMatchStore.getState().lastEmoteSentAt).toBe(0);
  });

  it("setPendingResolvedTrick stores the snapshot with a receivedAt stamp", () => {
    useMatchStore.getState().setPendingResolvedTrick({
      trick: [{ card: { rank: "K", suit: "S" }, playerSeat: 0 }],
      winnerSeat: 2,
    });

    const snap = useMatchStore.getState().pendingResolvedTrick;
    expect(snap).not.toBeNull();
    expect(snap?.trick).toHaveLength(1);
    expect(snap?.winnerSeat).toBe(2);
    expect(typeof snap?.receivedAt).toBe("number");
  });

  it("setPendingResolvedTrick(null) clears the snapshot", () => {
    useMatchStore.getState().setPendingResolvedTrick({
      trick: [{ card: { rank: "K", suit: "S" }, playerSeat: 0 }],
      winnerSeat: 2,
    });
    useMatchStore.getState().setPendingResolvedTrick(null);
    expect(useMatchStore.getState().pendingResolvedTrick).toBeNull();
  });

  it("clearGame resets pendingResolvedTrick", () => {
    useMatchStore.getState().setPendingResolvedTrick({
      trick: [{ card: { rank: "K", suit: "S" }, playerSeat: 0 }],
      winnerSeat: 2,
    });
    useMatchStore.getState().clearGame();
    expect(useMatchStore.getState().pendingResolvedTrick).toBeNull();
  });

  it("replaces matchState on subsequent setMatchState calls", () => {
    useMatchStore.getState().setMatchState(mockMatchState);

    const updatedState: MatchState = {
      ...mockMatchState,
      phase: "playing",
      trumpSuit: "H",
      activePlayerSeat: 2,
    };
    useMatchStore.getState().setMatchState(updatedState);

    const state = useMatchStore.getState();
    expect(state.matchState?.phase).toBe("playing");
    expect(state.matchState?.trumpSuit).toBe("H");
    expect(state.matchState?.activePlayerSeat).toBe(2);
  });
});
