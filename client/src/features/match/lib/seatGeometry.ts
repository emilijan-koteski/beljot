export const SEAT_DISC_PX = 64;
export const SEAT_DISC_PX_COMPACT = 44;
export const SEAT_FRAME_PAD_PX = 16;
export const SEAT_BORDER_PX = 3;

/** Outer size of a seat's avatar frame: the disc plus the frame padding. */
export function seatFramePx(compact: boolean): number {
  return (compact ? SEAT_DISC_PX_COMPACT : SEAT_DISC_PX) + SEAT_FRAME_PAD_PX;
}

/** Diameter inside the seat's team-colour border, where the turn timer ring sits flush. */
export function seatInnerPx(compact: boolean): number {
  return seatFramePx(compact) - 2 * SEAT_BORDER_PX;
}
