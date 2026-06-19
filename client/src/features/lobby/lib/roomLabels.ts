// Variant + match-mode labels resolved through i18n so the locale picks them
// (e.g. "Битола · 1001 поен" in MK). Unknown server values fall back to a
// title-cased / "N pts" approximation so a new variant or mode added on the
// server doesn't require a frontend change before it can be displayed at all.
//
// Shared by the lobby room cards and the Quick Play matchmaking strip so the
// two surfaces never drift on how a variant/mode reads.

export function variantLabel(t: (key: string) => string, v: string): string {
  if (v === "bitola") return t("lobby.card.variantBitola");
  return v ? v.charAt(0).toUpperCase() + v.slice(1) : "—";
}

export function modeLabel(t: (key: string) => string, m: string): string {
  if (m === "1001") return t("lobby.card.matchMode1001");
  if (m === "501") return t("lobby.card.matchMode501");
  return /^\d+$/.test(m) ? `${m} pts` : m || "—";
}
