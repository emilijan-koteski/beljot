import { Bot, ChevronDown, Crown, Users, UserX } from "lucide-react";
import { useTranslation } from "react-i18next";

import { EmptyState } from "@/shared/components/EmptyState";
import { Avatar } from "@/shared/components/ui/avatar";
import { Badge } from "@/shared/components/ui/badge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/shared/components/ui/dropdown-menu";
import { botDisplayName } from "@/shared/lib/botName";
import { cn } from "@/shared/lib/utils";
import type { RoomPlayer } from "@/shared/types/apiTypes";

interface RoomPlayersMenuProps {
  players: RoomPlayer[];
  ownerId: number;
  viewerId: number | null;
  canManage: boolean;
  teamOf: (player: RoomPlayer) => "A" | "B" | null;
  promotePending: boolean;
  kickPending: boolean;
  onPromote: (player: RoomPlayer) => void;
  onKick: (player: RoomPlayer) => void;
}

const ROW_ACTION = "size-6 shrink-0 justify-center p-0";

/**
 * "N in room": the room header's roster, with the host's Make host and Kick actions as menu items so the
 * arrow keys move between them. `canManage` is true only for the host while the room is waiting.
 */
export function RoomPlayersMenu({
  players,
  ownerId,
  viewerId,
  canManage,
  teamOf,
  promotePending,
  kickPending,
  onPromote,
  onKick,
}: RoomPlayersMenuProps) {
  const { t } = useTranslation();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className="text-ink hover:bg-surface-sunken focus-visible:ring-accent border-border inline-flex items-center gap-1.5 rounded-md border px-2 py-0.5 transition-colors outline-none focus-visible:ring-2"
        aria-label={t("room.inRoomList.ariaLabel")}
        data-testid="in-room-count"
      >
        <Users className="text-ink-dim size-3.5" />
        <span>{t("room.inRoomCount", { count: players.length })}</span>
        <ChevronDown className="text-ink-mute size-3" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="min-w-64" data-testid="in-room-list">
        <div className="text-brass-deep px-2 pt-1.5 pb-1 font-mono text-2xs font-semibold tracking-caps uppercase">
          {t("room.inRoomList.title")}
        </div>
        {players.length === 0 ? (
          <EmptyState compact>{t("room.inRoomList.empty")}</EmptyState>
        ) : (
          <ul role="none" className="flex flex-col">
            {players.map((p) => (
              <RosterRow
                key={p.isBot === true ? `bot-${p.seat}` : p.userId}
                player={p}
                ownerId={ownerId}
                viewerId={viewerId}
                canManage={canManage}
                team={teamOf(p)}
                promotePending={promotePending}
                kickPending={kickPending}
                onPromote={onPromote}
                onKick={onKick}
              />
            ))}
          </ul>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

interface RosterRowProps extends Omit<RoomPlayersMenuProps, "players" | "teamOf"> {
  player: RoomPlayer;
  team: "A" | "B" | null;
}

/** One roster line: avatar, name, seat badge and, for the host, the row's menu items. */
function RosterRow({
  player: p,
  ownerId,
  viewerId,
  canManage,
  team,
  promotePending,
  kickPending,
  onPromote,
  onKick,
}: RosterRowProps) {
  const { t } = useTranslation();
  const isBot = p.isBot === true;
  const isYou = !isBot && viewerId === p.userId;
  const isRoomOwner = !isBot && p.userId === ownerId;
  const canKick = canManage && !isRoomOwner && !isYou && !isBot;
  const canPromote = canKick && p.seat !== null;
  const seated = p.seat !== null;
  const name = isBot ? botDisplayName(t, p.seat) : p.username;
  const promoteLabel = t("room.promoteIconLabel", { username: p.username });
  const kickLabel = t("room.kickIconLabel", { username: p.username });

  return (
    <li
      role="none"
      className="flex items-center gap-2 px-2 py-1.5 text-sm"
      data-testid={isBot ? `in-room-list-bot-${p.seat}` : `in-room-list-item-${p.userId}`}
    >
      <Avatar
        name={name}
        avatarUrl={isBot ? null : p.avatarUrl}
        size={24}
        team={team}
        icon={isBot ? <Bot aria-hidden="true" /> : undefined}
      />
      <span className="text-ink flex min-w-0 flex-1 items-center gap-1">
        {isRoomOwner && <Crown className="text-brass-deep size-3 shrink-0" aria-hidden />}
        <span className="truncate font-medium">{name}</span>
        {isYou && (
          <span className="text-accent shrink-0 text-xs font-semibold">
            · {t("room.seatYouInline")}
          </span>
        )}
      </span>
      <Badge tone={seated ? "accent" : "neutral"} size="sm">
        {seated
          ? t("room.inRoomList.seated", { seat: (p.seat as number) + 1 })
          : t("room.inRoomList.notSeated")}
      </Badge>
      {canPromote && (
        <DropdownMenuItem
          className={cn(ROW_ACTION, "text-ink-mute")}
          aria-label={promoteLabel}
          title={promoteLabel}
          data-testid={`promote-player-${p.userId}`}
          disabled={promotePending}
          onClick={() => onPromote(p)}
        >
          <Crown className="size-3.5" />
        </DropdownMenuItem>
      )}
      {canKick && (
        <DropdownMenuItem
          variant="destructive"
          className={ROW_ACTION}
          aria-label={kickLabel}
          title={kickLabel}
          data-testid={`kick-list-${p.userId}`}
          disabled={kickPending}
          onClick={() => onKick(p)}
        >
          <UserX className="size-3.5" />
        </DropdownMenuItem>
      )}
    </li>
  );
}
