import { Armchair, Bot, Crown, Hourglass, Send, Users, UserX } from "lucide-react";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { FriendButton } from "@/shared/components/friends/FriendButton";
import { HonorShield } from "@/shared/components/HonorShield";
import { AppDialog } from "@/shared/components/ui/app-dialog";
import { Avatar } from "@/shared/components/ui/avatar";
import { Button } from "@/shared/components/ui/button";
import { Eyebrow } from "@/shared/components/ui/eyebrow";
import { normalizeHonorTier } from "@/shared/lib/honor";
import { cn } from "@/shared/lib/utils";
import type { RoomPlayer } from "@/shared/types/apiTypes";

import type { TableSeat } from "../lib/tableSeat";

export interface RoomRosterProps {
  table: TableSeat[];
  standing: RoomPlayer[];
  viewerId: number | null;
  ownerId: number;
  canManage: boolean;
  canInvite: boolean;
  className?: string;
  onPromote: (player: RoomPlayer) => void;
  onKick: (player: RoomPlayer) => void;
  onRemoveBot: (seat: number) => void;
  onInvite: () => void;
}

/** Everyone in the room: the table round from the viewer, open seats included, then whoever has no seat. */
export function RoomRoster({ table, standing, className, ...rest }: RoomRosterProps) {
  const { t } = useTranslation();
  const seated = table.filter((s) => s.player !== undefined).length;
  return (
    <div className={cn("flex flex-col gap-3", className)}>
      <section>
        <Eyebrow tone="mute" className="mb-0.5">
          {t("room.roster.atTable", { count: seated })}
        </Eyebrow>
        <ul className="divide-border m-0 list-none divide-y p-0">
          {table.map((s) =>
            s.player === undefined ? (
              <OpenSeatRow
                key={s.seat}
                seat={s}
                canInvite={rest.canInvite}
                onInvite={rest.onInvite}
              />
            ) : (
              <PlayerRow
                key={s.seat}
                player={s.player}
                name={s.name}
                team={s.team}
                returning={s.returning}
                {...rest}
              />
            ),
          )}
        </ul>
      </section>
      {standing.length > 0 && (
        <section>
          <Eyebrow tone="mute" className="mb-0.5">
            {t("room.roster.notSeated", { count: standing.length })}
          </Eyebrow>
          <ul className="divide-border m-0 list-none divide-y p-0">
            {standing.map((p) => (
              <PlayerRow
                key={p.userId}
                player={p}
                name={p.username}
                team={null}
                returning={false}
                {...rest}
              />
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}

interface RoomRosterDialogProps extends RoomRosterProps {
  open: boolean;
  onClose: () => void;
}

/** "N in room" opened on a phone: the roster in the app's dialog. */
export function RoomRosterDialog({ open, onClose, ...roster }: RoomRosterDialogProps) {
  const { t } = useTranslation();
  return (
    <AppDialog
      open={open}
      onClose={onClose}
      icon={<Users />}
      title={t("room.roster.title")}
      primary={{ label: t("common.done"), onClick: onClose }}
      testId="in-room-list"
    >
      <RoomRoster {...roster} />
    </AppDialog>
  );
}

function OpenSeatRow({
  seat,
  canInvite,
  onInvite,
}: {
  seat: TableSeat;
  canInvite: boolean;
  onInvite: () => void;
}) {
  const { t } = useTranslation();
  return (
    <li className="flex items-center gap-2.5 py-2">
      <span
        className="bg-surface flex size-8 shrink-0 items-center justify-center rounded-full border-2 border-dashed"
        style={{
          borderColor:
            seat.team === "A"
              ? "var(--team-a-edge)"
              : seat.team === "B"
                ? "var(--team-b-edge-soft)"
                : "var(--brass)",
        }}
      >
        <Armchair className="text-ink-mute size-3.5" aria-hidden="true" />
      </span>
      <span className="text-ink-dim min-w-0 flex-1 text-sm font-semibold">
        {t("room.seat.open")}
      </span>
      {canInvite && (
        <Button
          size="sm"
          variant="outline"
          onClick={onInvite}
          data-testid={`roster-invite-${seat.seat}`}
        >
          <Send className="size-3.5" />
          {t("roomInvite.panel.invite")}
        </Button>
      )}
    </li>
  );
}

interface PlayerRowProps extends Omit<
  RoomRosterProps,
  "table" | "standing" | "className" | "canInvite" | "onInvite"
> {
  player: RoomPlayer;
  name: string;
  team: "A" | "B" | null;
  returning: boolean;
}

function PlayerRow({
  player,
  name,
  team,
  returning,
  viewerId,
  ownerId,
  canManage,
  onPromote,
  onKick,
  onRemoveBot,
}: PlayerRowProps) {
  const { t } = useTranslation();
  const bot = player.isBot === true;
  const isYou = !bot && viewerId === player.userId;
  const isHost = !bot && player.userId === ownerId;
  const canAct = canManage && !isYou && !isHost;
  return (
    <li
      className="flex items-center gap-2.5 py-2"
      data-testid={bot ? `in-room-list-bot-${player.seat}` : `in-room-list-item-${player.userId}`}
    >
      <span className={cn("shrink-0", returning && "opacity-60")}>
        <Avatar
          name={name}
          avatarUrl={bot ? null : player.avatarUrl}
          size={32}
          team={team}
          icon={bot ? <Bot aria-hidden="true" /> : undefined}
        />
      </span>
      <span className="flex min-w-0 flex-1 flex-col leading-tight">
        <span className="text-ink inline-flex min-w-0 items-center gap-1 text-sm font-semibold">
          <span className="truncate">{name}</span>
          {isHost && (
            <Crown className="text-brass-deep size-3 shrink-0" aria-label={t("room.seatOwner")} />
          )}
          {isYou && (
            <span className="text-ink-mute shrink-0 font-normal">· {t("room.seatYouInline")}</span>
          )}
        </span>
        <RosterLine player={player} returning={returning} />
      </span>
      {canAct && bot && player.seat !== null && (
        <Button
          size="icon-sm"
          variant="outline"
          onClick={() => onRemoveBot(player.seat as number)}
          aria-label={t("room.removeBotIconLabel", { name })}
          title={t("room.removeBotIconLabel", { name })}
          data-testid={`roster-remove-bot-${player.seat}`}
          className="text-danger border-danger/30 hover:bg-danger/10 hover:text-danger"
        >
          <UserX className="size-3.5" />
        </Button>
      )}
      {canAct && !bot && (
        <span className="flex shrink-0 items-center gap-1">
          {player.seat !== null && (
            <Button
              size="icon-sm"
              variant="outline"
              onClick={() => onPromote(player)}
              aria-label={t("room.promoteIconLabel", { username: player.username })}
              title={t("room.promoteIconLabel", { username: player.username })}
              data-testid={`promote-player-${player.userId}`}
            >
              <Crown className="size-3.5" />
            </Button>
          )}
          <Button
            size="icon-sm"
            variant="outline"
            onClick={() => onKick(player)}
            aria-label={t("room.kickIconLabel", { username: player.username })}
            title={t("room.kickIconLabel", { username: player.username })}
            data-testid={`kick-list-${player.userId}`}
            className="text-danger border-danger/30 hover:bg-danger/10 hover:text-danger"
          >
            <UserX className="size-3.5" />
          </Button>
        </span>
      )}
      {!canManage && !bot && !isYou && (
        <FriendButton
          userId={player.userId}
          username={player.username}
          compact
          allowRemove={false}
        />
      )}
    </li>
  );
}

function RosterLine({ player, returning }: { player: RoomPlayer; returning: boolean }) {
  const { t } = useTranslation();
  if (player.isBot === true)
    return <span className="text-bot-deep text-2xs">{t("bots.badge")}</span>;
  const parts: ReactNode[] = [];
  if (typeof player.level === "number") parts.push(t("xp.short", { level: player.level }));
  if (typeof player.honorScore === "number") {
    parts.push(
      <span key="honor" className="inline-flex items-center gap-0.5">
        <HonorShield
          tier={normalizeHonorTier(player.honorTier ?? "", player.honorScore)}
          size={10}
        />
        {player.honorScore}
      </span>,
    );
  }
  if (returning) {
    parts.push(
      <span key="returning" className="inline-flex items-center gap-0.5">
        <Hourglass className="size-2.5" aria-hidden="true" />
        {t("room.seat.returning")}
      </span>,
    );
  }
  if (parts.length === 0) return null;
  return (
    <span className="text-ink-mute inline-flex items-center gap-1 text-2xs tabular-nums">
      {parts.map((part, i) => (
        <span key={i} className="inline-flex items-center gap-1">
          {i > 0 && <span aria-hidden="true">·</span>}
          {part}
        </span>
      ))}
    </span>
  );
}
