export const Z = {
  TABLE: 0,
  TRICK: 10,
  SEATS: 20,
  HUD: 25,
  CARD_FLIGHT: 30,
  SEAT_BANNER: 40,
  TABLE_ANIM: 50,

  REVEAL: 60,
  PROMPT_DIM: 70,
  BIDDER_HAND: 72,
  PROMPT: 74,
  UTIL: 80,
  UTIL_POPOVER: 82,
  SURRENDER: 90,
  BLOCKER: 100,
  CAPOT: 110,
  CHROME_TOP: 120,
  TOAST: 130,
} as const;

/** A stacking tier from `Z`; higher is closer to the viewer. Apply it as an inline `zIndex`. */
export type ZLayer = (typeof Z)[keyof typeof Z];
