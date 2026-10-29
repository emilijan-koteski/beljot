import { useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { toast } from "sonner";

import { queryKeys } from "@/shared/api/queryKeys";
import { getMyRoom, type MyRoom } from "@/shared/api/rooms";

function pathForRoom(room: MyRoom): string {
  if (room.status === "playing") return `/match/${room.roomId}`;
  if (room.isQuickPlay) return `/matchmaking/${room.roomId}`;
  return `/rooms/${room.roomId}`;
}

/**
 * Shows the "already in a room" error with a button back to that room or match, so the user is never stuck.
 * When the server finds no room any more (it cleared a broken one), it asks the user to try again instead.
 */
export function useAlreadyInRoomToast(): (message: string) => Promise<void> {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  return useCallback(
    async (message: string) => {
      let room: MyRoom | null = null;
      try {
        room = await queryClient.fetchQuery({
          queryKey: queryKeys.rooms.mine,
          queryFn: getMyRoom,
          staleTime: 0,
        });
      } catch {
        toast.error(message);
        return;
      }
      if (room === null) {
        toast.error(t("lobby.errors.alreadyInRoomCleared"));
        return;
      }
      const path = pathForRoom(room);
      toast.error(message, {
        action: {
          label: t(
            room.status === "playing" ? "lobby.errors.backToMatch" : "lobby.errors.backToRoom",
          ),
          onClick: () => navigate(path, { state: { fromRoom: true } }),
        },
      });
    },
    [navigate, queryClient, t],
  );
}
