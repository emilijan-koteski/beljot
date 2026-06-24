import { beforeEach, describe, expect, it } from "vitest";

import { useRoomStore } from "./roomStore";

describe("roomStore", () => {
  beforeEach(() => {
    useRoomStore.getState().reset();
    // reset() deliberately preserves insolventEjection (Story 9.3), so clear it
    // explicitly between tests to keep them isolated.
    useRoomStore.getState().setInsolventEjection(null);
  });

  it("initializes with null room, empty players, and matchStarted false", () => {
    const state = useRoomStore.getState();
    expect(state.room).toBeNull();
    expect(state.players).toEqual([]);
    expect(state.matchStarted).toBe(false);
  });

  it("addPlayer adds a new player to the list", () => {
    const player = {
      id: 1,
      roomId: 10,
      userId: 42,
      username: "Alice",
      seat: null,
      team: null,
      isBot: false,
      createdAt: "2026-04-12T00:00:00Z",
    };
    useRoomStore.getState().addPlayer(player, 2);

    const state = useRoomStore.getState();
    expect(state.players).toHaveLength(1);
    expect(state.players[0]!.username).toBe("Alice");
  });

  it("addPlayer updates room playerCount", () => {
    useRoomStore.getState().setRoom({
      id: 10,
      name: "Test",
      code: "ABC123",
      ownerId: 1,
      ownerUsername: "owner",
      variant: "bitola",
      matchMode: "1001",
      timerStyle: "relaxed",
      timerDurationSeconds: null,
      status: "waiting",
      playerCount: 1,
      isQuickPlay: false,
      coinBuyIn: 0,
      isPrivate: false,
      createdAt: "2026-04-12T00:00:00Z",
      updatedAt: "2026-04-12T00:00:00Z",
    });

    const player = {
      id: 2,
      roomId: 10,
      userId: 42,
      username: "Alice",
      seat: null,
      team: null,
      isBot: false,
      createdAt: "2026-04-12T00:00:00Z",
    };
    useRoomStore.getState().addPlayer(player, 2);

    expect(useRoomStore.getState().room?.playerCount).toBe(2);
  });

  it("addPlayer does not duplicate players with the same userId", () => {
    const player = {
      id: 1,
      roomId: 10,
      userId: 42,
      username: "Alice",
      seat: null,
      team: null,
      isBot: false,
      createdAt: "2026-04-12T00:00:00Z",
    };
    useRoomStore.getState().addPlayer(player, 1);
    useRoomStore.getState().addPlayer(player, 2);

    expect(useRoomStore.getState().players).toHaveLength(1);
  });

  it("removePlayer removes a player by userId", () => {
    const player = {
      id: 1,
      roomId: 10,
      userId: 42,
      username: "Alice",
      seat: null,
      team: null,
      isBot: false,
      createdAt: "2026-04-12T00:00:00Z",
    };
    useRoomStore.getState().addPlayer(player, 1);
    useRoomStore.getState().removePlayer(42, 0);

    expect(useRoomStore.getState().players).toHaveLength(0);
  });

  it("removePlayer updates ownerId when newOwnerId is provided", () => {
    useRoomStore.getState().setRoom({
      id: 10,
      name: "Test",
      code: "ABC123",
      ownerId: 42,
      ownerUsername: "owner",
      variant: "bitola",
      matchMode: "1001",
      timerStyle: "relaxed",
      timerDurationSeconds: null,
      status: "waiting",
      playerCount: 2,
      isQuickPlay: false,
      coinBuyIn: 0,
      isPrivate: false,
      createdAt: "2026-04-12T00:00:00Z",
      updatedAt: "2026-04-12T00:00:00Z",
    });

    useRoomStore.getState().removePlayer(42, 1, 99);

    expect(useRoomStore.getState().room?.ownerId).toBe(99);
  });

  it("updatePlayerSeat modifies a player's seat and team", () => {
    const player = {
      id: 1,
      roomId: 10,
      userId: 42,
      username: "Alice",
      seat: null,
      team: null,
      isBot: false,
      createdAt: "2026-04-12T00:00:00Z",
    };
    useRoomStore.getState().addPlayer(player, 1);

    useRoomStore.getState().updatePlayerSeat(42, 2, "teamA", null);

    const updated = useRoomStore.getState().players[0]!;
    expect(updated.seat).toBe(2);
    expect(updated.team).toBe("teamA");
  });

  it("addBot inserts a synthetic seat-keyed bot entry and skips duplicates", () => {
    const store = useRoomStore.getState();
    store.addBot(10, 1, "teamB");
    store.addBot(10, 3, "teamB");
    // Duplicate seat is a no-op (e.g. REST response raced the WS event).
    store.addBot(10, 1, "teamB");

    const players = useRoomStore.getState().players;
    expect(players).toHaveLength(2);
    expect(players[0]).toMatchObject({
      userId: 0,
      username: "",
      seat: 1,
      team: "teamB",
      isBot: true,
    });
    expect(players[1]).toMatchObject({ seat: 3, isBot: true });
  });

  it("removeBotBySeat removes only the bot at that seat, never humans or other bots", () => {
    const store = useRoomStore.getState();
    store.addPlayer(
      {
        id: 1,
        roomId: 10,
        userId: 42,
        username: "Alice",
        seat: 2,
        team: "teamA",
        isBot: false,
        createdAt: "",
      },
      1,
    );
    store.addBot(10, 1, "teamB");
    store.addBot(10, 3, "teamB");

    useRoomStore.getState().removeBotBySeat(1);

    const players = useRoomStore.getState().players;
    expect(players).toHaveLength(2);
    expect(players.some((p) => p.isBot === true && p.seat === 1)).toBe(false);
    expect(players.some((p) => p.isBot === true && p.seat === 3)).toBe(true);
    expect(players.some((p) => p.userId === 42)).toBe(true);
  });

  it("applies the human↔bot swap event sequence (seat_updated, bot_removed, bot_added)", () => {
    // Server order for an owner swapping bob (seat 2) with a bot (seat 1):
    // system:seat_updated moves the human first, then system:bot_removed
    // clears the bot's old seat and system:bot_added lands it on the
    // human's vacated one.
    const store = useRoomStore.getState();
    store.addPlayer(
      {
        id: 1,
        roomId: 10,
        userId: 42,
        username: "Bob",
        seat: 2,
        team: "teamA",
        isBot: false,
        createdAt: "",
      },
      1,
    );
    store.addBot(10, 1, "teamB");

    useRoomStore.getState().updatePlayerSeat(42, 1, "teamB", 2);
    useRoomStore.getState().removeBotBySeat(1);
    useRoomStore.getState().addBot(10, 2, "teamA");

    const players = useRoomStore.getState().players;
    expect(players).toHaveLength(2);
    expect(players.find((p) => p.userId === 42)).toMatchObject({ seat: 1, team: "teamB" });
    expect(players.find((p) => p.isBot === true)).toMatchObject({ seat: 2, team: "teamA" });
  });

  it("addBot is a no-op while ANY occupant still holds the seat (stale event guard)", () => {
    const store = useRoomStore.getState();
    store.addPlayer(
      {
        id: 1,
        roomId: 10,
        userId: 42,
        username: "Bob",
        seat: 2,
        team: "teamA",
        isBot: false,
        createdAt: "",
      },
      1,
    );

    // A bot_added for a seat a human still occupies is stale/out-of-order —
    // inserting would dual-occupy the seat in the store.
    useRoomStore.getState().addBot(10, 2, "teamA");

    const players = useRoomStore.getState().players;
    expect(players).toHaveLength(1);
    expect(players[0]!.isBot).toBe(false);
  });

  it("updatePlayerSeat never matches bot rows (all bots share userId 0)", () => {
    const store = useRoomStore.getState();
    store.addBot(10, 1, "teamB");
    // A (hypothetical) seat update for userId 0 must not drag the bot along.
    useRoomStore.getState().updatePlayerSeat(0, 2, "teamA", 1);

    const players = useRoomStore.getState().players;
    expect(players[0]).toMatchObject({ seat: 1, team: "teamB", isBot: true });
  });

  it("setMatchStarted sets the matchStarted flag", () => {
    useRoomStore.getState().setMatchStarted(true);
    expect(useRoomStore.getState().matchStarted).toBe(true);
  });

  it("reset clears all state", () => {
    useRoomStore.getState().setMatchStarted(true);
    useRoomStore.getState().addPlayer(
      {
        id: 1,
        roomId: 10,
        userId: 42,
        username: "Alice",
        seat: null,
        team: null,
        isBot: false,
        createdAt: "2026-04-12T00:00:00Z",
      },
      1,
    );

    useRoomStore.getState().reset();

    const state = useRoomStore.getState();
    expect(state.room).toBeNull();
    expect(state.players).toEqual([]);
    expect(state.matchStarted).toBe(false);
  });

  it("setReturnedUserIds replaces the presence set", () => {
    useRoomStore.getState().setReturnedUserIds([10, 20]);
    expect(useRoomStore.getState().returnedUserIds).toEqual([10, 20]);
  });

  it("markReturned appends a user once (idempotent)", () => {
    const { markReturned } = useRoomStore.getState();
    markReturned(10);
    markReturned(20);
    markReturned(10); // duplicate
    expect(useRoomStore.getState().returnedUserIds).toEqual([10, 20]);
  });

  it("setMatchStartedRoomId records the room whose match started", () => {
    useRoomStore.getState().setMatchStartedRoomId(7);
    expect(useRoomStore.getState().matchStartedRoomId).toBe(7);
    useRoomStore.getState().setMatchStartedRoomId(null);
    expect(useRoomStore.getState().matchStartedRoomId).toBeNull();
  });

  it("reset clears presence + match-start navigation state", () => {
    const store = useRoomStore.getState();
    store.setReturnedUserIds([1, 2, 3]);
    store.setMatchStartedRoomId(9);
    store.reset();
    expect(useRoomStore.getState().returnedUserIds).toEqual([]);
    expect(useRoomStore.getState().matchStartedRoomId).toBeNull();
  });

  // --- Insolvency ejection (Story 9.3) ---

  it("setInsolventEjection sets and clears the ejection notice", () => {
    useRoomStore.getState().setInsolventEjection({
      roomId: 7,
      buyIn: 500,
      balance: 100,
      reason: "ejected",
    });
    expect(useRoomStore.getState().insolventEjection).toEqual({
      roomId: 7,
      buyIn: 500,
      balance: 100,
      reason: "ejected",
    });

    useRoomStore.getState().setInsolventEjection(null);
    expect(useRoomStore.getState().insolventEjection).toBeNull();
  });

  it("reset PRESERVES the insolvency-ejection notice so the lobby modal survives RoomPage unmount", () => {
    useRoomStore.getState().setInsolventEjection({
      roomId: 7,
      buyIn: 500,
      balance: 100,
      reason: "roomClosed",
    });
    useRoomStore.getState().reset();
    // Survives reset (RoomPage calls reset() on unmount while navigating to lobby).
    expect(useRoomStore.getState().insolventEjection).toEqual({
      roomId: 7,
      buyIn: 500,
      balance: 100,
      reason: "roomClosed",
    });
    // ...but other state is cleared as usual.
    expect(useRoomStore.getState().matchStartedRoomId).toBeNull();
  });
});
