type Compass = 0 | 1 | 2 | 3;

export function compassOffset(seat: number, myPlayerSeat: number): Compass {
  return ((((seat - myPlayerSeat) % 4) + 4) % 4) as Compass;
}

/** A trick card's offset from the trick-area center toward its player, plus a rotation jitter. */
interface SlotPosition {
  offsetX: number;
  offsetY: number;
  rotation: number;
}

export const SLOT_POSITIONS: Record<Compass, SlotPosition> = {
  0: { offsetX: 0, offsetY: 60, rotation: -3 },
  1: { offsetX: 74, offsetY: 0, rotation: 8 },
  2: { offsetX: 0, offsetY: -60, rotation: 4 },
  3: { offsetX: -74, offsetY: 0, rotation: -8 },
};
