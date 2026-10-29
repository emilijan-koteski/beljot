import { useState } from "react";
import { useNavigate } from "react-router";
import { toast } from "sonner";

import { FetchError } from "@/shared/api/axiosClient";
import { getRoomByCode } from "@/shared/api/rooms";
import { useJoinRoomMutation } from "@/shared/hooks/mutations/useRooms";
import { joinFailureMessage } from "@/shared/lib/joinFailure";
import type { Room } from "@/shared/types/apiTypes";

export const ROOM_CODE_LENGTH = 6;

/** Normalises what the player types into a room code: capitals and digits, at most six characters. */
export function normalizeRoomCode(raw: string): string {
  return raw
    .toUpperCase()
    .replace(/[^A-Z0-9]/g, "")
    .slice(0, ROOM_CODE_LENGTH);
}

/**
 * Joining a room by its code: resolve the code, ask for the password when the room is private, then join
 * and open the room. Failures become toasts, except a wrong password, which the prompt shows inline.
 */
export function useJoinByCode() {
  const navigate = useNavigate();
  const joinRoomMutation = useJoinRoomMutation();
  const [resolving, setResolving] = useState(false);
  const [privateRoom, setPrivateRoom] = useState<Room | null>(null);
  const [passwordErrorKey, setPasswordErrorKey] = useState<string | null>(null);

  async function joinResolved(room: Room, password?: string) {
    try {
      await joinRoomMutation.mutateAsync({ id: room.id, password });
      setPrivateRoom(null);
      navigate(`/rooms/${room.id}`);
    } catch (err) {
      const code = err instanceof FetchError ? err.code : null;
      if (code === "WRONG_ROOM_PASSWORD") {
        setPasswordErrorKey("room.errors.wrongPassword");
        return;
      }
      setPrivateRoom(null);
      toast.error(joinFailureMessage(code));
    }
  }

  async function submit(rawCode: string): Promise<boolean> {
    const code = normalizeRoomCode(rawCode);
    if (code.length !== ROOM_CODE_LENGTH || resolving || joinRoomMutation.isPending) return false;
    setResolving(true);
    try {
      const { room } = await getRoomByCode(code);
      setResolving(false);
      if (room.isPrivate) {
        setPasswordErrorKey(null);
        setPrivateRoom(room);
        return true;
      }
      await joinResolved(room);
      return true;
    } catch (err) {
      setResolving(false);
      toast.error(joinFailureMessage(err instanceof FetchError ? err.code : null));
      return false;
    }
  }

  return {
    submit,
    busy: resolving || joinRoomMutation.isPending,
    passwordPrompt: {
      open: privateRoom !== null,
      roomName: privateRoom?.name ?? "",
      pending: joinRoomMutation.isPending,
      errorKey: passwordErrorKey,
      onSubmit: (password: string) => {
        if (!privateRoom) return;
        setPasswordErrorKey(null);
        void joinResolved(privateRoom, password);
      },
      onClose: () => {
        setPrivateRoom(null);
        setPasswordErrorKey(null);
      },
    },
  };
}
