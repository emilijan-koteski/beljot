import { useCallback } from "react";

import { ChatDock } from "@/features/chat/ChatDock";
import { useMatchStore } from "@/shared/stores/matchStore";

import { seatTeam, teamColors } from "../lib/tableTheme";

interface MatchChatDockProps {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * In-game match chat dock built on ChatDock with the dark felt skin.
 * It colors senders viewer-relative (gold = Us, silver = Them).
 */
export function MatchChatDock({ isOpen, onOpenChange }: MatchChatDockProps) {
  const roomId = useMatchStore((s) => s.roomId);
  const players = useMatchStore((s) => s.matchState?.players);
  const myPlayerSeat = useMatchStore((s) => s.myPlayerSeat);

  const resolveNameColor = useCallback(
    (userId: number): string => {
      if (myPlayerSeat === null || !players) return "var(--brass)";
      const player = players.find((p) => p.userId === userId);
      if (!player) return "var(--brass)";
      return teamColors(seatTeam(player.seat, myPlayerSeat))[0];
    },
    [players, myPlayerSeat],
  );

  if (roomId === null || roomId <= 0) return null;

  return (
    <ChatDock
      variant="match"
      roomId={roomId}
      open={isOpen}
      onOpenChange={onOpenChange}
      resolveNameColor={resolveNameColor}
      className="z-120"
    />
  );
}
