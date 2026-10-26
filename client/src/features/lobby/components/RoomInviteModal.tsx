import { Coins, Lock, MailOpen } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { toast } from "sonner";

import { PasswordPromptDialog } from "@/features/lobby/components/PasswordPromptDialog";
import { FetchError } from "@/shared/api/axiosClient";
import { playAlertSfx } from "@/shared/audio/audioEngine";
import { Button } from "@/shared/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/shared/components/ui/dialog";
import {
  useDeclineRoomInviteMutation,
  useJoinRoomMutation,
} from "@/shared/hooks/mutations/useRooms";
import { COIN_GOLD } from "@/shared/lib/coinGold";
import { formatCoins } from "@/shared/lib/formatCoins";
import { joinFailureMessage } from "@/shared/lib/joinFailure";
import { useRoomStore } from "@/shared/stores/roomStore";

/**
 * App-wide friend room-invite popup; it whistles once when shown unless the player responds or it closes first.
 * Accepting joins directly, or through the password prompt for a non-host invite into a private room.
 */
export function RoomInviteModal() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const invite = useRoomStore((s) => s.roomInvite);
  const ejection = useRoomStore((s) => s.roomEjection);
  const setRoomInvite = useRoomStore((s) => s.setRoomInvite);
  const joinRoomMutation = useJoinRoomMutation();
  const declineMutation = useDeclineRoomInviteMutation();

  const [showPasswordPrompt, setShowPasswordPrompt] = useState(false);
  const [passwordErrorKey, setPasswordErrorKey] = useState<string | null>(null);

  const inviteId = invite?.inviteId ?? null;
  const expiresAt = invite?.expiresAt ?? null;
  const isJoining = joinRoomMutation.isPending;

  // Closes the popup without telling the server, for an invite that is already dead.
  function dismiss() {
    setShowPasswordPrompt(false);
    setPasswordErrorKey(null);
    setRoomInvite(null);
  }

  // Voids the invite's password grant server-side (fire and forget) and closes the popup.
  function decline() {
    if (invite !== null) {
      declineMutation.mutate(invite.roomId);
    }
    dismiss();
  }

  useEffect(() => {
    setShowPasswordPrompt(false);
    setPasswordErrorKey(null);
  }, [inviteId]);

  useEffect(() => {
    if (expiresAt === null) return;
    const playerIsMidAction = isJoining || showPasswordPrompt;
    if (playerIsMidAction) return;
    const remainingMs = new Date(expiresAt).getTime() - Date.now();
    if (Number.isNaN(remainingMs)) return;
    if (remainingMs <= 0) {
      dismiss();
      return;
    }
    const timer = window.setTimeout(dismiss, remainingMs);
    return () => window.clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [inviteId, expiresAt, isJoining, showPasswordPrompt]);

  const ejectionNoticeTakesPriority = ejection !== null;
  const popupShown = invite !== null && !ejectionNoticeTakesPriority;
  const shownInviteKey = popupShown ? `${invite.inviteId}:${invite.expiresAt}` : null;
  const whistleRef = useRef<AbortController | null>(null);
  const silencedInviteKeyRef = useRef<string | null>(null);
  const playerResponded = isJoining || showPasswordPrompt;

  function silenceWhistle() {
    if (shownInviteKey !== null) silencedInviteKeyRef.current = shownInviteKey;
    whistleRef.current?.abort();
  }

  useEffect(() => {
    if (!playerResponded || shownInviteKey === null) return;
    silencedInviteKeyRef.current = shownInviteKey;
    whistleRef.current?.abort();
  }, [playerResponded, shownInviteKey]);

  useEffect(() => {
    if (shownInviteKey === null || shownInviteKey === silencedInviteKeyRef.current) return;
    const whistle = new AbortController();
    whistleRef.current = whistle;
    void playAlertSfx("invite", { dedupeKey: shownInviteKey, signal: whistle.signal });
    return () => whistle.abort();
  }, [shownInviteKey]);

  if (!popupShown) {
    return null;
  }

  async function join(roomId: number, password?: string) {
    if (invite === null) return;
    const { coinBuyIn, minHonor } = invite;
    try {
      await joinRoomMutation.mutateAsync({ id: roomId, password });
      dismiss();
      navigate(`/rooms/${roomId}`);
    } catch (err) {
      const code = err instanceof FetchError ? err.code : null;
      if (code === "WRONG_ROOM_PASSWORD" && showPasswordPrompt) {
        setPasswordErrorKey("room.errors.wrongPassword");
        return;
      }
      if (code === "WRONG_ROOM_PASSWORD") {
        dismiss();
        toast.error(t("roomInvite.errors.expired"));
        return;
      }
      if (code === "ALREADY_IN_ROOM") {
        dismiss();
        navigate(`/rooms/${roomId}`);
        return;
      }
      dismiss();
      toast.error(joinFailureMessage(code, { coinBuyIn, minHonor }));
    }
  }

  function handleAccept() {
    if (invite === null) return;
    if (invite.isPrivate === true && invite.isHostInvite !== true) {
      setPasswordErrorKey(null);
      setShowPasswordPrompt(true);
      return;
    }
    void join(invite.roomId);
  }

  return (
    <>
      <Dialog
        open={!showPasswordPrompt}
        onOpenChange={(next) => {
          if (!next) decline();
        }}
      >
        <DialogContent
          showCloseButton={false}
          data-testid="room-invite-modal"
          onPointerDownCapture={silenceWhistle}
        >
          <DialogHeader>
            <div className="flex items-center gap-3">
              <div className="bg-brass-soft border-brass/40 text-brass-deep flex size-11 shrink-0 items-center justify-center rounded-xl border">
                <MailOpen className="size-5" />
              </div>
              <DialogTitle data-testid="room-invite-title">
                {t("roomInvite.popup.title")}
              </DialogTitle>
            </div>
            <DialogDescription data-testid="room-invite-body">
              {t("roomInvite.popup.body", {
                inviter:
                  invite.inviterUsername.length > 0
                    ? invite.inviterUsername
                    : t("roomInvite.someone"),
                roomName: invite.roomName,
              })}
            </DialogDescription>
          </DialogHeader>

          <div className="flex flex-wrap items-center gap-2">
            {invite.coinBuyIn > 0 && (
              <span
                className="border-border bg-surface-sunken text-ink-dim inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium"
                data-testid="room-invite-buyin"
              >
                <Coins className="size-3" style={{ color: COIN_GOLD }} aria-hidden="true" />
                {t("roomInvite.popup.buyIn", { buyIn: formatCoins(invite.coinBuyIn) })}
              </span>
            )}
            {invite.isPrivate === true && (
              <span
                className="border-border bg-surface-sunken text-ink-dim inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium"
                data-testid="room-invite-private"
              >
                <Lock className="size-3" />
                {invite.isHostInvite === true
                  ? t("roomInvite.popup.privateHostInvite")
                  : t("roomInvite.popup.privatePasswordNeeded")}
              </span>
            )}
          </div>

          <DialogFooter>
            <Button
              type="button"
              onClick={handleAccept}
              data-testid="room-invite-accept"
              disabled={isJoining}
            >
              {isJoining ? t("roomInvite.popup.accepting") : t("roomInvite.popup.accept")}
            </Button>
            <Button
              type="button"
              variant="ghost"
              onClick={decline}
              data-testid="room-invite-decline"
              disabled={isJoining}
            >
              {t("roomInvite.popup.decline")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <PasswordPromptDialog
        open={showPasswordPrompt}
        roomName={invite.roomName}
        pending={isJoining}
        errorKey={passwordErrorKey}
        onSubmit={(password) => {
          setPasswordErrorKey(null);
          void join(invite.roomId, password);
        }}
        onClose={decline}
      />
    </>
  );
}
