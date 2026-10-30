import { Armchair, Bot, Crown, Hourglass, MoreHorizontal } from "lucide-react";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HonorShield } from "@/shared/components/HonorShield";
import { Avatar } from "@/shared/components/ui/avatar";
import { Button } from "@/shared/components/ui/button";
import { normalizeHonorTier } from "@/shared/lib/honor";
import { cn } from "@/shared/lib/utils";
import type { RoomPlayer } from "@/shared/types/apiTypes";

import { BOT_BUTTON_CLASS } from "../lib/botButton";
import type { Cardinal, TableSeat } from "../lib/tableSeat";

interface RoomTableProps {
  seats: TableSeat[];
  layout: "compact" | "wide";
  swapMode: boolean;
  center: ReactNode;
  onSeat: (seat: number) => void;
  onAddBot: (seat: number) => void;
}

const AREA: Record<Cardinal, string> = {
  south: "[grid-area:south]",
  east: "[grid-area:east]",
  north: "[grid-area:north]",
  west: "[grid-area:west]",
};

/**
 * The room's table: four seats around a parchment cross, the viewer at the bottom. A phone draws avatars
 * with the open seat's Invite and Bot under its chair; wider screens draw seat cards with the actions inside.
 */
export function RoomTable({ seats, layout, swapMode, center, onSeat, onAddBot }: RoomTableProps) {
  const wide = layout === "wide";
  return (
    <div
      data-testid="seats-grid"
      className={cn(
        "border-brass/40 relative grid items-center justify-items-center border [grid-template-areas:'._north_.''west_center_east''._south_.']",
        wide
          ? "grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] gap-4 rounded-[2.5rem] p-6"
          : "grid-cols-3 gap-y-2.5 rounded-[2rem] px-1 py-3",
      )}
      style={{
        background:
          "radial-gradient(ellipse 65% 58% at 50% 50%, var(--brass-soft), transparent 75%), var(--surface)",
      }}
    >
      {!wide && <TeamLines />}
      {seats.map((s) => (
        <div
          key={s.seat}
          data-testid={`seat-position-${s.cardinal}`}
          data-team={s.seat % 2 === 0 ? "teamA" : "teamB"}
          data-below-honor-bar={s.belowHonorBar ? "true" : undefined}
          className={cn(
            "relative z-10 flex min-w-0 justify-center",
            AREA[s.cardinal],
            wide && "w-full max-w-60",
          )}
        >
          {wide ? (
            <SeatCard seat={s} swapMode={swapMode} onSeat={onSeat} onAddBot={onAddBot} />
          ) : (
            <SeatBubble seat={s} swapMode={swapMode} onSeat={onSeat} onAddBot={onAddBot} />
          )}
        </div>
      ))}
      <div className="relative z-10 [grid-area:center]">{center}</div>
    </div>
  );
}

function TeamLines() {
  return (
    <>
      <div
        aria-hidden="true"
        className="pointer-events-none absolute top-[18%] bottom-[18%] left-1/2 w-0.5 -translate-x-1/2 opacity-45"
        style={{
          background:
            "linear-gradient(180deg, transparent, var(--team-a-edge) 25%, var(--team-a-edge) 75%, transparent)",
        }}
      />
      <div
        aria-hidden="true"
        className="pointer-events-none absolute top-1/2 right-[16%] left-[16%] h-0.5 -translate-y-1/2 opacity-60"
        style={{
          background:
            "linear-gradient(90deg, transparent, var(--team-b-edge-soft) 25%, var(--team-b-edge-soft) 75%, transparent)",
        }}
      />
    </>
  );
}

interface SeatViewProps {
  seat: TableSeat;
  swapMode: boolean;
  onSeat: (seat: number) => void;
  onAddBot: (seat: number) => void;
}

function edgeFor(team: "A" | "B" | null): string {
  if (team === "A") return "var(--team-a-edge)";
  if (team === "B") return "var(--team-b-edge-soft)";
  return "var(--brass)";
}

function OpenSeatActions({ seat, swapMode, onAddBot }: SeatViewProps) {
  const { t } = useTranslation();
  if (swapMode) return null;
  return (
    <>
      {seat.canAddBot && (
        <Button
          size="sm"
          variant="outline"
          onClick={() => onAddBot(seat.seat)}
          disabled={seat.pending}
          aria-label={t("room.addBot")}
          data-testid={`add-bot-seat-${seat.seat}`}
          className={BOT_BUTTON_CLASS}
        >
          <Bot className="size-3.5" />
          {t("bots.badge")}
        </Button>
      )}
    </>
  );
}

