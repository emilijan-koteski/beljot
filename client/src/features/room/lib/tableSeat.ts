import type { RoomPlayer } from "@/shared/types/apiTypes";

export type Cardinal = "south" | "east" | "north" | "west";

const CARDINALS: readonly Cardinal[] = ["south", "east", "north", "west"];

/** Where a seat sits on screen for a viewer: their own seat at the bottom, the next seat on their right. */
export function cardinalFor(seat: number, viewerSeat: number | null): Cardinal {
  return CARDINALS[(seat - (viewerSeat ?? 0) + 4) % 4] ?? "south";
}

/** Everything the table needs to draw one seat, resolved by the room page. */
export interface TableSeat {
  seat: 0 | 1 | 2 | 3;
  cardinal: Cardinal;
  team: "A" | "B" | null;
  player?: RoomPlayer;
  name: string;
  isYou: boolean;
  isHost: boolean;
  returning: boolean;
  belowHonorBar: boolean;
  swapSource: boolean;
  pending: boolean;
  clickable: boolean;
  canAddBot: boolean;
  hostMenu: boolean;
}
