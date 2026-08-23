import i18n from "i18next";

/** Outcomes a room-invite rejection can mean to the caller. */
export type InviteFailureKind =
  /** The invite did not land and cannot be retried into a better state right
   *  now — show `message` against the friend it concerns. */
  | "rejected"
  /** The friend already has a live popup for this room. From the inviter's side
   *  that is what "sent" looks like, so a panel may present it as success. */
  | "alreadyPending"
  /** The ROOM is gone (closed, started, or we were removed). Nothing on the
   *  inviting surface can ever succeed again — close it rather than retry. */
  | "roomGone";

export interface InviteFailure {
  kind: InviteFailureKind;
  message: string;
}

/**
 * THE room-invite failure → message mapping, shared by every surface that can
 * send an invite: the waiting room's InviteFriendsDialog and the match-stats
 * per-player Reinvite control.
 *
 * Shared for the same reason as `joinFailure.ts` (Story 11.5 D4): the invite
 * codes were already duplicated once, and a divergent second copy means a new
 * server code reaches one surface and silently falls through to "please try
 * again" on the other.
 *
 * The KIND is separate from the message because the two callers legitimately
 * differ in what they DO: the panel renders `rejected` inline on the row and
 * treats `alreadyPending` as a sent invite, while the compact reinvite button
 * has no inline slot and toasts every kind.
 */
export function inviteFailure(code: string | null): InviteFailure {
  switch (code) {
    case "FRIEND_NOT_AVAILABLE":
      return { kind: "rejected", message: i18n.t("roomInvite.errors.notAvailable") };
    case "ROOM_FULL":
      return { kind: "rejected", message: i18n.t("roomInvite.errors.roomFull") };
    case "NOT_FRIENDS":
      return { kind: "rejected", message: i18n.t("roomInvite.errors.notFriends") };
    case "INVITE_ALREADY_PENDING":
      return { kind: "alreadyPending", message: i18n.t("roomInvite.errors.alreadyPending") };
    case "ROOM_NOT_FOUND":
    case "NOT_IN_ROOM":
      return { kind: "roomGone", message: i18n.t("roomInvite.errors.roomGone") };
    default:
      return { kind: "rejected", message: i18n.t("roomInvite.errors.sendFailed") };
  }
}
