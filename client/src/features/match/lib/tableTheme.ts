/**
 * Viewer-relative team identity: the viewer's own team always reads as Gold ("Us"), the other as Silver ("Them").
 */
export type SeatTeam = "gold" | "silver";

/** Viewer-relative team for a seat; partners share seat parity, so the viewer and their partner are Gold. */
export function seatTeam(seat: number, viewerSeat: number): SeatTeam {
  return seat % 2 === viewerSeat % 2 ? "gold" : "silver";
}

export const TEAM_GOLD = ["#e8c25a", "#a07d1a"] as const;

export const TEAM_SILVER = ["#d8dde4", "#8b919a"] as const;

export const TURN_LIME = ["#00e5a0", "#00a878"] as const;

export const TURN_URGENT = ["#ef4444", "#b91c1c"] as const;

export type TeamGradient = readonly [string, string];

/** Resolve the bright/deep gradient stops for a viewer-relative team. */
export function teamColors(team: SeatTeam): TeamGradient {
  return team === "gold" ? TEAM_GOLD : TEAM_SILVER;
}

/** i18n key for the viewer-relative team label ("Us" / "Them"). */
export function teamLabelKey(team: SeatTeam): "team.us" | "team.them" {
  return team === "gold" ? "team.us" : "team.them";
}
