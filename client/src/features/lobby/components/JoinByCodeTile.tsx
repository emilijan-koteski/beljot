import { KeyRound } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { toast } from "sonner";

import { PasswordPromptDialog } from "@/features/lobby/components/PasswordPromptDialog";
import { FetchError } from "@/shared/api/axiosClient";
import { getRoomByCode } from "@/shared/api/rooms";
import { Button } from "@/shared/components/ui/button";
import { useJoinRoomMutation } from "@/shared/hooks/mutations/useRooms";
import { joinFailureMessage } from "@/shared/lib/joinFailure";
import type { Room } from "@/shared/types/apiTypes";

const CODE_LENGTH = 6;

/**
 * Room-code input with an inline Join button, submitting on Enter or click.
 * Resolves the code first and prompts for a password when the room is private.
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

  // Maps a lookup or join failure to a toast with the generic shared copy.
  // WRONG_ROOM_PASSWORD is handled inline by the prompt, never here.
  function toastError(err: unknown) {
    const c = err instanceof FetchError ? err.code : null;
    toast.error(joinFailureMessage(c));
  }

  async function joinResolvedRoom(room: Room, password?: string) {
    try {
      await joinRoomMutation.mutateAsync({ id: room.id, password });
      setPendingPrivateRoom(null);
      navigate(`/rooms/${room.id}`);
    } catch (err) {
      const c = err instanceof FetchError ? err.code : null;
      if (c === "WRONG_ROOM_PASSWORD") {
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
      className="bg-surface flex items-center gap-2.5 rounded-lg border border-border pr-2.5 pl-4.5 transition-[border-color,box-shadow] has-[input:focus-visible]:border-ring has-[input:focus-visible]:ring-3 has-[input:focus-visible]:ring-ring/70"
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
        className="text-ink min-w-0 flex-1 bg-transparent py-2.5 text-base font-semibold tracking-widest tabular-nums outline-none placeholder:font-normal placeholder:tracking-normal placeholder:text-ink-mute"
      />
      <Button
        size="sm"
        onClick={submit}
        disabled={!isValid || busy}
        data-testid="join-by-code-button"
      >
        {t("lobby.actions.joinByCode.cta")}
      </Button>

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
