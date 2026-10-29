import { DoorOpen, Radio } from "lucide-react";
import { useTranslation } from "react-i18next";

import { RoomCard } from "@/features/lobby/components/RoomCard";
import { EmptyState } from "@/shared/components/EmptyState";
import { Button } from "@/shared/components/ui/button";
import type { Room } from "@/shared/types/apiTypes";

export const ROOM_GRID =
  "grid grid-cols-2 gap-2.5 md:grid-cols-3 md:gap-3 lg:grid-cols-[repeat(auto-fill,minmax(13rem,1fr))]";

type Props = {
  rooms: Room[];
  onJoin?: (room: Room) => void;
  live?: boolean;
  hasSearch?: boolean;
  onClearSearch?: () => void;
};

/**
 * Room cards in two columns on a phone, three on a tablet, and as many 208px cards as fit on a desktop.
 * `live` lists matches in play, read-only.
 */
export function RoomGrid({ rooms, onJoin, live = false, hasSearch, onClearSearch }: Props) {
  const { t } = useTranslation();

  if (rooms.length === 0) {
    if (live) {
      return (
        <EmptyState
          size="sm"
          testId="room-list-empty-live"
          icon={<Radio />}
          title={t("lobby.empty.liveTitle")}
        >
          {t("lobby.empty.liveDescription")}
        </EmptyState>
      );
    }
    return (
      <EmptyState
        size="sm"
        testId={hasSearch ? "room-list-empty-search" : "room-list-empty"}
        icon={<DoorOpen />}
        title={t("lobby.empty.title")}
        action={
          hasSearch &&
          onClearSearch && (
            <Button
              variant="link"
              size="inline"
              className="text-xs"
              onClick={onClearSearch}
              data-testid="room-list-clear-search-empty"
            >
              {t("lobby.empty.clear")}
            </Button>
          )
        }
      >
        {t("lobby.empty.description")}
      </EmptyState>
    );
  }

  return (
    <div className={ROOM_GRID} data-testid="room-grid">
      {rooms.map((room) => (
        <RoomCard key={room.id} room={room} onJoin={onJoin} live={live} />
      ))}
    </div>
  );
}
