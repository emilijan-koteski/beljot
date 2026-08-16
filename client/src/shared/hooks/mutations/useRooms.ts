import { useMutation, useQueryClient } from "@tanstack/react-query";

import { queryKeys } from "@/shared/api/queryKeys";
import {
  addBot,
  createRoom,
  declineRoomInvite,
  inviteToRoom,
  joinRoom,
  kickPlayer,
  leaveRoom,
  leaveSeat,
  quickJoin,
  quickPlay,
  removeBot,
  selectSeat,
  startMatch,
  swapSeats,
  transferOwnership,
  updateRoomPrivacy,
} from "@/shared/api/rooms";
import type {
  CreateRoomRequest,
  QuickPlayResponse,
  Room,
  RoomPlayer,
  SelectSeatResponse,
} from "@/shared/types/apiTypes";

export function useCreateRoomMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: CreateRoomRequest) => createRoom(req),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.rooms.all });
    },
  });
}

export function useJoinRoomMutation() {
  const queryClient = useQueryClient();
  // Story 9.6: threads an optional private-room password through to joinRoom.
  // Public-room joins pass `{ id }` with no password (unchanged request shape).
  return useMutation<Room, Error, { id: number; password?: string }>({
    mutationFn: ({ id, password }) => joinRoom(id, password),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.rooms.all });
    },
  });
}

/**
 * Invite a friend into a waiting room (Story 11.5, AC2). On success the room's
 * invitable-friends roster is invalidated so the just-invited friend's row
 * re-renders against fresh availability (they may now be mid-join).
 *
 * Errors are surfaced by the caller — the panel renders them inline per row
 * rather than as a page-level toast.
 */
export function useInviteToRoomMutation() {
  const queryClient = useQueryClient();
  return useMutation<void, Error, { roomId: number; friendUserId: number }>({
    mutationFn: ({ roomId, friendUserId }) => inviteToRoom(roomId, friendUserId),
    onSuccess: (_data, { roomId }) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.rooms.invitableFriends(roomId) });
    },
  });
}

// Declining is fire-and-forget: the popup closes immediately either way, since a
// failed void only means the invite lapses on its own TTL instead of at once.
// Blocking the dismissal on a network round-trip would be a worse trade.
export function useDeclineRoomInviteMutation() {
  return useMutation<void, Error, number>({
    mutationFn: (roomId) => declineRoomInvite(roomId),
  });
}

export function useLeaveRoomMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => leaveRoom(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.rooms.all });
    },
  });
}

export function useSelectSeatMutation() {
  return useMutation<SelectSeatResponse, Error, { roomId: number; seat: number }>({
    mutationFn: ({ roomId, seat }) => selectSeat(roomId, seat),
  });
}

export function useLeaveSeatMutation() {
  return useMutation<{ players: RoomPlayer[] }, Error, { roomId: number }>({
    mutationFn: ({ roomId }) => leaveSeat(roomId),
  });
}

export function useTransferOwnershipMutation() {
  return useMutation<{ ownerId: number }, Error, { roomId: number; userId: number }>({
    mutationFn: ({ roomId, userId }) => transferOwnership(roomId, userId),
  });
}

export function useStartMatchMutation() {
  return useMutation({
    mutationFn: (roomId: number) => startMatch(roomId),
  });
}

export function useQuickPlayMutation() {
  return useMutation({
    mutationFn: (signal?: AbortSignal) => quickPlay(signal),
  });
}

export function useQuickJoinMutation() {
  const queryClient = useQueryClient();
  return useMutation<QuickPlayResponse, Error, number>({
    mutationFn: (id: number) => quickJoin(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.rooms.all });
    },
  });
}

export function useKickPlayerMutation() {
  return useMutation<{ playerCount: number }, Error, { roomId: number; userId: number }>({
    mutationFn: ({ roomId, userId }) => kickPlayer(roomId, userId),
  });
}

export function useAddBotMutation() {
  return useMutation<{ players: RoomPlayer[] }, Error, { roomId: number; seat: number }>({
    mutationFn: ({ roomId, seat }) => addBot(roomId, seat),
  });
}

export function useRemoveBotMutation() {
  return useMutation<{ players: RoomPlayer[] }, Error, { roomId: number; seat: number }>({
    mutationFn: ({ roomId, seat }) => removeBot(roomId, seat),
  });
}

export function useSwapSeatsMutation() {
  return useMutation<
    { players: RoomPlayer[] },
    Error,
    { roomId: number; seatA: number; seatB: number }
  >({
    mutationFn: ({ roomId, seatA, seatB }) => swapSeats(roomId, seatA, seatB),
  });
}

// Owner privacy edit (Story 9.6): set/change the password or revert to public.
export function useUpdateRoomPrivacyMutation() {
  const queryClient = useQueryClient();
  return useMutation<Room, Error, { roomId: number; isPrivate: boolean; password?: string }>({
    mutationFn: ({ roomId, isPrivate, password }) =>
      updateRoomPrivacy(roomId, { isPrivate, password }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.rooms.all });
    },
  });
}
