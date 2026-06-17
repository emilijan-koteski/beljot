import { create } from "zustand";

import type { Room, RoomPlayer } from "@/shared/types/apiTypes";

export interface RoomState {
  room: Room | null;
  players: RoomPlayer[];
  matchStarted: boolean;
  // Room whose match just started (from system:match_started). Consumed by the
  // always-mounted match-start navigator (D145b) so a seated player is routed
  // into the match even when not on RoomPage; cleared after navigating.
  matchStartedRoomId: number | null;
  currentRoomId: number | null;
  kickedFromRoomId: number | null;
  // User IDs of players "present" in a reopened room (returned via "Return to
  // room" or freshly joined). The owner Start button is gated on every seated
  // human appearing here; seats whose human is absent show "waiting to return".
  returnedUserIds: number[];

  setRoom: (room: Room | null) => void;
  setPlayers: (players: RoomPlayer[]) => void;
  setCurrentRoomId: (roomId: number | null) => void;
  setReturnedUserIds: (userIds: number[]) => void;
  markReturned: (userId: number) => void;
  addPlayer: (player: RoomPlayer, playerCount: number) => void;
  removePlayer: (userId: number, playerCount: number, newOwnerId?: number) => void;
  updatePlayerSeat: (
    userId: number,
    seat: number | null,
    team: string | null,
    previousSeat: number | null,
  ) => void;
  // Bot seating (Story 10.3). All bots share userId 0, so bot mutations MUST
  // match by seat — never by userId. Human paths above stay userId-keyed.
  addBot: (roomId: number, seat: number, team: string) => void;
  removeBotBySeat: (seat: number) => void;
  setRoomOwner: (newOwnerId: number) => void;
  setMatchStarted: (started: boolean) => void;
  setMatchStartedRoomId: (roomId: number | null) => void;
  setKickedFromRoom: (roomId: number | null) => void;
  reset: () => void;
}

const initialState = {
  room: null,
  players: [],
  matchStarted: false,
  matchStartedRoomId: null,
  currentRoomId: null,
  kickedFromRoomId: null,
  returnedUserIds: [] as number[],
};

export const useRoomStore = create<RoomState>((set) => ({
  ...initialState,

  setRoom: (room) => set({ room }),

  setPlayers: (players) => set({ players }),

  setCurrentRoomId: (currentRoomId) => set({ currentRoomId }),

  setReturnedUserIds: (returnedUserIds) => set({ returnedUserIds }),

  markReturned: (userId) =>
    set((state) =>
      state.returnedUserIds.includes(userId)
        ? state
        : { returnedUserIds: [...state.returnedUserIds, userId] },
    ),

  addPlayer: (player, playerCount) =>
    set((state) => ({
      players: state.players.some((p) => p.userId === player.userId)
        ? state.players
        : [...state.players, player],
      room: state.room ? { ...state.room, playerCount } : state.room,
    })),

  removePlayer: (userId, playerCount, newOwnerId) =>
    set((state) => ({
      players: state.players.filter((p) => p.userId !== userId),
      room: state.room
        ? {
            ...state.room,
            playerCount,
            ownerId: newOwnerId ?? state.room.ownerId,
          }
        : state.room,
    })),

  updatePlayerSeat: (userId, seat, team, _previousSeat) =>
    set((state) => ({
      players: state.players.map((p) =>
        p.userId === userId && p.isBot !== true ? { ...p, seat, team } : p,
      ),
    })),

  addBot: (roomId, seat, team) =>
    set((state) => ({
      // Guard against ANY occupant on the seat, not just a bot: the server's
      // human↔bot swap emits seat_updated (human moves off) BEFORE bot_added
      // on the same ordered socket, so a still-occupied seat means this event
      // is stale/out-of-order — inserting would dual-occupy the seat in the
      // store. The next full setPlayers snapshot converges either way.
      players: state.players.some((p) => p.seat === seat)
        ? state.players
        : [
            ...state.players,
            {
              id: 0,
              roomId,
              userId: 0,
              username: "",
              seat,
              team,
              isBot: true,
              createdAt: new Date().toISOString(),
            },
          ],
    })),

  removeBotBySeat: (seat) =>
    set((state) => ({
      players: state.players.filter((p) => !(p.isBot === true && p.seat === seat)),
    })),

  setRoomOwner: (newOwnerId) =>
    set((state) => ({
      room: state.room ? { ...state.room, ownerId: newOwnerId } : state.room,
    })),

  setMatchStarted: (matchStarted) => set({ matchStarted }),

  setMatchStartedRoomId: (matchStartedRoomId) => set({ matchStartedRoomId }),

  setKickedFromRoom: (kickedFromRoomId) => set({ kickedFromRoomId }),

  reset: () => set(initialState),
}));
