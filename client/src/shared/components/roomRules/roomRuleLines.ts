import type { TFunction } from "i18next";
import {
  ArrowRightLeft,
  Bot,
  CircleSlash2,
  Flag,
  Lock,
  LockOpen,
  type LucideIcon,
  Medal,
  Megaphone,
  MegaphoneOff,
  RotateCcw,
  RotateCw,
  Sprout,
  Target,
  Zap,
} from "lucide-react";

import { roomBotCount } from "@/shared/lib/roomOccupancy";
import type { Room } from "@/shared/types/apiTypes";

export type RoomRuleSource = Pick<
  Room,
  | "clockwise"
  | "declarationsEnabled"
  | "declarationsTransfer"
  | "stopAtTarget"
  | "isPrivate"
  | "allowNewPlayers"
  | "isQuickPlay"
  | "coinBuyIn"
  | "players"
>;

export interface RuleLine {
  key: string;
  label: string;
  value: string;
  icon: LucideIcon;
  changed: boolean;
  unused?: boolean;
}

const FORM = "lobby.createRoomModal";

/** The four match rules in a fixed order, worded as the create-room form words them. */
export function matchRuleLines(t: TFunction, room: RoomRuleSource): RuleLine[] {
  const clockwise = room.clockwise === true;
  const declarationsOff = room.declarationsEnabled === false;
  const voided = room.declarationsTransfer === false && !declarationsOff;
  const stop = room.stopAtTarget === true;
  return [
    {
      key: "direction",
      label: t(`${FORM}.direction`),
      value: t(clockwise ? `${FORM}.directionClockwise` : `${FORM}.directionCounterClockwise`),
      icon: clockwise ? RotateCw : RotateCcw,
      changed: clockwise,
    },
    {
      key: "declarations",
      label: t(`${FORM}.declarations`),
      value: t(declarationsOff ? `${FORM}.declarationsOff` : `${FORM}.declarationsOn`),
      icon: declarationsOff ? MegaphoneOff : Megaphone,
      changed: declarationsOff,
    },
    {
      key: "fall",
      label: t(`${FORM}.declarationsTransfer`),
      value: declarationsOff
        ? t("roomRules.notUsed")
        : t(voided ? `${FORM}.declarationsTransferOff` : `${FORM}.declarationsTransferOn`),
      icon: voided ? CircleSlash2 : ArrowRightLeft,
      changed: voided,
      unused: declarationsOff,
    },
    {
      key: "end",
      label: t(`${FORM}.matchEnd`),
      value: t(stop ? `${FORM}.matchEndStop` : `${FORM}.matchEndFinish`),
      icon: stop ? Target : Flag,
      changed: stop,
    },
  ];
}

/** Who may sit down beyond the honor floor: access, new players, seats held by bots and, for Quick Play, how seats fill. */
export function roomRuleLines(t: TFunction, room: RoomRuleSource): RuleLine[] {
  const veteransOnly = room.allowNewPlayers === false;
  const lines: RuleLine[] = [
    {
      key: "access",
      label: t("roomRules.access"),
      value: t(room.isPrivate ? "roomRules.accessPrivate" : "roomRules.accessPublic"),
      icon: room.isPrivate ? Lock : LockOpen,
      changed: room.isPrivate,
    },
    {
      key: "newPlayers",
      label: t(`${FORM}.allowNewPlayers`),
      value: t(veteransOnly ? `${FORM}.allowNewPlayersNo` : `${FORM}.allowNewPlayersYes`),
      icon: veteransOnly ? Medal : Sprout,
      changed: veteransOnly,
    },
  ];
  const bots = roomBotCount(room);
  if (bots > 0) {
    lines.push({
      key: "bots",
      label: t("roomRules.bots"),
      value: String(bots),
      icon: Bot,
      changed: true,
    });
  }
  if (room.isQuickPlay) {
    lines.push({
      key: "seating",
      label: t("roomRules.seating"),
      value: t("roomRules.seatingQuickPlay"),
      icon: Zap,
      changed: false,
    });
  }
  return lines;
}
