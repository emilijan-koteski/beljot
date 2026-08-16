import i18n from "i18next";

import { formatCoins } from "@/shared/lib/formatCoins";
import { honorScoreOrPrior } from "@/shared/lib/honor";
import { useAuthStore } from "@/shared/stores/authStore";

/**
 * The numbers a join-failure message can interpolate. Both are optional so a
 * call site that does not hold them (join-by-code, an invite popup) degrades to
 * the param-less generic copy for that branch alone rather than for all of them.
 * A full `Room` satisfies this structurally.
 */
export interface JoinFailureContext {
  coinBuyIn?: number;
  minHonor?: number;
}

/**
 * THE join-failure → message mapping. Every client join entry point routes
 * through this one function: the lobby card (LobbyPage.joinRoomFlow), the
 * join-by-code tile, both RoomPage deep-link paths, and the Story 11.5 friend
 * invite accept.
 *
 * It is shared rather than duplicated because that duplication has already cost
 * real bugs twice: Stories 9.6 and 9.8 each added an error code to one join path
 * and forgot another, and both escaped review to be caught only in manual E2E
 * (Story 11.5 D4). A new code added here now reaches every path at once.
 *
 * WRONG_ROOM_PASSWORD is deliberately absent: it is never a toast. Every caller
 * intercepts it first and renders it inline in the password prompt so the player
 * can retry (Story 9.6 AC4).
 *
 * The numbers are composed LOCALLY (Story 9.2 Decision B) — the server's error
 * payload carries only a code, and the caller holds the room plus the viewer's
 * own auth envelope.
 */
export function joinFailureMessage(code: string | null, room?: JoinFailureContext | null): string {
  if (code === "ROOM_NOT_FOUND") return i18n.t("lobby.errors.roomNotFound");
  if (code === "ROOM_FULL") return i18n.t("lobby.errors.roomFull");
  if (code === "ALREADY_IN_ROOM") return i18n.t("lobby.errors.alreadyInRoom");

  if (code === "INSUFFICIENT_COINS") {
    // typeof, never truthiness — a room with a buy-in of 0 is legitimate (and
    // could never produce this code, but the guard must not depend on that).
    if (typeof room?.coinBuyIn !== "number") return i18n.t("room.errors.insufficientCoinsGeneric");
    return i18n.t("room.errors.insufficientCoins", {
      buyIn: formatCoins(room.coinBuyIn),
      balance: formatCoins(useAuthStore.getState().user?.walletBalance ?? 0),
    });
  }

  if (code === "HONOR_TOO_LOW") {
    if (typeof room?.minHonor !== "number") return i18n.t("room.errors.honorTooLowGeneric");
    // honorScoreOrPrior, never `|| 80` — a real score of 0 must survive.
    return i18n.t("room.errors.honorTooLow", {
      minHonor: room.minHonor,
      honor: honorScoreOrPrior(useAuthStore.getState().user?.honorScore),
    });
  }

  if (code === "NEW_PLAYER_NOT_ALLOWED") return i18n.t("room.errors.newPlayerNotAllowed");

  return i18n.t("lobby.errors.joinFailed");
}
