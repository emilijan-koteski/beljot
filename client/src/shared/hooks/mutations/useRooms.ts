import { useMutation, useQueryClient } from "@tanstack/react-query";

import { queryKeys } from "@/shared/api/queryKeys";
import {
  addBot,
  createRoom,
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
