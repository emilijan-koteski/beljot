import type { TFunction } from "i18next";

import type { PlayerState } from "@/shared/types/matchTypes";

/**
 * Single source of the bot display rule (Story 10.3): bot identity is
 * seat-derived ("Bot 1"–"Bot 4") and localized at render time — the server
 * sends only `isBot` + the seat, never a name. Apply wherever a `username`
 * renders from a players array so an empty bot username never leaks through
 * as a blank. A swapped/moved bot changes name with its seat — accepted
 * consequence, not a bug.
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
