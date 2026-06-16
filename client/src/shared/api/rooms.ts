import { axiosClient } from "@/shared/api/axiosClient";
import type {
  CreateRoomRequest,
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

export function joinRoom(id: number): Promise<Room> {
  return axiosClient.post(`/rooms/${id}/join`);
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
