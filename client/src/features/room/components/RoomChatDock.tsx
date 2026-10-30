import { useCallback } from "react";

import { ChatDock } from "@/features/chat/ChatDock";
import { useAuthStore } from "@/shared/stores/authStore";
import { useRoomStore } from "@/shared/stores/roomStore";

/**
 * Room-scoped chat dock that colors sender names gold or silver relative to the viewer's seat.
 * On a phone its launcher sits above the pinned Leave and Start bar.
 */
export function RoomChatDock({
  roomId,
  aboveBottomBar = false,
}: {
  roomId: number;
  aboveBottomBar?: boolean;
}) {
  const me = useAuthStore((s) => s.user);
  const players = useRoomStore((s) => s.players);
  const viewerSeat =
    me?.id != null ? (players.find((p) => p.userId === me.id)?.seat ?? null) : null;

  const resolveNameColor = useCallback(
    (userId: number): string | undefined => {
      const senderSeat = players.find((p) => p.userId === userId)?.seat ?? null;
      if (viewerSeat === null || senderSeat === null) return "var(--accent)";
      return viewerSeat % 2 === senderSeat % 2 ? "var(--team-a)" : "var(--team-b)";
    },
    [players, viewerSeat],
  );

  return (
    <ChatDock
      variant="room"
      roomId={roomId}
      resolveNameColor={resolveNameColor}
      launcherClassName={aboveBottomBar ? "bottom-19" : undefined}
    />
  );
}
