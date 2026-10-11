import i18n from "i18next";

import { formatCoins } from "@/shared/lib/formatCoins";
import { honorScoreOrPrior } from "@/shared/lib/honor";
import { useAuthStore } from "@/shared/stores/authStore";

/**
 * Room numbers a join-failure message can interpolate; a missing one falls back
 * to that branch's generic copy. A full `Room` satisfies this structurally.
 */
export interface JoinFailureContext {
  coinBuyIn?: number;
  minHonor?: number;
}

/**
 * Maps a join error code to a toast message for every join entry point. Callers
 * handle WRONG_ROOM_PASSWORD inline themselves, so it is not mapped here.
 */
export function joinFailureMessage(code: string | null, room?: JoinFailureContext | null): string {
  if (code === "ROOM_NOT_FOUND") return i18n.t("lobby.errors.roomNotFound");
  if (code === "ROOM_FULL") return i18n.t("lobby.errors.roomFull");
  if (code === "ALREADY_IN_ROOM") return i18n.t("lobby.errors.alreadyInRoom");

  if (code === "INSUFFICIENT_COINS") {
    if (typeof room?.coinBuyIn !== "number") return i18n.t("room.errors.insufficientCoinsGeneric");
    return i18n.t("room.errors.insufficientCoins", {
      buyIn: formatCoins(room.coinBuyIn),
      balance: formatCoins(useAuthStore.getState().user?.walletBalance ?? 0),
    });
  }

  if (code === "HONOR_TOO_LOW") {
    if (typeof room?.minHonor !== "number") return i18n.t("room.errors.honorTooLowGeneric");
    return i18n.t("room.errors.honorTooLow", {
      minHonor: room.minHonor,
      honor: honorScoreOrPrior(useAuthStore.getState().user?.honorScore),
    });
  }

  if (code === "NEW_PLAYER_NOT_ALLOWED") return i18n.t("room.errors.newPlayerNotAllowed");

  return i18n.t("lobby.errors.joinFailed");
}
