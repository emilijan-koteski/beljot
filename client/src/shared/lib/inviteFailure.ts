import i18n from "i18next";

/**
 * Outcomes of a rejected room invite: `rejected` shows the message, `alreadyPending`
 * may read as sent, and `roomGone` means the inviting surface should close.
 */
export type InviteFailureKind = "rejected" | "alreadyPending" | "roomGone";

export interface InviteFailure {
  kind: InviteFailureKind;
  message: string;
}

/** Maps a room-invite error code to its failure kind and message, shared by every invite surface. */
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
