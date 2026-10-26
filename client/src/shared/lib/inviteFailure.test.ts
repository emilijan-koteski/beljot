import "@/shared/i18n/i18n";

import { describe, expect, it } from "vitest";

import { i18n } from "@/shared/i18n/i18n";

import { inviteFailure } from "./inviteFailure";

describe("inviteFailure", () => {
  it("maps NOT_FRIENDS to the friends-only message", () => {
    expect(inviteFailure("NOT_FRIENDS")).toEqual({
      kind: "rejected",
      message: "This player only accepts invites from friends",
    });
  });

  it.each([
    ["FRIEND_NOT_AVAILABLE", "rejected", "roomInvite.errors.notAvailable"],
    ["ROOM_FULL", "rejected", "roomInvite.errors.roomFull"],
    ["INVITE_ALREADY_PENDING", "alreadyPending", "roomInvite.errors.alreadyPending"],
    ["ROOM_NOT_FOUND", "roomGone", "roomInvite.errors.roomGone"],
    ["NOT_IN_ROOM", "roomGone", "roomInvite.errors.roomGone"],
    ["SOMETHING_ELSE", "rejected", "roomInvite.errors.sendFailed"],
  ])("maps %s to %s", (code, kind, messageKey) => {
    expect(inviteFailure(code)).toEqual({ kind, message: i18n.t(messageKey) });
  });

  it("treats a missing code as a generic send failure", () => {
    expect(inviteFailure(null)).toEqual({
      kind: "rejected",
      message: i18n.t("roomInvite.errors.sendFailed"),
    });
  });
});
