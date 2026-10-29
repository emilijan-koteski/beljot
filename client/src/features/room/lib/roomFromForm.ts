import type { Room } from "@/shared/types/apiTypes";
import type { MatchMode } from "@/shared/types/matchTypes";

export interface RoomFormValues {
  name: string;
  variant: "bitola" | "croatia";
  matchMode: MatchMode;
  timerStyle: "relaxed" | "per-move";
  timerDuration: number;
  coinBuyIn: number;
  isPrivate: boolean;
  minHonor: number;
  allowNewPlayers: boolean;
  declarationsEnabled: boolean;
  stopAtTarget: boolean;
  declarationsTransfer: boolean;
  clockwise: boolean;
  hostUsername: string;
  hostAvatarUrl: string | null;
}

/** The room the lobby will list for these form values, with the host alone in seat 0. */
export function roomFromForm(p: RoomFormValues): Room {
  return {
    id: 0,
    name: p.name,
    code: "",
    ownerId: 0,
    ownerUsername: p.hostUsername,
    ownerAvatarUrl: p.hostAvatarUrl,
    players: [
      {
        id: 0,
        roomId: 0,
        userId: 0,
        username: p.hostUsername,
        avatarUrl: p.hostAvatarUrl,
        seat: 0,
        team: "A",
        isBot: false,
        createdAt: "",
      },
    ],
    variant: p.variant,
    matchMode: p.matchMode,
    timerStyle: p.timerStyle,
    timerDurationSeconds: p.timerStyle === "relaxed" ? null : p.timerDuration,
    coinBuyIn: p.coinBuyIn,
    isPrivate: p.isPrivate,
    minHonor: p.minHonor,
    allowNewPlayers: p.allowNewPlayers,
    declarationsEnabled: p.declarationsEnabled,
    stopAtTarget: p.stopAtTarget,
    declarationsTransfer: p.declarationsTransfer,
    clockwise: p.clockwise,
    status: "waiting",
    playerCount: 1,
    isQuickPlay: false,
    createdAt: "",
    updatedAt: "",
  };
}
