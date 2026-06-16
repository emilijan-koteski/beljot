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

/**
 * Mutates the cached `["rooms", "waiting"]` list in response to WS room
 * events so the lobby grid stays live without a manual refetch. The hook
 * itself is no-op — the dispatcher (useWsDispatch.ts) calls handleWsMessage
 * directly on each frame, and TanStack Query's subscribers re-render from
 * the mutated cache.
 */
export function useRoomUpdates() {
  // No-op — kept as a marker import so a future replacement of the dispatcher
  // wiring has a single deprecation site to update.
}

export function handleWsMessage(event: MessageEvent): void {
  try {
    const message = JSON.parse(String(event.data)) as WsMessage;

    switch (message.type) {
      case SYSTEM_ROOM_CREATED: {
        const payload = message.payload as RoomCreatedPayload;
        update((rooms) => {
          // De-dupe: WS race against an in-flight REST list query can deliver
          // the same room twice. setQueryData runs after the list resolves, so
          // we have to skip any id already in the cache.
          if (rooms.some((r) => r.id === payload.id)) return rooms;
          return [payload as unknown as Room, ...rooms];
        });
        return;
      }

      case SYSTEM_ROOM_UPDATED: {
        const payload = message.payload as RoomUpdatedPayload;
        if (payload.status !== "waiting") {
          // Room transitioned out of "waiting" (game started, closed) — drop.
          update((rooms) => rooms.filter((r) => r.id !== payload.id));
          return;
        }
        update((rooms) => {
          // D144: a reopened room (completed → waiting) broadcasts room_updated,
          // not room_created — but it was dropped from the grid when it went to
          // playing/completed, so a plain .map would silently lose it. Upsert:
          // add-if-missing (matching room_created's prepend) so strangers can
          // discover and join the reopened room live.
          if (!rooms.some((r) => r.id === payload.id)) {
            return [payload as unknown as Room, ...rooms];
          }
          return rooms.map((r) => {
            if (r.id !== payload.id) return r;
            // Prefer the payload's players[] when present (server includes
            // them on every lifecycle event now), but fall back to whatever
            // the cache already has so an older event without players still
            // doesn't blank the seat chips.
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
    // Ignore non-JSON / parse errors — same defensive stance as the original.
  }
}
