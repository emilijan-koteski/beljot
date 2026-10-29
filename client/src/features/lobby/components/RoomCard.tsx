import type { TFunction } from "i18next";
import { Bot, Coins, Lock, Zap } from "lucide-react";
import { useTranslation } from "react-i18next";

import { HonorShield } from "@/shared/components/HonorShield";
import { RoomRulesPopover } from "@/shared/components/roomRules/RoomRules";
import { Avatar } from "@/shared/components/ui/avatar";
import { Badge } from "@/shared/components/ui/badge";
import { DetailRow, DetailRows } from "@/shared/components/ui/detail-rows";
import { botDisplayName } from "@/shared/lib/botName";
import { COIN_GOLD } from "@/shared/lib/coinGold";
import { formatCoins } from "@/shared/lib/formatCoins";
import {
  honorFloorLabel,
  type HonorGateViewer,
  honorIsNewPlayer,
  honorQualifies,
  honorScoreOrPrior,
  honorTierForScore,
} from "@/shared/lib/honor";
import { variantLabel } from "@/shared/lib/roomLabels";
import { isRoomFull } from "@/shared/lib/roomOccupancy";
import { cn } from "@/shared/lib/utils";
import { useAuthStore } from "@/shared/stores/authStore";
import type { Room } from "@/shared/types/apiTypes";

const CARD =
  "bg-surface border-border relative flex min-w-0 flex-col gap-2.5 rounded-xl border p-3 text-left shadow-xs";
const CARD_JOINABLE =
  "transition-[border-color,box-shadow,translate] hover:-translate-y-0.5 hover:border-brass hover:shadow-md";
const CARD_DISABLED = "opacity-60";
const JOIN_OVERLAY =
  "absolute inset-0 cursor-pointer rounded-xl outline-none focus-visible:ring-3 focus-visible:ring-ring/70 disabled:cursor-default";
const SEAT_AVATAR = 30;
const TEAM_ORDER = [0, 2, 1, 3] as const;

type Props = {
  room: Room;
  onJoin?: (room: Room) => void;
  live?: boolean;
  preview?: boolean;
};

function blockedReason(t: TFunction, room: Room, viewer: HonorGateViewer, full: boolean): string {
  if (full) return t("lobby.card.fullTitle");
  if (honorIsNewPlayer(viewer.isNewPlayer)) return t("room.errors.newPlayerNotAllowed");
  return t("lobby.card.minHonorLockedTitle", {
    minHonor: room.minHonor,
    honor: honorScoreOrPrior(viewer.honorScore),
  });
}

/**
 * The lobby's room card, one height for every room: the name, six label-value rows and the four seats.
 * A room the viewer can't join (full, or above their honor) dims and ignores taps; `live` cards are read-only
 * and `preview` renders the create-room dialog's copy of the card.
 */
export function RoomCard({ room, onJoin, live = false, preview = false }: Props) {
  const { t } = useTranslation();
  const viewer = useAuthStore((s) => s.user) ?? {};
  const listed = !live && !preview;
  const full = isRoomFull(room);
  const locked = listed && !honorQualifies(room, viewer);
  const disabled = listed && (full || locked);

  return (
    <article
      data-testid="room-card"
      data-room-id={room.id}
      data-state={live ? "live" : disabled ? "disabled" : "open"}
      className={cn(CARD, listed && !disabled && CARD_JOINABLE, disabled && CARD_DISABLED)}
    >
      {listed && (
        <button
          type="button"
          onClick={() => onJoin?.(room)}
          disabled={disabled}
          title={disabled ? blockedReason(t, room, viewer, full) : undefined}
          aria-label={t("lobby.card.joinAria", { name: room.name })}
          data-testid="room-card-join"
          className={JOIN_OVERLAY}
        />
      )}
      <div className="flex min-w-0 items-center gap-1.5">
        {room.isQuickPlay && (
          <Zap
            className="text-accent size-3.5 shrink-0"
            role="img"
            aria-label={t("lobby.card.quickPlay")}
          />
        )}
        {room.isPrivate && (
          <Lock
            className="text-brass-deep size-3.5 shrink-0"
            role="img"
            aria-label={t("lobby.card.private")}
          />
        )}
        <h3 className="text-ink m-0 min-w-0 truncate text-sm font-semibold">{room.name}</h3>
      </div>
      <DetailRows>
        <DetailRow label={t("lobby.card.rows.variant")}>{variantLabel(t, room.variant)}</DetailRow>
        <DetailRow label={t("lobby.card.rows.target")}>{room.matchMode}</DetailRow>
        <DetailRow label={t("lobby.card.rows.timer")}>
          {room.timerStyle === "relaxed"
            ? t("lobby.card.relaxed")
            : t("lobby.card.timerShort", { seconds: room.timerDurationSeconds ?? 0 })}
        </DetailRow>
        <DetailRow label={t("lobby.card.rows.buyIn")} testId="room-card-buy-in">
          {room.coinBuyIn > 0 ? (
            <>
              <Coins className="size-3" style={{ color: COIN_GOLD }} aria-hidden="true" />
              {formatCoins(room.coinBuyIn)}
            </>
          ) : (
            t("lobby.card.free")
          )}
        </DetailRow>
        <DetailRow
          label={t("lobby.card.rows.honor")}
          tone={locked ? "danger" : undefined}
          testId="room-card-honor"
        >
          {room.minHonor > 0 ? (
            <>
              <HonorShield tier={honorTierForScore(room.minHonor)} size={12} />
              {honorFloorLabel(room.minHonor)}
            </>
          ) : (
            t("lobby.card.honorAny")
          )}
        </DetailRow>
        <DetailRow label={t("lobby.card.rows.rules")}>
          <RoomRulesPopover room={room} live={live} className="relative z-10" />
        </DetailRow>
      </DetailRows>
      <RoomSeats room={room} />
      {live && (
        <span className="flex text-2xs">
          <Badge
            size="sm"
            tone="accent"
            icon={<span className="size-1.25 rounded-full bg-current" />}
          >
            {t("lobby.card.live")}
          </Badge>
        </span>
      )}
    </article>
  );
}

/** The four seats in team order (0 and 2, a divider, 1 and 3): avatars, a blue bot face for bots, dashed rings when open. */
function RoomSeats({ room }: { room: Room }) {
  const { t } = useTranslation();
  return (
    <div className="flex items-center justify-between" data-testid="room-card-seats">
      {TEAM_ORDER.map((seat, i) => {
        const team = seat % 2 === 0 ? "A" : "B";
        const player = room.players?.find((p) => p.seat === seat);
        const bot = player?.isBot === true;
        const name = player
          ? bot
            ? botDisplayName(t, seat)
            : player.username
          : t("lobby.card.openSeat");
        return (
          <span
            key={seat}
            className={cn("flex", i === 2 && "border-border border-l pl-1.5")}
            data-testid={`room-${room.id}-seat-${seat}`}
            data-seat={player ? (bot ? "bot" : "player") : "open"}
          >
            {player ? (
              <Avatar
                name={name}
                avatarUrl={bot ? null : player.avatarUrl}
                size={SEAT_AVATAR}
                team={team}
                icon={bot ? <Bot className="text-bot" aria-hidden="true" /> : undefined}
              />
            ) : (
              <span
                className="bg-surface-sunken size-7.5 rounded-full border-2 border-dashed"
                style={{
                  borderColor: team === "A" ? "var(--team-a-edge-soft)" : "var(--team-b-edge-soft)",
                }}
              />
            )}
            <span className="sr-only">{name}</span>
          </span>
        );
      })}
    </div>
  );
}
