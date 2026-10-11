import type { MatchMode, Variant } from "@/shared/types/matchTypes";

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
  return /^\d+$/.test(m) ? t("lobby.card.matchModeGeneric", { points: m }) : m || "—";
}

/** Spelled-out mode name for the create-room form — "1001 points". */
export function modeOptionLabel(t: Translate, m: MatchMode | string): string {
  if (m === "1001") return t("lobby.createRoomModal.matchMode1001");
  if (m === "501") return t("lobby.createRoomModal.matchMode501");
  return modeLabel(t, m);
}