function SeatStats({ player, seat }: { player: RoomPlayer; seat: number }) {
  const { t } = useTranslation();
  const honor = player.honorScore;
  const hasLevel = typeof player.level === "number";
  const hasHonor = typeof honor === "number";
  if (!hasLevel && !hasHonor) return null;
  return (
    <span className="text-ink-mute inline-flex items-center gap-1 text-2xs leading-none tabular-nums">
      {hasLevel && (
        <span
          data-testid={`seat-level-${seat}`}
          data-level={player.level}
          title={t("xp.levelLabel", { level: player.level })}
        >
          {t("xp.short", { level: player.level })}
        </span>
      )}
      {hasLevel && hasHonor && <span aria-hidden="true">·</span>}
      {typeof honor === "number" && (
        <span
          className="inline-flex items-center gap-0.5"
          data-testid={`seat-honor-${seat}`}
          data-honor={honor}
        >
          <HonorShield tier={normalizeHonorTier(player.honorTier ?? "", honor)} size={10} />
          {honor}
        </span>
      )}
    </span>
  );
}

function SeatName({ seat, className }: { seat: TableSeat; className?: string }) {
  const { t } = useTranslation();
  const bot = seat.player?.isBot === true;
  return (
    <span
      className={cn(
        "max-w-full truncate font-semibold",
        bot
          ? "text-bot-deep"
          : seat.team === "B"
            ? "text-team-b"
            : seat.team === "A"
              ? "text-team-a"
              : "text-ink",
        className,
      )}
      data-testid={bot ? `bot-name-${seat.seat}` : undefined}
    >
      {seat.name}
      {seat.isYou && (
        <span className="text-ink-mute font-normal"> · {t("room.seatYouInline")}</span>
      )}
    </span>
  );
}

function SeatBadges({ seat, size }: { seat: TableSeat; size: "sm" | "md" }) {
  const { t } = useTranslation();
  const badge = size === "sm" ? "size-5" : "size-5.5";
  return (
    <>
      {seat.isHost && (
        <span
          title={t("room.seatOwner")}
          className={cn(
            "bg-brass text-brass-ink absolute -top-1 -right-1 flex items-center justify-center rounded-full border-2 border-(--surface)",
            badge,
          )}
        >
          <Crown className="size-2.5" aria-hidden="true" />
        </span>
      )}
      {seat.returning && (
        <span
          title={t("room.seat.returning")}
          data-testid={`waiting-to-return-${seat.seat}`}
          className={cn(
            "bg-surface text-ink-dim border-border absolute -right-1 -bottom-1 flex items-center justify-center rounded-full border",
            badge,
          )}
        >
          <Hourglass className="size-3 animate-pulse" aria-hidden="true" />
        </span>
      )}
    </>
  );
}

function avatarRing(seat: TableSeat): string | undefined {
  if (seat.belowHonorBar) return "ring-3 ring-(--h1-line) ring-offset-2 ring-offset-(--surface)";
  if (seat.swapSource) return "ring-3 ring-accent ring-offset-2 ring-offset-(--surface)";
  return undefined;
}

