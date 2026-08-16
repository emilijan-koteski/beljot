import { Coins, Lock, MailOpen } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { toast } from "sonner";

import { PasswordPromptDialog } from "@/features/lobby/components/PasswordPromptDialog";
import { FetchError } from "@/shared/api/axiosClient";
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
import { formatCoins } from "@/shared/lib/formatCoins";
import { joinFailureMessage } from "@/shared/lib/joinFailure";
import { useRoomStore } from "@/shared/stores/roomStore";

/**
 * Story 11.5: the incoming friend room-invite popup. Store-driven and
 * always-mounted, but mounted at the APP level (AppLayout) rather than inside
 * LobbyPage — the server marks a friend invitable whenever they are online, not
 * in a match and not in a room, which includes anyone reading their profile or
 * the rules page. Mounting it on the lobby alone made every one of those invites
 * a silent black hole: pushed, stored, never rendered, expired.
 *
 * Accepting takes one of three paths, selected entirely by the payload:
 *
 *   host invite            -> join with NO password; the server's one-time grant
 *                             carries the invitee past the password gate (AC3)
 *   non-host + private     -> the Story 9.6 password prompt, verbatim (AC4)
 *   non-host + public      -> a plain join (AC5)
 *
 * Every failure routes through the ONE shared joinFailureMessage mapping — the
 * invite accept is a fourth join entry point, and forking the mapping is exactly
 * the bug 9.6 and 9.8 each shipped once (D4).
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

  // Dismiss WITHOUT telling the server: used when the invite is already dead
  // (expired, consumed by a successful join) so there is nothing to void.
  function dismiss() {
    setShowPasswordPrompt(false);
    setPasswordErrorKey(null);
    setRoomInvite(null);
  }

  // The player actively said no. Void the grant server-side so a declined invite
  // cannot still walk them past a password for the rest of its TTL. Fire and
  // forget — the popup closes either way, and a failed void just means the
  // invite lapses on its own clock instead of immediately.
  function decline() {
    if (invite !== null) {
      declineMutation.mutate(invite.roomId);
    }
    dismiss();
  }

  // A NEW invite replacing an unanswered one must not inherit the old one's
  // dialog state. The store holds a single invite slot, so without this the
  // password typed for room A would be submitted to room B — the prompt stays
  // open across the swap and PasswordPromptDialog only clears its input on an
  // open false->true edge.
  useEffect(() => {
    setShowPasswordPrompt(false);
    setPasswordErrorKey(null);
  }, [inviteId]);

  // Auto-dismiss at the server's absolute expiry (AC2). Without this the popup
  // would outlive the grant that backs it. The server timestamp is authoritative;
  // we only mirror it in the UI.
  //
  // Deliberately inert while a join is in flight or the password prompt is open:
  // firing then would tear both dialogs off screen mid-action with no message —
  // yanking the field out from under someone typing their password, or unmounting
  // during an accept that is about to succeed. The effect re-runs when either
  // clears, so an invite that expired meanwhile is dropped a moment later instead.
  useEffect(() => {
    if (expiresAt === null) return;
    if (isJoining || showPasswordPrompt) return;
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

  // One notice at a time: the ejection modal is also always-mounted and also
  // opens on arrival in the lobby. An ejected player is room-less and match-less,
  // i.e. invitable, so both can be live at once — two open dialogs mean the
  // second traps focus and the first becomes unreachable behind it. The invite
  // waits; it keeps its own expiry either way.
  if (invite === null || ejection !== null) {
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
        // Keep the prompt open with the inline error so the player can retry.
        setPasswordErrorKey("room.errors.wrongPassword");
        return;
      }
      if (code === "WRONG_ROOM_PASSWORD") {
        // No prompt is open, so there is nowhere to render an inline error — this
        // is the host-invite path whose grant has gone (voided by a WS blip, a
        // room that briefly filled, or expiry), or a room that turned private
        // after the invite was sent. Writing the error into the closed prompt
        // would show the player absolutely nothing and leave the modal up
        // forever, which is the dead end AC7 exists to prevent. Say the invite is
        // no longer valid and let them go.
        dismiss();
        toast.error(t("roomInvite.errors.expired"));
        return;
      }
      if (code === "ALREADY_IN_ROOM") {
        // They are already seated somewhere — every other join path routes the
        // player to the room rather than stranding them on a toast.
        dismiss();
        navigate(`/rooms/${roomId}`);
        return;
      }
      // Full / closed / gated: say why, and leave the player where they are
      // rather than on a stuck modal (AC7). Both numbers are in the payload, so
      // the coin AND honor messages are the specific variants here, exactly as on
      // every other join path.
      dismiss();
      toast.error(joinFailureMessage(code, { coinBuyIn, minHonor }));
    }
  }

  function handleAccept() {
    if (invite === null) return;
    // A non-host invite into a private room still has to clear the password
    // gate — only the OWNER's invite carries a bypass (AC4).
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
        <DialogContent showCloseButton={false} data-testid="room-invite-modal">
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
            {/* Go int — a buy-in of 0 is a real, free room, so compare
                explicitly rather than relying on truthiness. */}
            {invite.coinBuyIn > 0 && (
              <span
                className="border-border bg-surface-sunken text-ink-dim inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium"
                data-testid="room-invite-buyin"
              >
                <Coins className="size-3" />
                {t("roomInvite.popup.buyIn", { buyIn: formatCoins(invite.coinBuyIn) })}
              </span>
            )}
            {invite.isPrivate === true && (
              <span
                className="border-border bg-surface-sunken text-ink-dim inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium"
                data-testid="room-invite-private"
              >
                <Lock className="size-3" />
                {/* The host's invite waves the invitee past the password, so say
                    so — otherwise a "private" badge reads as "you will be asked
                    for a password" and the accept feels like a gamble. */}
                {invite.isHostInvite === true
                  ? t("roomInvite.popup.privateHostInvite")
                  : t("roomInvite.popup.privatePasswordNeeded")}
              </span>
            )}
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              onClick={decline}
              data-testid="room-invite-decline"
              disabled={isJoining}
            >
              {t("roomInvite.popup.decline")}
            </Button>
            <Button
              type="button"
              onClick={handleAccept}
              data-testid="room-invite-accept"
              disabled={isJoining}
            >
              {isJoining ? t("roomInvite.popup.accepting") : t("roomInvite.popup.accept")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Story 9.6's prompt, reused verbatim for the non-host private path. */}
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
