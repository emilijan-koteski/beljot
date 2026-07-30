import { create } from "zustand";

import type { Room, RoomPlayer } from "@/shared/types/apiTypes";

// Room-ejection notice (Story 9.3, widened by Story 9.8). Set by the return-time
// 409 handler (MatchPage), the per-user system:insolvent_ejected and
// system:honor_ejected events, and the system:room_closed_insolvent event — they
// all feed this single field so the lobby arrival modal is the one consumer.
//
// `reason` selects the copy:
//   "insolvent"  -> balance vs buy-in (Story 9.3)
//   "honor"      -> the player's honor vs the room's minimum (Story 9.8)
//   "roomClosed" -> the reason-agnostic room-closed notice
//
// The type is named for the ROOM EJECTION it represents rather than for
// insolvency, one of its three reasons: a field called `insolventEjection`
// holding `reason: "honor"` is a name that has stopped telling the truth, and
// 9.7's review spent ten patches undoing exactly that class of drift.
export interface RoomEjection {
  roomId: number;
  // Insolvency numbers — meaningful when reason === "insolvent".
  buyIn: number;
  balance: number;
  // Honor numbers — present only when reason === "honor". Optional rather than
  // defaulted, because a real honor score of 0 is legitimate and must never be
  // confused with "not supplied".
  minHonor?: number;
  honor?: number;
  reason: "insolvent" | "roomClosed" | "honor";
}

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
  // Room-ejection notice consumed by the lobby arrival modal (Story 9.3, 9.8).
  roomEjection: RoomEjection | null;
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
  setRoomEjection: (ejection: RoomEjection | null) => void;
  reset: () => void;
}

const initialState = {
  room: null,
  players: [],
  matchStarted: false,
  matchStartedRoomId: null,
  currentRoomId: null,
  kickedFromRoomId: null,
  roomEjection: null as RoomEjection | null,
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

  setRoomEjection: (roomEjection) => set({ roomEjection }),

  // Preserve the room-ejection notice across reset: RoomPage calls reset() on
  // unmount, which fires while we are navigating the ejected player to the
  // lobby — wiping it here would deny the lobby modal its one chance to render.
  // The modal clears the field itself on close (Story 9.3).
  reset: () => set((state) => ({ ...initialState, roomEjection: state.roomEjection })),
}));
