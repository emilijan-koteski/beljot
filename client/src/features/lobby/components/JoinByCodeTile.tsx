import { KeyRound } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { toast } from "sonner";

import { PasswordPromptDialog } from "@/features/lobby/components/PasswordPromptDialog";
import { FetchError } from "@/shared/api/axiosClient";
import { getRoomByCode } from "@/shared/api/rooms";
import { useJoinRoomMutation } from "@/shared/hooks/mutations/useRooms";
import { cn } from "@/shared/lib/utils";
import type { Room } from "@/shared/types/apiTypes";

const CODE_LENGTH = 6;

/**
 * 6-char uppercase code input + Join button inline. Replaces the older
 * <JoinByCode> sidebar tile; mirrors the design's hero action rail. Submits
 * on Enter and on button click.
 *
 * Story 9.6: a code can resolve to a private room. We resolve the code first
 * (getRoomByCode exposes room.isPrivate but never demands the password), then
 * prompt for the password before joining when the room is private.
 */
export function JoinByCodeTile() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [code, setCode] = useState("");
  const [resolving, setResolving] = useState(false);
  const [pendingPrivateRoom, setPendingPrivateRoom] = useState<Room | null>(null);
  const [passwordErrorKey, setPasswordErrorKey] = useState<string | null>(null);
  const joinRoomMutation = useJoinRoomMutation();
  const isValid = code.trim().length === CODE_LENGTH;
  const busy = resolving || joinRoomMutation.isPending;

  // Maps a join/lookup failure to a toast. WRONG_ROOM_PASSWORD is handled inline
  // by the prompt, never here.
  function toastError(err: unknown) {
    const c = err instanceof FetchError ? err.code : null;
    if (c === "ROOM_NOT_FOUND") toast.error(t("lobby.errors.roomNotFound"));
    else if (c === "ROOM_FULL") toast.error(t("lobby.errors.roomFull"));
    else if (c === "INSUFFICIENT_COINS")
      // Join-by-code has no room object in scope, so it can't compose the
      // {{buyIn}}/{{balance}} message — use the param-less generic variant.
      toast.error(t("room.errors.insufficientCoinsGeneric"));
    else if (c === "ALREADY_IN_ROOM") toast.error(t("lobby.errors.alreadyInRoom"));
    else toast.error(t("lobby.errors.joinFailed"));
  }

  async function joinResolvedRoom(room: Room, password?: string) {
    try {
      await joinRoomMutation.mutateAsync({ id: room.id, password });
      setPendingPrivateRoom(null);
      navigate(`/rooms/${room.id}`);
    } catch (err) {
      const c = err instanceof FetchError ? err.code : null;
      if (c === "WRONG_ROOM_PASSWORD") {
        // Keep the prompt open with the inline error for a retry (AC4).
        setPasswordErrorKey("room.errors.wrongPassword");
        return;
      }
      setPendingPrivateRoom(null);
      toastError(err);
    }
  }

  async function submit() {
    const trimmed = code.trim().toUpperCase();
    if (trimmed.length !== CODE_LENGTH || busy) return;

    setResolving(true);
    try {
      const { room } = await getRoomByCode(trimmed);
      setResolving(false);
      if (room.isPrivate) {
        setPasswordErrorKey(null);
        setPendingPrivateRoom(room);
        return;
      }
      await joinResolvedRoom(room);
    } catch (err) {
      setResolving(false);
      toastError(err);
    }
  }

  return (
    <div
      className="bg-surface flex items-center gap-2.5 rounded-lg border border-border pr-2.5 pl-4.5"
      data-testid="join-by-code"
    >
      <KeyRound className="text-ink-dim size-4 shrink-0" strokeWidth={1.8} />
      <input
        value={code}
        onChange={(e) => setCode(e.target.value.toUpperCase().slice(0, CODE_LENGTH))}
        onKeyDown={(e) => e.key === "Enter" && submit()}
        placeholder={t("lobby.actions.joinByCode.placeholder")}
        maxLength={CODE_LENGTH}
        data-testid="join-by-code-input"
        className="text-ink min-w-0 flex-1 bg-transparent py-2.5 text-base font-semibold tracking-[2px] tabular-nums outline-none placeholder:font-normal placeholder:tracking-normal placeholder:text-ink-off"
      />
      <button
        onClick={submit}
        disabled={!isValid || busy}
        data-testid="join-by-code-button"
        className={cn(
          "rounded-[10px] border border-transparent px-3.5 py-2 text-xs font-semibold transition-colors",
          isValid
            ? "bg-ink text-background cursor-pointer"
            : "bg-surface-sunken text-ink-mute cursor-default opacity-80",
        )}
      >
        {t("lobby.actions.joinByCode.cta")}
      </button>

      <PasswordPromptDialog
        open={pendingPrivateRoom !== null}
        roomName={pendingPrivateRoom?.name ?? ""}
        pending={joinRoomMutation.isPending}
        errorKey={passwordErrorKey}
        onSubmit={(password) => {
          if (pendingPrivateRoom) {
            setPasswordErrorKey(null);
            void joinResolvedRoom(pendingPrivateRoom, password);
          }
        }}
        onClose={() => {
          setPendingPrivateRoom(null);
          setPasswordErrorKey(null);
        }}
      />
    </div>
  );
}
