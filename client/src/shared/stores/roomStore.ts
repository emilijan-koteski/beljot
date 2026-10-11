import { create } from "zustand";

import type { Room, RoomPlayer } from "@/shared/types/apiTypes";

/** Room-ejection notice shown once by the lobby arrival modal; `reason` selects the copy. */
interface RoomEjection {
  roomId: number;
  buyIn: number;
  balance: number;
  minHonor?: number;
  honor?: number;
  reason: "insolvent" | "roomClosed" | "honor";
}

/** An incoming room invite shown by RoomInviteModal; `isHostInvite` means no password prompt. */
export interface RoomInvite {
  inviteId: number;
  roomId: number;
  roomName: string;
  inviterUserId: number;
  inviterUsername: string;
  coinBuyIn: number;
  isPrivate: boolean;
  isHostInvite: boolean;
  minHonor: number;
  expiresAt: string;
}

interface RoomState {
  room: Room | null;
  players: RoomPlayer[];
  matchStarted: boolean;
  matchStartedRoomId: number | null;
  currentRoomId: number | null;
  kickedFromRoomId: number | null;
  roomEjection: RoomEjection | null;
  roomInvite: RoomInvite | null;
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
    avatarUrl?: string | null,
  ) => void;
  addBot: (roomId: number, seat: number, team: string) => void;
  removeBotBySeat: (seat: number) => void;
  setRoomOwner: (newOwnerId: number) => void;
  setMatchStarted: (started: boolean) => void;
  setMatchStartedRoomId: (roomId: number | null) => void;
  setKickedFromRoom: (roomId: number | null) => void;
  setRoomEjection: (ejection: RoomEjection | null) => void;
  setRoomInvite: (invite: RoomInvite | null) => void;
  reset: () => void;
  clearSessionNotices: () => void;
}

const initialState = {
  room: null,
  players: [],
  matchStarted: false,
  matchStartedRoomId: null,
  currentRoomId: null,
  kickedFromRoomId: null,
  roomEjection: null as RoomEjection | null,
  roomInvite: null as RoomInvite | null,
  returnedUserIds: [] as number[],
};

/** A taken seat means bot_added is stale, since the server's human-to-bot swap sends seat_updated first. */
const isSeatTaken = (players: RoomPlayer[], seat: number) => players.some((p) => p.seat === seat);

/** Notices that survive reset(), which RoomPage calls mid-navigation before the lobby can show them. */
const keepNotices = (state: RoomState) => ({
  roomEjection: state.roomEjection,
  roomInvite: state.roomInvite,
});

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

  updatePlayerSeat: (userId, seat, team, _previousSeat, avatarUrl) =>
    set((state) => ({
      players: state.players.map((p) =>
        p.userId === userId && p.isBot !== true
          ? { ...p, seat, team, ...(avatarUrl !== undefined ? { avatarUrl } : {}) }
          : p,
      ),
    })),

  addBot: (roomId, seat, team) =>
    set((state) => ({
      players: isSeatTaken(state.players, seat)
        ? state.players
        : [
            ...state.players,
            {
              id: 0,
              roomId,
              userId: 0,
              username: "",
              avatarUrl: null,
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

  setRoomInvite: (roomInvite) => set({ roomInvite }),

  reset: () =>
    set((state) => ({
      ...initialState,
      ...keepNotices(state),
    })),

  clearSessionNotices: () => set({ ...initialState }),
}));
