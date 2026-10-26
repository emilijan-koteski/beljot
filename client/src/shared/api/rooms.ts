import { axiosClient } from "@/shared/api/axiosClient";
import type {
  CreateRoomRequest,
  InvitableFriend,
  QuickPlayResponse,
  Room,
  RoomDetail,
  RoomPlayer,
  SelectSeatResponse,
} from "@/shared/types/apiTypes";

export function createRoom(req: CreateRoomRequest): Promise<Room> {
  return axiosClient.post("/rooms", req);
}

export function getRooms(status: string = "waiting"): Promise<Room[]> {
  return axiosClient.get("/rooms", { params: { status } });
}

export function getRoom(id: number): Promise<RoomDetail> {
  return axiosClient.get(`/rooms/${id}`);
}

export function getRoomByCode(code: string): Promise<RoomDetail> {
  return axiosClient.get(`/rooms/code/${encodeURIComponent(code)}`);
}

/** Join a room, sending a body only when a private-room password is given. */
export function joinRoom(id: number, password?: string): Promise<Room> {
  return axiosClient.post(`/rooms/${id}/join`, password !== undefined ? { password } : undefined);
}

/** The caller's friends, each flagged with whether they can be invited into this room now. */
export function listInvitableFriends(roomId: number): Promise<InvitableFriend[]> {
  return axiosClient.get(`/rooms/${roomId}/invitable-friends`);
}

/**
 * Idle lobby players who are not the caller's friends and accept their invites, filtered by an
 * optional username substring. The server sorts and caps the list; opted-out players are absent.
 */
export function listInvitablePlayers(roomId: number, search: string): Promise<InvitableFriend[]> {
  return axiosClient.get(`/rooms/${roomId}/invitable-players`, {
    params: search === "" ? undefined : { search },
  });
}

/** Invite a friend or lobby player into this room; the server decides any password bypass from the room owner. */
export function inviteToRoom(roomId: number, friendUserId: number): Promise<void> {
  return axiosClient.post(`/rooms/${roomId}/invite`, { friendUserId });
}

/** Void the caller's own outstanding invite to this room; a missing invite is a no-op. */
export function declineRoomInvite(roomId: number): Promise<void> {
  return axiosClient.post(`/rooms/${roomId}/invite/decline`);
}

/** Set or change the room password, or revert the room to public. */
export function updateRoomPrivacy(
  roomId: number,
  body: { isPrivate: boolean; password?: string },
): Promise<Room> {
  return axiosClient.post(`/rooms/${roomId}/privacy`, body);
}

export function leaveRoom(id: number): Promise<void> {
  return axiosClient.post(`/rooms/${id}/leave`);
}

/** Reopen a finished room for another match, keeping the caller's seat; 404 NOT_IN_ROOM if they left. */
export function returnToRoom(roomId: number): Promise<RoomDetail> {
  return axiosClient.post(`/rooms/${roomId}/return`);
}

export function selectSeat(roomId: number, seat: number): Promise<SelectSeatResponse> {
  return axiosClient.post(`/rooms/${roomId}/seat`, { seat });
}

export function leaveSeat(roomId: number): Promise<{ players: RoomPlayer[] }> {
  return axiosClient.post(`/rooms/${roomId}/leave-seat`);
}

export function transferOwnership(roomId: number, userId: number): Promise<{ ownerId: number }> {
  return axiosClient.post(`/rooms/${roomId}/transfer-ownership`, { userId });
}

export function startMatch(roomId: number): Promise<Room> {
  return axiosClient.post(`/rooms/${roomId}/start`);
}

export function quickPlay(signal?: AbortSignal): Promise<QuickPlayResponse> {
  return axiosClient.post("/rooms/quick-play", undefined, { signal });
}

/** Join a specific quick-play room with auto-seating, returning the same shape as quickPlay. */
export function quickJoin(id: number): Promise<QuickPlayResponse> {
  return axiosClient.post(`/rooms/${id}/quick-join`);
}

export function kickPlayer(roomId: number, userId: number): Promise<{ playerCount: number }> {
  return axiosClient.post(`/rooms/${roomId}/kick`, { userId });
}

export function addBot(roomId: number, seat: number): Promise<{ players: RoomPlayer[] }> {
  return axiosClient.post(`/rooms/${roomId}/bots`, { seat });
}

export function removeBot(roomId: number, seat: number): Promise<{ players: RoomPlayer[] }> {
  return axiosClient.delete(`/rooms/${roomId}/bots/${seat}`);
}

export function swapSeats(
  roomId: number,
  seatA: number,
  seatB: number,
): Promise<{ players: RoomPlayer[] }> {
  return axiosClient.post(`/rooms/${roomId}/swap-seats`, { seatA, seatB });
}
