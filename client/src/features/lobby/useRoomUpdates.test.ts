import { afterEach, describe, expect, it } from "vitest";

import { queryClient } from "@/shared/api/queryClient";
import { queryKeys } from "@/shared/api/queryKeys";
import type { Room } from "@/shared/types/apiTypes";

import { handleWsMessage } from "./useRoomUpdates";

const WAITING = queryKeys.rooms.list("waiting");

function roomUpdated(payload: Record<string, unknown>): MessageEvent {
  return new MessageEvent("message", {
    data: JSON.stringify({ type: "system:room_updated", payload }),
  });
}

function room(id: number, extra: Record<string, unknown> = {}): Room {
  return {
    id,
    name: `Room ${id}`,
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
    players: [],
    createdAt: "",
    updatedAt: "",
    ...extra,
  } as unknown as Room;
}

describe("useRoomUpdates — D144 reopened room visibility", () => {
  afterEach(() => {
    queryClient.clear();
  });

  it("adds a reopened (waiting) room that is missing from an already-loaded grid", () => {
    queryClient.setQueryData<Room[]>(WAITING, [room(1)]);

    handleWsMessage(roomUpdated({ ...room(2), status: "waiting" }));

    const rooms = queryClient.getQueryData<Room[]>(WAITING) ?? [];
    expect(rooms.map((r) => r.id)).toEqual([2, 1]); // upserted (prepended)
  });

  it("updates an existing waiting room in place without duplicating", () => {
    queryClient.setQueryData<Room[]>(WAITING, [room(1, { playerCount: 1 })]);

    handleWsMessage(roomUpdated({ ...room(1), playerCount: 3 }));

    const rooms = queryClient.getQueryData<Room[]>(WAITING) ?? [];
    expect(rooms).toHaveLength(1);
    expect(rooms[0]!.playerCount).toBe(3);
  });

  it("removes a room that transitioned out of waiting", () => {
    queryClient.setQueryData<Room[]>(WAITING, [room(1)]);

    handleWsMessage(roomUpdated({ ...room(1), status: "playing" }));

    expect(queryClient.getQueryData<Room[]>(WAITING)).toEqual([]);
  });

  it("leaves an unloaded grid untouched (no cache to upsert into)", () => {
    handleWsMessage(roomUpdated({ ...room(9), status: "waiting" }));
    expect(queryClient.getQueryData<Room[]>(WAITING)).toBeUndefined();
  });
});