function SeatBubble(props: SeatViewProps) {
  const { seat, swapMode, onSeat } = props;
  const { t } = useTranslation();
  if (seat.player === undefined) {
    return (
      <div className="flex w-28 flex-col items-center">
        <button
          type="button"
          onClick={() => onSeat(seat.seat)}
          disabled={!seat.clickable || seat.pending}
          data-testid={`player-seat-${seat.seat}`}
          className="group flex cursor-pointer flex-col items-center gap-1 disabled:cursor-default disabled:opacity-60"
        >
          <span
            className="bg-surface group-hover:bg-brass-soft flex size-13 items-center justify-center rounded-full border-2 border-dashed transition-colors"
            style={{ borderColor: edgeFor(seat.team) }}
          >
            <Armchair className="text-ink-dim size-5" aria-hidden="true" />
          </span>
          <span className="text-ink-dim text-xs font-semibold">
            {swapMode ? t("room.seat.moveHere") : t("room.seat.sit")}
          </span>
        </button>
        {seat.canAddBot && !swapMode && (
          <Button
            size="sm"
            variant="outline"
            onClick={() => props.onAddBot(seat.seat)}
            disabled={seat.pending}
            aria-label={t("room.addBot")}
            data-testid={`add-bot-seat-${seat.seat}`}
            className={cn("mt-1 h-7 gap-1 px-2 text-xs", BOT_BUTTON_CLASS)}
          >
            <Bot className="size-3.5" />
            {t("bots.badge")}
          </Button>
        )}
      </div>
    );
  }
  const bot = seat.player.isBot === true;
  return (
    <button
      type="button"
      onClick={() => onSeat(seat.seat)}
      disabled={!seat.clickable || seat.pending}
      data-testid={`player-seat-${seat.seat}`}
      className={cn(
        "flex w-28 flex-col items-center gap-1 disabled:cursor-default",
        seat.clickable && "cursor-pointer",
        seat.pending && "opacity-60",
        seat.returning && "[&>span:first-child]:opacity-60",
      )}
    >
      <span className={cn("relative rounded-full", avatarRing(seat))}>
        <Avatar
          name={seat.name}
          avatarUrl={bot ? null : seat.player.avatarUrl}
          size={52}
          team={seat.team}
          icon={bot ? <Bot aria-hidden="true" /> : undefined}
        />
        <SeatBadges seat={seat} size="sm" />
      </span>
      <SeatName seat={seat} className="text-xs" />
      {bot ? (
        <span className="text-bot -mt-0.5 text-2xs" data-testid={`bot-badge-${seat.seat}`}>
          {t("bots.badge")}
        </span>
      ) : (
        <SeatStats player={seat.player} seat={seat.seat} />
      )}
    </button>
  );
}

function SeatCard(props: SeatViewProps) {
  const { seat, swapMode, onSeat } = props;
  const { t } = useTranslation();
  if (seat.player === undefined) {
    return (
      <div
        className="bg-surface/70 flex w-full flex-col items-center gap-2 rounded-xl border-2 border-dashed px-3 py-3"
        style={{ borderColor: edgeFor(seat.team) }}
      >
        <span className="text-ink-dim text-sm font-semibold">{t("room.seat.open")}</span>
        <div className="flex flex-wrap items-center justify-center gap-1.5">
          <Button
            size="sm"
            variant="outline"
            onClick={() => onSeat(seat.seat)}
            disabled={!seat.clickable || seat.pending}
            data-testid={`player-seat-${seat.seat}`}
          >
            <Armchair className="size-3.5" />
            {swapMode ? t("room.seat.moveHere") : t("room.seat.sit")}
          </Button>
          <OpenSeatActions {...props} />
        </div>
      </div>
    );
  }
  const bot = seat.player.isBot === true;
  return (
    <button
      type="button"
      onClick={() => onSeat(seat.seat)}
      disabled={!seat.clickable || seat.pending}
      data-testid={`player-seat-${seat.seat}`}
      className={cn(
        "flex w-full items-center gap-3 rounded-xl border px-3 py-2.5 text-left shadow-xs transition-[border-color,box-shadow] disabled:cursor-default",
        seat.clickable && "hover:border-brass cursor-pointer",
        seat.pending && "opacity-60",
      )}
      style={{
        borderColor: edgeFor(seat.team),
        background:
          seat.team === "A"
            ? "var(--team-a-tint)"
            : seat.team === "B"
              ? "var(--team-b-tint)"
              : "var(--surface)",
      }}
    >
      <span
        className={cn(
          "relative shrink-0 rounded-full",
          avatarRing(seat),
          seat.returning && "opacity-60",
        )}
      >
        <Avatar
          name={seat.name}
          avatarUrl={bot ? null : seat.player.avatarUrl}
          size={44}
          team={seat.team}
          icon={bot ? <Bot aria-hidden="true" /> : undefined}
        />
        <SeatBadges seat={seat} size="md" />
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-1 leading-tight">
        <SeatName seat={seat} className="text-sm" />
        {bot ? (
          <span className="text-bot-deep text-xs" data-testid={`bot-badge-${seat.seat}`}>
            {t("bots.badge")}
          </span>
        ) : (
          <SeatStats player={seat.player} seat={seat.seat} />
        )}
      </span>
      {seat.hostMenu && (
        <MoreHorizontal className="text-ink-mute size-4 shrink-0" aria-hidden="true" />
      )}
    </button>
  );
}
