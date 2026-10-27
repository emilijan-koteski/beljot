import { DoorOpen } from "lucide-react";
import { useTranslation } from "react-i18next";

import { RoomCard } from "@/features/lobby/components/RoomCard";
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
      <div
        data-testid={hasSearch ? "room-list-empty-search" : "room-list-empty"}
        className="bg-surface flex flex-col items-center gap-3 rounded-lg border border-dashed border-brass px-5 py-14 text-center"
      >
        <span className="bg-accent-soft text-accent flex size-14 items-center justify-center rounded-full">
          <DoorOpen className="size-6" />
        </span>
        <h3 className="text-ink m-0 text-lg font-semibold">{t("lobby.empty.title")}</h3>
        <p className="text-ink-dim m-0 max-w-90 text-xs">{t("lobby.empty.description")}</p>
        {hasSearch && onClearSearch && (
          <button
            onClick={onClearSearch}
            data-testid="room-list-clear-search-empty"
            className="text-accent text-xs font-semibold hover:underline"
          >
            {t("lobby.empty.clear")}
          </button>
        )}
      </div>
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
