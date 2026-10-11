import type { TFunction } from "i18next";

import type { PlayerState } from "@/shared/types/matchTypes";

/**
 * Localized, seat-derived bot name ("Bot 1"–"Bot 4"); the server never sends a
 * bot username, so use this wherever a bot's name renders.
 */
export function botDisplayName(t: TFunction, seat: number | null | undefined): string {
  return t("bots.seatName", { n: (seat ?? 0) + 1 });
}

/**
 * Resolves a match-table player's display name: localized bot name for bot
 * seats, the username otherwise. Returns null for a missing player or an
 * (unexpected) empty human username so callers keep their own fallbacks.
 */
export function playerDisplayName(
  t: TFunction,
  player: Pick<PlayerState, "seat" | "username" | "isBot"> | null | undefined,
): string | null {
  if (!player) return null;
  if (player.isBot === true) return botDisplayName(t, player.seat);
  return player.username || null;
}
