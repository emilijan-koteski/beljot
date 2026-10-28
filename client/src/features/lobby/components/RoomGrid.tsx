import { DoorOpen } from "lucide-react";
import { useTranslation } from "react-i18next";

import { RoomCard } from "@/features/lobby/components/RoomCard";
import { EmptyState } from "@/shared/components/EmptyState";
import type { Room } from "@/shared/types/apiTypes";

type Props = {
  rooms: Room[];
  onJoin: (room: Room) => void;
  hasSearch?: boolean;
  onClearSearch?: () => void;
};

/**
 * List-layout grid of room cards, each staggered in by its index.
 * Shows an empty state, with a "Clear search" action when a search is active.
 */
export function RoomGrid({ rooms, onJoin, hasSearch, onClearSearch }: Props) {
  const { t } = useTranslation();

  if (rooms.length === 0) {
    return (
      <EmptyState
        testId={hasSearch ? "room-list-empty-search" : "room-list-empty"}
        icon={<DoorOpen />}
        title={t("lobby.empty.title")}
        action={
          hasSearch &&
          onClearSearch && (
            <button
              onClick={onClearSearch}
              data-testid="room-list-clear-search-empty"
              className="text-accent text-sm font-semibold hover:underline"
            >
              {t("lobby.empty.clear")}
            </button>
          )
        }
      >
        {t("lobby.empty.description")}
      </EmptyState>
    );
  }

  return (
    <div className="grid grid-cols-1 gap-3.5 md:grid-cols-2 xl:grid-cols-3">
      {rooms.map((room, i) => (
        <RoomCard key={room.id} room={room} onJoin={onJoin} index={i} />
      ))}
    </div>
  );
}
