import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

// Stub sonner so toast assertions can run; the real module exports more than
// these methods but the dispatcher only touches `info`/`warning`/`success`/`error`.
vi.mock("sonner", () => ({
  toast: {
    info: vi.fn(),
    warning: vi.fn(),
    success: vi.fn(),
    error: vi.fn(),
  },
}));

import { toast } from "sonner";

import { queryClient } from "@/shared/api/queryClient";
import { queryKeys } from "@/shared/api/queryKeys";
import { useAuthStore } from "@/shared/stores/authStore";
import { useChatStore } from "@/shared/stores/chatStore";
import { useMatchStore } from "@/shared/stores/matchStore";
import { useRoomStore } from "@/shared/stores/roomStore";
import type { Room, User } from "@/shared/types/apiTypes";
import type { MatchState } from "@/shared/types/matchTypes";
import type { WsMessage } from "@/shared/types/wsEvents";

import { __resetWsDispatchStateForTests, useWsDispatch } from "./useWsDispatch";

const mockMatchState: MatchState = {
  id: 1,
  roomId: 100,
  variant: "bitola",
  matchMode: "1001",
  phase: "playing",
  handNumber: 1,
  dealerSeat: 0,
  trumpSuit: "S",
  trumpCallerSeat: 0,
  trumpCandidate: null,
  biddingRound: 1,
  biddingPassCount: 0,
  deck: [],
  activePlayerSeat: 1,
  trickNumber: 1,
  currentTrick: [],
  leadSuit: null,
  trickWinnerSeat: null,
  awaitingDeclaration: false,
  declarationsResolved: false,
  players: [
    {
      hand: [{ rank: "K", suit: "S" }],
      seat: 0,
      userId: 10,
      username: "Alice",
      team: "teamA",
      declarations: [],
      connected: true,
      isBot: false,
    },
    {
      hand: [{ rank: "7", suit: "H" }],
      seat: 1,
      userId: 20,
      username: "Bob",
      team: "teamB",
      declarations: [],
      connected: true,
      isBot: false,
    },
    {
      hand: [{ rank: "A", suit: "D" }],
      seat: 2,
      userId: 30,
      username: "Carol",
      team: "teamA",
      declarations: [],
      connected: true,
      isBot: false,
    },
    {
      hand: [{ rank: "9", suit: "C" }],
      seat: 3,
      userId: 40,
      username: "Dave",
      team: "teamB",
      declarations: [],
      connected: true,
      isBot: false,
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

describe("useWsDispatch", () => {
  beforeEach(() => {
    queryClient.clear();
    useMatchStore.getState().reset();
    useRoomStore.getState().reset();
    useChatStore.setState({ lobbyMessages: [], matchMessages: [], roomMessages: [] });
    __resetWsDispatchStateForTests();
    vi.restoreAllMocks();
  });

  it("routes system:room_created to queryClient room cache", () => {
    // Seed the cache with an empty rooms list
    queryClient.setQueryData<Room[]>(queryKeys.rooms.list("waiting"), []);

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    const message: WsMessage = {
      type: "system:room_created",
      payload: {
        id: 1,
        name: "Test Room",
        code: "ABC123",
        ownerId: 42,
        ownerUsername: "owner",
        variant: "bitola",
        matchMode: "1001",
        timerStyle: "relaxed",
        timerDurationSeconds: null,
        status: "waiting",
        playerCount: 1,
        createdAt: "2026-04-12T00:00:00Z",
        updatedAt: "2026-04-12T00:00:00Z",
      },
    };

    dispatch(message);

    const rooms = queryClient.getQueryData<Room[]>(queryKeys.rooms.list("waiting"));
    expect(rooms).toHaveLength(1);
    expect(rooms![0]!.id).toBe(1);
    expect(rooms![0]!.name).toBe("Test Room");
  });

  it("routes system:room_updated to queryClient room cache", () => {
    // Pre-populate with a room
    queryClient.setQueryData<Room[]>(queryKeys.rooms.list("waiting"), [
      {
        id: 2,
        name: "Old Name",
        code: "XYZ789",
        ownerId: 42,
        ownerUsername: "owner",
        variant: "bitola",
        matchMode: "1001",
        timerStyle: "relaxed",
        timerDurationSeconds: null,
        status: "waiting",
        playerCount: 1,
        isQuickPlay: false,
        coinBuyIn: 0,
        createdAt: "2026-04-12T00:00:00Z",
        updatedAt: "2026-04-12T00:00:00Z",
      },
    ]);

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "system:room_updated",
      payload: {
        id: 2,
        name: "Updated Name",
        code: "XYZ789",
        ownerId: 42,
        ownerUsername: "owner",
        variant: "bitola",
        matchMode: "1001",
        timerStyle: "per-move",
        timerDurationSeconds: 30,
        status: "waiting",
        playerCount: 2,
        createdAt: "2026-04-12T00:00:00Z",
        updatedAt: "2026-04-12T00:01:00Z",
      },
    });

    const rooms = queryClient.getQueryData<Room[]>(queryKeys.rooms.list("waiting"));
    expect(rooms).toHaveLength(1);
    expect(rooms![0]!.name).toBe("Updated Name");
    expect(rooms![0]!.playerCount).toBe(2);
  });

  it("removes room from cache when status is not waiting", () => {
    queryClient.setQueryData<Room[]>(queryKeys.rooms.list("waiting"), [
      {
        id: 3,
        name: "Active Room",
        code: "AAA111",
        ownerId: 42,
        ownerUsername: "owner",
        variant: "bitola",
        matchMode: "1001",
        timerStyle: "relaxed",
        timerDurationSeconds: null,
        status: "waiting",
        playerCount: 4,
        isQuickPlay: false,
        coinBuyIn: 0,
        createdAt: "2026-04-12T00:00:00Z",
        updatedAt: "2026-04-12T00:00:00Z",
      },
    ]);

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "system:room_updated",
      payload: {
        id: 3,
        name: "Active Room",
        code: "AAA111",
        ownerId: 42,
        ownerUsername: "owner",
        variant: "bitola",
        matchMode: "1001",
        timerStyle: "relaxed",
        timerDurationSeconds: null,
        status: "playing",
        playerCount: 4,
        createdAt: "2026-04-12T00:00:00Z",
        updatedAt: "2026-04-12T00:01:00Z",
      },
    });

    const rooms = queryClient.getQueryData<Room[]>(queryKeys.rooms.list("waiting"));
    expect(rooms).toHaveLength(0);
  });

  it("handles error: prefix without crashing", () => {
    const consoleSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "error:not_your_turn",
      payload: { code: "NOT_YOUR_TURN", message: "it is not your turn" },
    });

    expect(consoleSpy).toHaveBeenCalled();
    consoleSpy.mockRestore();
  });

  it("handles malformed message payload without crashing", () => {
    const consoleSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "unknown:prefix",
      payload: {},
    });

    expect(consoleSpy).toHaveBeenCalled();
    consoleSpy.mockRestore();
  });

  it("ignores auth events handled by useWebSocket", () => {
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    // These should not throw or route to any store
    dispatch({ type: "system:authenticated", payload: { userId: 42 } });
    dispatch({ type: "error:auth_failed", payload: { message: "bad token" } });

    // No rooms added to cache
    expect(queryClient.getQueryData(queryKeys.rooms.list("waiting"))).toBeUndefined();
  });

  it("dispatches event:match_state to matchStore", () => {
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "event:match_state",
      payload: mockMatchState,
    });

    const state = useMatchStore.getState();
    expect(state.matchState).not.toBeNull();
    expect(state.matchState?.phase).toBe("playing");
    expect(state.matchState?.roomId).toBe(100);
    expect(state.roomId).toBe(100);
  });

  it("does NOT clear declarationReveal / belotReveal on an unrelated event:match_state", () => {
    // A reveal must survive trailing/standalone match_states (e.g. the snapshot
    // another player's card-play broadcasts mid-reveal) — closing it on an
    // unrelated state push was the bug where the declaration dialog vanished
    // the instant anyone played. Stale-reveal clearing on reconnect now lives
    // in MatchPage's overlay-clear effect, not the dispatcher.
    useMatchStore.setState({
      declarationReveal: {
        winnerTeam: 0,
        declarations: [
          {
            playerSeat: 0,
            type: "sequence",
            cards: ["9S", "TS", "JS", "QS"],
            value: 50,
          },
        ],
      },
      belotReveal: {
        playerSeat: 0,
        team: 0,
        cardId: "QS",
      },
    });

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "event:match_state",
      payload: mockMatchState,
    });

    const state = useMatchStore.getState();
    expect(state.declarationReveal).not.toBeNull();
    expect(state.belotReveal).not.toBeNull();
    expect(state.matchState).not.toBeNull();
  });

  it("preserves declarationReveal across the trailing event:match_state in normal flow", () => {
    // Server flow (manager.go:513→554): event:declarations_resolved is
    // emitted, then event:match_state is broadcast for state sync. The
    // trailing snapshot must NOT wipe the reveal that just rendered, or
    // the user never sees declarations.
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "event:declarations_resolved",
      payload: {
        winnerTeam: 0,
        declarations: [
          {
            playerSeat: 0,
            type: "sequence",
            cards: ["9S", "TS", "JS", "QS"],
            value: 50,
          },
        ],
      },
    });
    expect(useMatchStore.getState().declarationReveal).not.toBeNull();

    dispatch({
      type: "event:match_state",
      payload: mockMatchState,
    });

    const state = useMatchStore.getState();
    expect(state.declarationReveal).not.toBeNull();
    expect(state.matchState).not.toBeNull();
  });

  it("preserves belotReveal across the trailing event:match_state in normal flow", () => {
    // Server flow (manager.go:609→614): event:belot_announced is emitted,
    // then event:match_state is broadcast. Same suppression contract as
    // declarations_resolved.
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "event:belot_announced",
      payload: {
        playerSeat: 0,
        team: 0,
        cardId: "QS",
      },
    });
    expect(useMatchStore.getState().belotReveal).not.toBeNull();

    dispatch({
      type: "event:match_state",
      payload: mockMatchState,
    });

    const state = useMatchStore.getState();
    expect(state.belotReveal).not.toBeNull();
    expect(state.matchState).not.toBeNull();
  });

  it("keeps the reveal across multiple event:match_state pushes (no single-shot wipe)", () => {
    // The reveal lives for its full countdown regardless of how many
    // match_states arrive while it's up — players keep acting (each play
    // broadcasts a snapshot) and the declaration dialog must not blink out.
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "event:declarations_resolved",
      payload: {
        winnerTeam: 0,
        declarations: [
          {
            playerSeat: 0,
            type: "sequence",
            cards: ["9S", "TS", "JS", "QS"],
            value: 50,
          },
        ],
      },
    });
    dispatch({ type: "event:match_state", payload: mockMatchState });
    expect(useMatchStore.getState().declarationReveal).not.toBeNull();

    dispatch({ type: "event:match_state", payload: mockMatchState });
    // Still up after a second (and any later) snapshot.
    expect(useMatchStore.getState().declarationReveal).not.toBeNull();
  });

  it("dispatches event:card_played to matchStore", () => {
    useMatchStore.getState().setMatchState(mockMatchState);

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "event:card_played",
      payload: { playerSeat: 1, cardId: "7H", autoPlayed: false },
    });

    const state = useMatchStore.getState();
    expect(state.matchState?.currentTrick).toHaveLength(1);
    expect(state.matchState?.currentTrick[0]!.playerSeat).toBe(1);
  });

  it("snapshots the full trick into pendingResolvedTrick on event:trick_resolved", () => {
    // Reproduces the production race: event:card_played for the 4th card
    // arrives, then event:trick_resolved arrives in the same tick. The
    // dispatcher must snapshot all four cards into pendingResolvedTrick
    // BEFORE clearing currentTrick, otherwise the collect animation can't
    // run because TrickArea sees currentTrick jump 3 → 0.
    useMatchStore.getState().setMatchState(mockMatchState);

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    // Simulate four card_played events filling the trick.
    dispatch({
      type: "event:card_played",
      payload: { playerSeat: 0, cardId: "KS", autoPlayed: false },
    });
    dispatch({
      type: "event:card_played",
      payload: { playerSeat: 1, cardId: "7H", autoPlayed: false },
    });
    dispatch({
      type: "event:card_played",
      payload: { playerSeat: 2, cardId: "AD", autoPlayed: false },
    });
    dispatch({
      type: "event:card_played",
      payload: { playerSeat: 3, cardId: "9C", autoPlayed: false },
    });
    expect(useMatchStore.getState().matchState?.currentTrick).toHaveLength(4);

    // Now the resolve event clears the trick.
    dispatch({
      type: "event:trick_resolved",
      payload: { winnerSeat: 2, winnerTeam: 0, cards: ["KS", "7H", "AD", "9C"] },
    });

    const state = useMatchStore.getState();
    // Live trick zeroed (server cleared it).
    expect(state.matchState?.currentTrick).toHaveLength(0);
    // Snapshot has all four cards + winner — drives the collect flight.
    expect(state.pendingResolvedTrick).not.toBeNull();
    expect(state.pendingResolvedTrick?.trick).toHaveLength(4);
    expect(state.pendingResolvedTrick?.winnerSeat).toBe(2);
  });

  it("does not snapshot when event:trick_resolved arrives with an already-empty trick", () => {
    // Reconnect-style edge case: the snapshot should only fire when there's
    // actually a trick to capture. An empty currentTrick means we'd render
    // nothing in the collect animation, so we don't bother.
    useMatchStore.getState().setMatchState(mockMatchState);

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "event:trick_resolved",
      payload: { winnerSeat: 1, winnerTeam: 1, cards: [] },
    });

    expect(useMatchStore.getState().pendingResolvedTrick).toBeNull();
  });

  it("dispatches event:hand_scored to matchStore and sets scoreRevealData", () => {
    useMatchStore.getState().setMatchState(mockMatchState);

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "event:hand_scored",
      payload: {
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
      },
    });

    const state = useMatchStore.getState();
    expect(state.matchState?.teamScores[0]).toBe(70);
    expect(state.matchState?.teamScores[1]).toBe(92);
    expect(state.scoreRevealData).not.toBeNull();
    expect(state.scoreRevealData?.teamACardPoints).toBe(70);
    expect(state.scoreRevealData?.capot).toBe(false);
  });

  it("zeroes handPoints and declarationPoints on event:hand_scored to clear stale potential", () => {
    useMatchStore.getState().setMatchState({
      ...mockMatchState,
      handPoints: [70, 82],
      declarationPoints: [0, 50],
      belotPoints: [0, 0],
    });

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "event:hand_scored",
      payload: {
        teamACardPoints: 70,
        teamBCardPoints: 82,
        teamADeclPoints: 0,
        teamBDeclPoints: 50,
        lastTrickTeam: 1,
        lastTrickBonus: 10,
        capot: false,
        capotTeam: null,
        capotBonus: 0,
        failedContract: false,
        contractingTeam: 1,
        teamAHandTotal: 70,
        teamBHandTotal: 142,
        teamAMatchScore: 70,
        teamBMatchScore: 142,
      },
    });

    const state = useMatchStore.getState();
    expect(state.matchState?.handPoints).toEqual([0, 0]);
    expect(state.matchState?.declarationPoints).toEqual([0, 0]);
  });

  it("dispatches event:match_end to matchStore", () => {
    useMatchStore.getState().setMatchState(mockMatchState);

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "event:match_end",
      payload: {
        winnerTeam: 0,
        teamAFinalScore: 1020,
        teamBFinalScore: 850,
        matchDurationSec: 300,
      },
    });

    const state = useMatchStore.getState();
    expect(state.matchState?.phase).toBe("match_end");
    expect(state.matchState?.teamScores[0]).toBe(1020);
    expect(state.matchState?.teamScores[1]).toBe(850);
    expect(state.matchEndData?.matchDurationSec).toBe(300);
  });

  it("dispatches event:trump_selected to matchStore.trumpReveal", () => {
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "event:trump_selected",
      payload: { playerSeat: 2, trumpSuit: "S", cardId: "7S" },
    });

    const reveal = useMatchStore.getState().trumpReveal;
    expect(reveal).not.toBeNull();
    expect(reveal?.playerSeat).toBe(2);
    expect(reveal?.trumpSuit).toBe("S");
    expect(reveal?.cardId).toBe("7S");
  });

  it("ignores malformed event:trump_selected payloads (cardId guard)", () => {
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "event:trump_selected",
      payload: { playerSeat: 2, trumpSuit: "S", cardId: "" },
    });
    dispatch({
      type: "event:trump_selected",
      payload: { playerSeat: 2, trumpSuit: "S", cardId: "X" },
    });

    expect(useMatchStore.getState().trumpReveal).toBeNull();
    expect(warnSpy).toHaveBeenCalled();
    warnSpy.mockRestore();
  });

  // --- Room lobby event dispatch tests ---

  it("dispatches system:bot_added / system:bot_removed to roomStore by seat", () => {
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;
    useRoomStore.getState().setCurrentRoomId(10);

    dispatch({ type: "system:bot_added", payload: { roomId: 10, seat: 2, team: "teamA" } });
    expect(useRoomStore.getState().players).toHaveLength(1);
    expect(useRoomStore.getState().players[0]).toMatchObject({
      userId: 0,
      seat: 2,
      team: "teamA",
      isBot: true,
    });

    dispatch({ type: "system:bot_removed", payload: { roomId: 10, seat: 2 } });
    expect(useRoomStore.getState().players).toHaveLength(0);
  });

  it("ignores bot events for other rooms and malformed payloads", () => {
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;
    useRoomStore.getState().setCurrentRoomId(10);

    dispatch({ type: "system:bot_added", payload: { roomId: 99, seat: 2, team: "teamA" } });
    dispatch({ type: "system:bot_added", payload: { roomId: 10, seat: "2", team: "teamA" } });
    dispatch({ type: "system:bot_added", payload: {} });
    expect(useRoomStore.getState().players).toHaveLength(0);
  });

  it("dispatches system:player_joined to roomStore", () => {
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "system:player_joined",
      payload: {
        roomId: 10,
        userId: 42,
        username: "Alice",
        playerCount: 2,
      },
    });

    const state = useRoomStore.getState();
    expect(state.players).toHaveLength(1);
    expect(state.players[0]!.userId).toBe(42);
    expect(state.players[0]!.username).toBe("Alice");
    expect(state.players[0]!.id).toBe(42); // Uses userId, not hardcoded 0 (D24 fix)
  });

  it("dispatches system:player_left to roomStore", () => {
    // Pre-populate a player
    useRoomStore.getState().addPlayer(
      {
        id: 42,
        roomId: 10,
        userId: 42,
        username: "Alice",
        seat: null,
        team: null,
        isBot: false,
        createdAt: "",
      },
      2,
    );

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "system:player_left",
      payload: {
        roomId: 10,
        userId: 42,
        username: "Alice",
        playerCount: 1,
      },
    });

    expect(useRoomStore.getState().players).toHaveLength(0);
  });

  it("dispatches system:seat_updated to roomStore", () => {
    useRoomStore.getState().addPlayer(
      {
        id: 42,
        roomId: 10,
        userId: 42,
        username: "Alice",
        seat: null,
        team: null,
        isBot: false,
        createdAt: "",
      },
      1,
    );

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "system:seat_updated",
      payload: {
        roomId: 10,
        userId: 42,
        username: "Alice",
        seat: 2,
        team: "teamA",
        previousSeat: null,
      },
    });

    const player = useRoomStore.getState().players[0]!;
    expect(player.seat).toBe(2);
    expect(player.team).toBe("teamA");
  });

  it("dispatches system:match_started to roomStore", () => {
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "system:match_started",
      payload: { roomId: 10 },
    });

    expect(useRoomStore.getState().matchStarted).toBe(true);
  });

  it("appends system:chat_message with scope=lobby to chatStore", () => {
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "system:chat_message",
      payload: {
        userId: 7,
        username: "alice",
        message: "hello world",
        timestamp: "2026-04-18T10:00:00Z",
        scope: "lobby",
      },
    });

    const messages = useChatStore.getState().lobbyMessages;
    expect(messages).toHaveLength(1);
    expect(messages[0]).toMatchObject({
      userId: 7,
      username: "alice",
      message: "hello world",
      scope: "lobby",
    });
  });

  it("appends system:chat_message with scope=match to chatStore.matchMessages when in a match", () => {
    // Dispatcher requires matchStore.roomId to be set (defence in depth)
    useMatchStore.setState({ roomId: 42 });

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "system:chat_message",
      payload: {
        userId: 7,
        username: "alice",
        message: "team only",
        timestamp: "2026-04-18T10:00:00Z",
        scope: "match",
      },
    });

    const state = useChatStore.getState();
    expect(state.matchMessages).toHaveLength(1);
    expect(state.matchMessages[0]).toMatchObject({
      userId: 7,
      username: "alice",
      message: "team only",
      scope: "match",
    });
    expect(state.lobbyMessages).toHaveLength(0);
  });

  it("drops scope=match payloads when matchStore.roomId is null (no active match)", () => {
    // roomId=null is the default from beforeEach reset — a match payload
    // arriving after clearGame must not leak into the next match's history.
    expect(useMatchStore.getState().roomId).toBeNull();

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "system:chat_message",
      payload: {
        userId: 7,
        username: "alice",
        message: "stale",
        timestamp: "2026-04-18T10:00:00Z",
        scope: "match",
      },
    });

    expect(useChatStore.getState().matchMessages).toHaveLength(0);
  });

  it("appends system:chat_message with scope=room to chatStore.roomMessages when in a room", () => {
    // Dispatcher requires roomStore.currentRoomId to be set (defence in depth)
    useRoomStore.setState({ currentRoomId: 5 });

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "system:chat_message",
      payload: {
        userId: 7,
        username: "alice",
        message: "room ping",
        timestamp: "2026-04-22T10:00:00Z",
        scope: "room",
      },
    });

    const state = useChatStore.getState();
    expect(state.roomMessages).toHaveLength(1);
    expect(state.roomMessages[0]).toMatchObject({
      userId: 7,
      username: "alice",
      message: "room ping",
      scope: "room",
    });
    expect(state.lobbyMessages).toHaveLength(0);
    expect(state.matchMessages).toHaveLength(0);
  });

  it("drops scope=room payloads when roomStore.currentRoomId is null", () => {
    // currentRoomId=null is the default from beforeEach reset — a stale room
    // payload arriving after leaving the room must not leak into the next
    // room's history.
    expect(useRoomStore.getState().currentRoomId).toBeNull();

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "system:chat_message",
      payload: {
        userId: 7,
        username: "alice",
        message: "stale",
        timestamp: "2026-04-22T10:00:00Z",
        scope: "room",
      },
    });

    expect(useChatStore.getState().roomMessages).toHaveLength(0);
  });

  it("scope=global does not leak into matchMessages (partition isolation)", () => {
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "system:chat_message",
      payload: {
        userId: 1,
        username: "alice",
        message: "global-only",
        timestamp: "2026-04-18T10:00:00Z",
        scope: "lobby",
      },
    });

    expect(useChatStore.getState().matchMessages).toHaveLength(0);
    expect(useChatStore.getState().lobbyMessages).toHaveLength(1);
  });

  it("ignores malformed system:chat_message payloads (defensive validation)", () => {
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    const malformedPayloads: unknown[] = [
      null,
      undefined,
      {},
      { userId: "not-a-number", username: "x", message: "x", timestamp: "x", scope: "lobby" },
      { userId: 1, username: null, message: "x", timestamp: "x", scope: "lobby" },
      { userId: 1, username: "x", message: 42, timestamp: "x", scope: "lobby" },
      { userId: 1, username: "x", message: "x", timestamp: undefined, scope: "lobby" },
      { userId: 1, username: "x", message: "x", timestamp: "x" /* no scope */ },
    ];

    for (const payload of malformedPayloads) {
      dispatch({ type: "system:chat_message", payload });
    }

    expect(useChatStore.getState().lobbyMessages).toHaveLength(0);
    expect(warnSpy).toHaveBeenCalled();
    warnSpy.mockRestore();
  });

  it("dispatches system:room_kicked to roomStore when currentRoomId matches", () => {
    useRoomStore.getState().setCurrentRoomId(5);
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "system:room_kicked",
      payload: { roomId: 5, reason: "kicked_by_owner" },
    });

    expect(useRoomStore.getState().kickedFromRoomId).toBe(5);
  });

  it("ignores system:room_kicked for a different room", () => {
    useRoomStore.getState().setCurrentRoomId(9);
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "system:room_kicked",
      payload: { roomId: 5, reason: "kicked_by_owner" },
    });

    expect(useRoomStore.getState().kickedFromRoomId).toBeNull();
  });

  it("ignores system:room_kicked when currentRoomId is null", () => {
    // currentRoomId=null means the user is not viewing any room. Setting
    // kickedFromRoomId here would persist as a sticky flag and trip on a
    // later re-entry to the same room — so the dispatcher requires a
    // positive room match.
    expect(useRoomStore.getState().currentRoomId).toBeNull();
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "system:room_kicked",
      payload: { roomId: 5, reason: "kicked_by_owner" },
    });

    expect(useRoomStore.getState().kickedFromRoomId).toBeNull();
  });

  // --- Insolvency ejection (Story 9.3) ---

  it("dispatches system:insolvent_ejected to roomStore as an 'ejected' signal", () => {
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "system:insolvent_ejected",
      payload: { roomId: 7, buyIn: 500, balance: 120 },
    });

    expect(useRoomStore.getState().insolventEjection).toEqual({
      roomId: 7,
      buyIn: 500,
      balance: 120,
      reason: "ejected",
    });
  });

  it("dispatches system:insolvent_ejected regardless of currentRoomId (direct per-user push)", () => {
    // currentRoomId is null (player not on any room page) — the event must still
    // set the signal because it is a direct per-user push.
    expect(useRoomStore.getState().currentRoomId).toBeNull();
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "system:insolvent_ejected",
      payload: { roomId: 3, buyIn: 1000, balance: 0 },
    });

    expect(useRoomStore.getState().insolventEjection?.reason).toBe("ejected");
    expect(useRoomStore.getState().insolventEjection?.roomId).toBe(3);
  });

  it("ignores a malformed system:insolvent_ejected payload", () => {
    // reset() deliberately preserves insolventEjection (so the lobby modal
    // survives RoomPage's unmount), so clear it explicitly for this assertion.
    useRoomStore.getState().setInsolventEjection(null);
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "system:insolvent_ejected",
      payload: { roomId: "7", buyIn: 500 } as never,
    });

    expect(useRoomStore.getState().insolventEjection).toBeNull();
  });

  it("dispatches system:room_closed_insolvent to roomStore as a 'roomClosed' signal", () => {
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "system:room_closed_insolvent",
      payload: { roomId: 9 },
    });

    expect(useRoomStore.getState().insolventEjection).toEqual({
      roomId: 9,
      buyIn: 0,
      balance: 0,
      reason: "roomClosed",
    });
  });

  // --- Surrender (Story 8.2) ---

  it("dispatches event:surrender_proposed to matchStore.surrenderProposed", () => {
    useMatchStore.getState().setMatchState(mockMatchState);

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "event:surrender_proposed",
      payload: {
        proposerSeat: 0,
        proposerTeam: 0,
        proposerUsername: "Alice",
        partnerSeat: 2,
      },
    });

    const state = useMatchStore.getState();
    expect(state.surrenderProposed).not.toBeNull();
    expect(state.surrenderProposed?.proposerSeat).toBe(0);
    expect(state.surrenderProposed?.partnerSeat).toBe(2);
  });

  it("ignores event:surrender_proposed when no active game state", () => {
    // Defence in depth — matchState=null means no active match, so the
    // proposal must not surface in the store.
    expect(useMatchStore.getState().matchState).toBeNull();

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "event:surrender_proposed",
      payload: {
        proposerSeat: 0,
        proposerTeam: 0,
        proposerUsername: "Alice",
        partnerSeat: 2,
      },
    });

    expect(useMatchStore.getState().surrenderProposed).toBeNull();
  });

  it("dispatches event:surrender_declined to matchStore.surrenderDeclined", () => {
    useMatchStore.getState().setMatchState(mockMatchState);

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "event:surrender_declined",
      payload: { proposerSeat: 0, decliningSeat: 2 },
    });

    const declined = useMatchStore.getState().surrenderDeclined;
    expect(declined).not.toBeNull();
    expect(declined?.proposerSeat).toBe(0);
    expect(declined?.decliningSeat).toBe(2);
  });

  it("fires info toast for event:auto_action of type pass_trump", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    const { result } = renderHook(() => useWsDispatch());
    result.current({
      type: "event:auto_action",
      payload: { playerSeat: 1, type: "pass_trump" },
    });
    expect(toast.info).toHaveBeenCalledTimes(1);
  });

  it("fires info toast for event:auto_action of type skip_declare", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    const { result } = renderHook(() => useWsDispatch());
    result.current({
      type: "event:auto_action",
      payload: { playerSeat: 2, type: "skip_declare" },
    });
    expect(toast.info).toHaveBeenCalledTimes(1);
  });

  it("fires info toast for event:auto_action of type skip_belot", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    const { result } = renderHook(() => useWsDispatch());
    result.current({
      type: "event:auto_action",
      payload: { playerSeat: 3, type: "skip_belot" },
    });
    expect(toast.info).toHaveBeenCalledTimes(1);
  });

  it("ignores event:auto_action when matchState is null", () => {
    const { result } = renderHook(() => useWsDispatch());
    result.current({
      type: "event:auto_action",
      payload: { playerSeat: 0, type: "pass_trump" },
    });
    expect(toast.info).not.toHaveBeenCalled();
  });

  it("ignores malformed event:auto_action payloads", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    const consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const { result } = renderHook(() => useWsDispatch());
    result.current({
      type: "event:auto_action",
      payload: { playerSeat: 99, type: "pass_trump" },
    });
    result.current({
      type: "event:auto_action",
      payload: { playerSeat: 1, type: "bogus" as unknown as "pass_trump" },
    });
    // NaN slips through `typeof === "number"`; the explicit Number.isInteger
    // guard must reject it so toasts never render "Player NaN".
    result.current({
      type: "event:auto_action",
      payload: { playerSeat: Number.NaN, type: "pass_trump" },
    });
    // Non-integer floats also rejected by the integer guard.
    result.current({
      type: "event:auto_action",
      payload: { playerSeat: 1.5, type: "pass_trump" },
    });
    expect(toast.info).not.toHaveBeenCalled();
    expect(consoleWarnSpy).toHaveBeenCalled();
    consoleWarnSpy.mockRestore();
  });

  it("propagates outcomeReason+surrenderedBySeat through event:match_end", () => {
    useMatchStore.getState().setMatchState(mockMatchState);

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "event:match_end",
      payload: {
        winnerTeam: 1,
        teamAFinalScore: 420,
        teamBFinalScore: 600,
        matchDurationSec: 280,
        outcomeReason: "surrender",
        surrenderedBySeat: 0,
      },
    });

    const data = useMatchStore.getState().matchEndData;
    expect(data?.outcomeReason).toBe("surrender");
    expect(data?.surrenderedBySeat).toBe(0);
  });

  it("does NOT crash on error:surrender_exhausted", () => {
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    expect(() =>
      dispatch({
        type: "error:surrender_exhausted",
        payload: { code: "SURRENDER_EXHAUSTED", message: "already used" },
      }),
    ).not.toThrow();
  });

  it("surfaces toast.error on error:surrender_exhausted", () => {
    vi.mocked(toast.error).mockClear();
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "error:surrender_exhausted",
      payload: { code: "SURRENDER_EXHAUSTED", message: "already used" },
    });

    // i18next without an initialized bundle in this test suite returns the
    // resolved key path or undefined depending on setup; the load-bearing
    // assertion is that the dispatcher calls toast.error exactly once for
    // this event type, which the previous "does NOT crash" test missed.
    expect(toast.error).toHaveBeenCalledTimes(1);
  });

  it("writes ERROR_SURRENDER_EXHAUSTED to matchStore.lastError", () => {
    expect(useMatchStore.getState().lastError).toBeNull();
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "error:surrender_exhausted",
      payload: { code: "SURRENDER_EXHAUSTED", message: "already used" },
    });

    expect(useMatchStore.getState().lastError).toBe("error:surrender_exhausted");
  });

  // --- Story 8.5-1 AC2: error:match_start_failed ---

  it("dispatches error:match_start_failed via toast and writes lastError", () => {
    expect(useMatchStore.getState().lastError).toBeNull();
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "error:match_start_failed",
      payload: {
        roomId: 42,
        message: "Failed to start the game. Please try again.",
      },
    });

    expect(useMatchStore.getState().lastError).toBe("error:match_start_failed");
    expect(toast.error).toHaveBeenCalledTimes(1);
  });

  it("ignores malformed error:match_start_failed payloads", () => {
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "error:match_start_failed",
      payload: { unrelatedField: true } as unknown as { roomId?: number; message?: string },
    });

    expect(toast.error).not.toHaveBeenCalled();
    expect(warnSpy).toHaveBeenCalled();
    warnSpy.mockRestore();
  });

  // --- Emote (Story 8.3) ---

  it("writes a valid system:emote payload to matchStore.activeEmotes", () => {
    useMatchStore.getState().setMatchState(mockMatchState);

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "system:emote",
      payload: { playerSeat: 2, emote: "thumbs_up" },
    });

    const slot = useMatchStore.getState().activeEmotes[2];
    expect(slot).not.toBeNull();
    expect(slot?.emote).toBe("thumbs_up");
    expect(typeof slot?.receivedAt).toBe("number");
  });

  it("ignores malformed system:emote payloads (defensive validation)", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    const malformedPayloads: unknown[] = [
      null,
      undefined,
      {},
      { playerSeat: -1, emote: "thumbs_up" },
      { playerSeat: 4, emote: "thumbs_up" },
      { playerSeat: "2", emote: "thumbs_up" },
      { playerSeat: 0, emote: "shrug" },
      { playerSeat: 0, emote: 42 },
      { playerSeat: 0 /* no emote */ },
    ];

    for (const payload of malformedPayloads) {
      dispatch({ type: "system:emote", payload });
    }

    // No slots written.
    expect(useMatchStore.getState().activeEmotes).toEqual({
      0: null,
      1: null,
      2: null,
      3: null,
    });
    expect(warnSpy).toHaveBeenCalled();
    warnSpy.mockRestore();
  });

  it("ignores system:emote when no active game state (defence in depth)", () => {
    expect(useMatchStore.getState().matchState).toBeNull();

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "system:emote",
      payload: { playerSeat: 0, emote: "clap" },
    });

    expect(useMatchStore.getState().activeEmotes).toEqual({
      0: null,
      1: null,
      2: null,
      3: null,
    });
  });

  // --- Player declared (trick-1 "has a declaration" banner) ---

  it("writes a valid event:player_declared payload to matchStore.activeDeclares", () => {
    useMatchStore.getState().setMatchState(mockMatchState);

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "event:player_declared",
      payload: { playerSeat: 2 },
    });

    const slot = useMatchStore.getState().activeDeclares[2];
    expect(slot).not.toBeNull();
    expect(typeof slot?.receivedAt).toBe("number");
    // Other seats untouched.
    expect(useMatchStore.getState().activeDeclares[0]).toBeNull();
  });

  it("ignores malformed event:player_declared payloads (defensive validation)", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    const malformedPayloads: unknown[] = [
      null,
      undefined,
      {},
      { playerSeat: -1 },
      { playerSeat: 4 },
      { playerSeat: "2" },
    ];

    for (const payload of malformedPayloads) {
      dispatch({ type: "event:player_declared", payload });
    }

    expect(useMatchStore.getState().activeDeclares).toEqual({
      0: null,
      1: null,
      2: null,
      3: null,
    });
    expect(warnSpy).toHaveBeenCalled();
    warnSpy.mockRestore();
  });

  it("ignores event:player_declared when no active game state (defence in depth)", () => {
    expect(useMatchStore.getState().matchState).toBeNull();

    const { result } = renderHook(() => useWsDispatch());
    const dispatch = result.current;

    dispatch({
      type: "event:player_declared",
      payload: { playerSeat: 0 },
    });

    expect(useMatchStore.getState().activeDeclares).toEqual({
      0: null,
      1: null,
      2: null,
      3: null,
    });
  });
});

