import type { Room } from "@/shared/types/apiTypes";

export const ROOM_SEATS = 4;

type ListedRoom = Pick<Room, "players" | "playerCount">;

/**
 * People and bots in a listed room. `playerCount` counts humans only, so it misses bots;
 * the player list carries both, the way the server counts a room as full.
 */
export function roomOccupancy(room: ListedRoom): number {
  return room.players ? room.players.length : room.playerCount;
}

/** True once humans and bots fill all four seats, so nobody else can join. */
export function isRoomFull(room: ListedRoom): boolean {
  return roomOccupancy(room) >= ROOM_SEATS;
}

/** Seats the host has given to bots. */
export function roomBotCount(room: Pick<Room, "players">): number {
  return room.players?.filter((p) => p.isBot === true).length ?? 0;
}
