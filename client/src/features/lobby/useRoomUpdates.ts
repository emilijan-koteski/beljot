import { queryClient } from "@/shared/api/queryClient";
import { queryKeys } from "@/shared/api/queryKeys";
import type { Room, RoomPlayer } from "@/shared/types/apiTypes";
import type {
  PlayerJoinedPayload,
  PlayerLeftPayload,
  RoomCreatedPayload,
  RoomUpdatedPayload,
  SeatUpdatedPayload,
  WsMessage,
} from "@/shared/types/wsEvents";
import {
  SYSTEM_PLAYER_JOINED,
  SYSTEM_PLAYER_LEFT,
  SYSTEM_ROOM_CREATED,
  SYSTEM_ROOM_UPDATED,
  SYSTEM_SEAT_UPDATED,
} from "@/shared/types/wsEvents";

const WAITING = queryKeys.rooms.list("waiting");

function update(updater: (rooms: Room[]) => Room[]) {
  queryClient.setQueryData<Room[]>(WAITING, (old) => (old ? updater(old) : undefined));
}

/** Applies a WS room event to the cached waiting-room list, ignoring frames that fail to parse. */
export function handleWsMessage(event: MessageEvent): void {
  try {
    const message = JSON.parse(String(event.data)) as WsMessage;

    switch (message.type) {
      case SYSTEM_ROOM_CREATED: {
        const payload = message.payload as RoomCreatedPayload;
        update((rooms) => {
          if (rooms.some((r) => r.id === payload.id)) return rooms;
          return [payload as unknown as Room, ...rooms];
        });
        return;
      }

      case SYSTEM_ROOM_UPDATED: {
        const payload = message.payload as RoomUpdatedPayload;
        if (payload.status !== "waiting") {
          update((rooms) => rooms.filter((r) => r.id !== payload.id));
          return;
        }
        update((rooms) => {
          if (!rooms.some((r) => r.id === payload.id)) {
            return [payload as unknown as Room, ...rooms];
          }
          return rooms.map((r) => {
            if (r.id !== payload.id) return r;
            const players = payload.players ?? r.players;
            return { ...r, ...(payload as unknown as Partial<Room>), players };
          });
        });
        return;
      }

      case SYSTEM_PLAYER_JOINED: {
        const payload = message.payload as PlayerJoinedPayload;
        update((rooms) =>
          rooms.map((r) => {
            if (r.id !== payload.roomId) return r;
            const next: RoomPlayer = {
              id: payload.userId,
              roomId: payload.roomId,
              userId: payload.userId,
              username: payload.username,
              avatarUrl: typeof payload.avatarUrl === "string" ? payload.avatarUrl : null,
              seat: null,
              team: null,
              isBot: false,
              createdAt: new Date().toISOString(),
            };
            const players = r.players ?? [];
            if (players.some((p) => p.userId === payload.userId)) {
              return { ...r, playerCount: payload.playerCount };
            }
            return {
              ...r,
              playerCount: payload.playerCount,
              players: [...players, next],
            };
          }),
        );
        return;
      }

      case SYSTEM_PLAYER_LEFT: {
        const payload = message.payload as PlayerLeftPayload;
        update((rooms) =>
          rooms.map((r) => {
            if (r.id !== payload.roomId) return r;
            return {
              ...r,
              playerCount: payload.playerCount,
              players: (r.players ?? []).filter((p) => p.userId !== payload.userId),
              ownerId: payload.newOwnerId ?? r.ownerId,
            };
          }),
        );
        return;
      }

      case SYSTEM_SEAT_UPDATED: {
        const payload = message.payload as SeatUpdatedPayload;
        update((rooms) =>
          rooms.map((r) => {
            if (r.id !== payload.roomId) return r;
            return {
              ...r,
              players: (r.players ?? []).map((p) =>
                p.userId === payload.userId ? { ...p, seat: payload.seat, team: payload.team } : p,
              ),
            };
          }),
        );
        return;
      }
    }
  } catch {
    return;
  }
}