describe("useWsDispatch — coin settlement (Story 9.2)", () => {
  const baseUser: User = {
    id: 10,
    username: "Alice",
    email: "alice@test.dev",
    languagePreference: "en",
    walletBalance: 5000,
    loginStreakDays: 0,
    createdAt: "2026-06-18T00:00:00Z",
  };

  beforeEach(() => {
    useMatchStore.getState().reset();
    __resetWsDispatchStateForTests();
    vi.restoreAllMocks();
    useAuthStore.setState({ user: { ...baseUser } });
  });

  it("updates authStore walletBalance and stores the settlement on a positive delta (no toast)", () => {
    const { result } = renderHook(() => useWsDispatch());
    result.current({
      type: "event:coin_settlement",
      payload: { coinDelta: 500, newBalance: 5500, pot: 2000 },
    });

    expect(useAuthStore.getState().user?.walletBalance).toBe(5500);
    expect(useMatchStore.getState().coinSettlement).toEqual({
      coinDelta: 500,
      newBalance: 5500,
      pot: 2000,
    });
    // The win/loss amount now lives in the result dialog, not a toast.
    expect(toast.success).not.toHaveBeenCalled();
    expect(toast.info).not.toHaveBeenCalled();
  });

  it("updates balance and stores the settlement on a negative delta (no toast)", () => {
    const { result } = renderHook(() => useWsDispatch());
    result.current({
      type: "event:coin_settlement",
      payload: { coinDelta: -500, newBalance: 4500, pot: 2000 },
    });

    expect(useAuthStore.getState().user?.walletBalance).toBe(4500);
    expect(useMatchStore.getState().coinSettlement?.coinDelta).toBe(-500);
    expect(toast.info).not.toHaveBeenCalled();
    expect(toast.success).not.toHaveBeenCalled();
  });

  it("stores a zero-delta settlement (dialog decides not to render it)", () => {
    const { result } = renderHook(() => useWsDispatch());
    result.current({
      type: "event:coin_settlement",
      payload: { coinDelta: 0, newBalance: 5000, pot: 1000 },
    });

    expect(useAuthStore.getState().user?.walletBalance).toBe(5000);
    expect(useMatchStore.getState().coinSettlement?.coinDelta).toBe(0);
    expect(toast.success).not.toHaveBeenCalled();
    expect(toast.info).not.toHaveBeenCalled();
  });

  it("ignores a malformed settlement payload without touching balance or store", () => {
    const { result } = renderHook(() => useWsDispatch());
    const malformed = [
      { coinDelta: "500", newBalance: 5500, pot: 2000 },
      { coinDelta: 500, newBalance: null, pot: 2000 },
      { coinDelta: 1.5, newBalance: 5500, pot: 2000 },
      {},
    ];
    for (const payload of malformed) {
      result.current({ type: "event:coin_settlement", payload });
    }

    expect(useAuthStore.getState().user?.walletBalance).toBe(5000);
    expect(useMatchStore.getState().coinSettlement).toBeNull();
    expect(toast.success).not.toHaveBeenCalled();
    expect(toast.info).not.toHaveBeenCalled();
  });

  it("clears a prior settlement when a new match_end arrives", () => {
    const { result } = renderHook(() => useWsDispatch());
    result.current({
      type: "event:coin_settlement",
      payload: { coinDelta: 500, newBalance: 5500, pot: 2000 },
    });
    expect(useMatchStore.getState().coinSettlement).not.toBeNull();

    result.current({
      type: "event:match_end",
      payload: {
        winnerTeam: 0,
        teamAFinalScore: 1001,
        teamBFinalScore: 700,
        matchDurationSec: 120,
      },
    });

    // match_end resets the settlement so a subsequent free match can't show
    // the previous match's coin delta in the result dialog.
    expect(useMatchStore.getState().coinSettlement).toBeNull();
  });
});
