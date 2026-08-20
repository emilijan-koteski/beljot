import type { MatchMode, Variant } from "@/shared/types/matchTypes";

/**
 * THE single place that turns a server variant / match-mode string into
 * display text. Every surface reads through here — lobby cards, the Quick Play
 * strip, the room page, the create-room modal, the in-match HUD and match
 * history — so adding a variant is a one-line change instead of the four
 * independent mappings and three parallel i18n key families this replaced.
 *
 * Lives in `shared/lib` rather than under a feature: match history and the
 * match HUD both need it, and importing a `features/lobby` helper from those
 * was a feature-boundary crossing that only happened because this was the
 * least-bad home available.
 *
 * i18n namespaces:
 *   - Variants use ONE family, `lobby.card.variant*`. The strings were
 *     byte-identical across the three families that used to exist, so they
 *     collapsed.
 *   - Modes keep TWO forms because they genuinely differ: the compact
 *     "1001 pts" for chips and badges, and the long "1001 points" the
 *     create-room form spells out. Both are reached from here.
 *
 * Unknown server values fall back to a readable approximation rather than a
 * raw key, so a variant or mode added on the server renders as something
 * sensible before the frontend catches up.
 */

type Translate = (key: string, options?: Record<string, unknown>) => string;

/** Compact variant name — "Bitola", "Croatian". */
export function variantLabel(t: Translate, v: Variant | string): string {
  if (v === "bitola") return t("lobby.card.variantBitola");
  if (v === "croatia") return t("lobby.card.variantCroatia");
  return v ? v.charAt(0).toUpperCase() + v.slice(1) : "—";
}

/** Compact mode name for chips and badges — "1001 pts". */
export function modeLabel(t: Translate, m: MatchMode | string): string {
  if (m === "1001") return t("lobby.card.matchMode1001");
  if (m === "501") return t("lobby.card.matchMode501");
  // Localized, not a hardcoded English `${m} pts` (D138): the fallback used to
  // be the one string on this path that ignored the active locale.
  return /^\d+$/.test(m) ? t("lobby.card.matchModeGeneric", { points: m }) : m || "—";
}

/** Spelled-out mode name for the create-room form — "1001 points". */
export function modeOptionLabel(t: Translate, m: MatchMode | string): string {
  if (m === "1001") return t("lobby.createRoomModal.matchMode1001");
  if (m === "501") return t("lobby.createRoomModal.matchMode501");
  return modeLabel(t, m);
}
