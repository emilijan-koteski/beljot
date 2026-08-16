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

// joinRoom posts an optional private-room password (Story 9.6). Public-room
// joins pass no password and send no body, preserving the pre-9.6 request shape.
export function joinRoom(id: number, password?: string): Promise<Room> {
  return axiosClient.post(`/rooms/${id}/join`, password !== undefined ? { password } : undefined);
}

// listInvitableFriends returns the caller's friends annotated with whether they
// can be invited into this room right now (Story 11.5, AC1). Caller must be a
// member of the (waiting) room; availability is computed server-side.
export function listInvitableFriends(roomId: number): Promise<InvitableFriend[]> {
  return axiosClient.get(`/rooms/${roomId}/invitable-friends`);
}

// inviteToRoom issues a friend invite into this room (Story 11.5, AC2). Whether
// the invite carries a password bypass is decided SERVER-side from the room's
// owner — deliberately no flag is sent here (D3).
export function inviteToRoom(roomId: number, friendUserId: number): Promise<void> {
  return axiosClient.post(`/rooms/${roomId}/invite`, { friendUserId });
}

// declineRoomInvite voids the caller's own outstanding invite to this room
// (Story 11.5). Called by the INVITEE, who is not in the room. Without it a
// declined invite stays consumable until its TTL lapses, so a "no thanks" would
// still leave the door open for a minute. Always succeeds server-side — a
// missing or already-expired invite is a no-op, not an error.
export function declineRoomInvite(roomId: number): Promise<void> {
  return axiosClient.post(`/rooms/${roomId}/invite/decline`);
}

// updateRoomPrivacy sets/changes the room password or reverts the room to public
// (Story 9.6). Owner-only + waiting-only server-side. Does not eject seated players.
export function updateRoomPrivacy(
  roomId: number,
  body: { isPrivate: boolean; password?: string },
): Promise<Room> {
  return axiosClient.post(`/rooms/${roomId}/privacy`, body);
}

export function leaveRoom(id: number): Promise<void> {
  return axiosClient.post(`/rooms/${id}/leave`);
}

// Reopens a finished room (status completed -> waiting) so the same group can
// play another match without recreating it. The caller's original seat is
// preserved server-side; rejects with 404 NOT_IN_ROOM if the caller was kicked
// or left, in which case the UI routes them back to the lobby.
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

// Joins a SPECIFIC quick-play room (the one tapped in the lobby grid),
// auto-seating the player and running the auto-start check. Returns the same
// shape as quickPlay so the caller can route to the matchmaking screen (or the
// game when this join filled the last seat).
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
