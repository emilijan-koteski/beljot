import { Bot, Crown, Hourglass, Shuffle, UserX } from "lucide-react";
import { useTranslation } from "react-i18next";

import { HonorShield } from "@/shared/components/HonorShield";
import { Avatar } from "@/shared/components/ui/avatar";
import { Badge } from "@/shared/components/ui/badge";
import { useMediaQuery } from "@/shared/hooks/useMediaQuery";
import { botDisplayName } from "@/shared/lib/botName";
import { normalizeHonorTier } from "@/shared/lib/honor";
import { cn } from "@/shared/lib/utils";
import type { RoomPlayer } from "@/shared/types/apiTypes";

type SeatMode = "us" | "them" | "neutral";

const SEAT_AREA_CLASS = {
  south: "sm:[grid-area:south]",
  east: "sm:[grid-area:east]",
  north: "sm:[grid-area:north]",
  west: "sm:[grid-area:west]",
} as const;

type SeatTileProps = {
  seatIndex: 0 | 1 | 2 | 3;
  cardinal: "south" | "east" | "north" | "west";
  mode: SeatMode;
  player?: RoomPlayer;
  isYou: boolean;
  isHost: boolean;
  isSwapSource: boolean;
  swapMode: boolean;
  isClickable: boolean;
  isPending: boolean;
  waitingToReturn?: boolean;
  belowHonorBar?: boolean;
  ownerCanActOnRow: boolean;
  onSelect: () => void;
  onKick?: () => void;
  onPromote?: () => void;
  onAddBot?: () => void;
  onRemoveBot?: () => void;
};

/**
 * Per-seat tile in the in-room diamond, painted in the viewer-relative team mode that RoomPage resolves.
 */
export function SeatTile({
  seatIndex,
  cardinal,
  mode,
  player,
  isYou,
  isHost,
  isSwapSource,
  swapMode,
  isClickable,
  isPending,
  waitingToReturn = false,
  belowHonorBar = false,
  ownerCanActOnRow,
  onSelect,
  onKick,
  onPromote,
  onAddBot,
  onRemoveBot,
}: SeatTileProps) {
  const { t } = useTranslation();
  const isCompact = useMediaQuery("(max-width: 639px)");
  const avatarSize = isCompact ? 36 : 44;
  const filled = Boolean(player);
  const isBot = player?.isBot === true;
  const displayName = isBot ? botDisplayName(t, seatIndex) : (player?.username ?? "");
  const showOwnerControls = ownerCanActOnRow && filled && !isYou && !swapMode;
  const showRemoveBot = Boolean(onRemoveBot) && isBot && !swapMode;
  const showAddBot = Boolean(onAddBot) && !filled && !swapMode;
  const showSwapHint = swapMode && filled && !isSwapSource && !isYou;

  const tokens = MODE_TOKENS[mode];
  const tileStyle: React.CSSProperties = {
    background: filled ? tokens.fill : "transparent",
    border: belowHonorBar
      ? "2px solid var(--h1-line)"
      : filled
        ? `2px solid ${tokens.edge}`
        : `2px dashed ${tokens.edgeSoft}`,
    boxShadow: belowHonorBar
      ? `0 0 0 3px var(--h1-soft)`
      : isSwapSource
        ? `0 0 0 3px var(--accent-soft), 0 0 0 2px var(--accent)`
        : isYou
          ? `0 0 0 2px var(--accent), var(--shadow-glow)`
          : filled
            ? "var(--shadow-sm)"
            : "none",
  };

  return (
    <div
      className={cn("group relative", SEAT_AREA_CLASS[cardinal])}
      data-testid={`seat-position-${cardinal}`}
      data-team={seatIndex % 2 === 0 ? "teamA" : "teamB"}
      data-below-honor-bar={belowHonorBar ? "true" : undefined}
    >
      <button
        type="button"
        onClick={onSelect}
        disabled={!isClickable || isPending}
        data-testid={`player-seat-${seatIndex}`}
        className={cn(
          "flex min-h-22 w-full flex-col items-center justify-center gap-1 rounded-2xl p-2.5 transition-all sm:min-h-32.5 sm:gap-2 sm:p-3.5",
          isClickable && !isPending ? "cursor-pointer" : "cursor-default",
          isPending && "pointer-events-none opacity-60",
        )}
        style={tileStyle}
      >
        {filled && player ? (
          <>
            <Avatar
              name={displayName}
              avatarUrl={isBot ? null : player.avatarUrl}
              size={avatarSize}
              team={mode === "us" ? "A" : mode === "them" ? "B" : null}
              icon={isBot ? <Bot aria-hidden="true" /> : undefined}
            />
            <div className="flex flex-col items-center gap-1">
              <span
                className="text-ink inline-flex items-center gap-1 text-sm font-semibold tracking-tight sm:text-sm"
                data-testid={isBot ? `bot-name-${seatIndex}` : undefined}
              >
                {isHost && <Crown className="text-brass-deep size-3.5" aria-hidden="true" />}
                {displayName}
              </span>

              {!isBot &&
                (typeof player.level === "number" || typeof player.honorScore === "number") && (
                  <div className="flex items-center gap-2">
                    {typeof player.level === "number" && (
                      <span
                        className="text-ink-dim text-xs leading-none font-bold whitespace-nowrap tabular-nums"
                        data-testid={`seat-level-${seatIndex}`}
                        data-level={player.level}
                        title={t("xp.levelLabel", { level: player.level })}
                      >
                        {t("xp.short", { level: player.level })}
                      </span>
                    )}
                    {typeof player.honorScore === "number" && (
                      <span
                        className="text-ink-dim inline-flex items-center gap-1 text-xs leading-none font-bold tabular-nums"
                        data-testid={`seat-honor-${seatIndex}`}
                        data-honor={player.honorScore}
                        title={t("profile.honor.meterLabel", {
                          score: player.honorScore,
                          tier: t(
                            `profile.honor.tier.${normalizeHonorTier(player.honorTier ?? "", player.honorScore)}`,
                          ),
                        })}
                      >
                        <HonorShield
                          tier={normalizeHonorTier(player.honorTier ?? "", player.honorScore)}
                          size={12}
                        />
                        {player.honorScore}
                      </span>
                    )}
                  </div>
                )}
              <div className="flex flex-wrap items-center justify-center gap-1">
                {isBot && (
                  <span data-testid={`bot-badge-${seatIndex}`}>
                    <Badge tone="bot" icon={<Bot className="size-2.5" aria-hidden="true" />}>
                      {t("bots.badge")}
                    </Badge>
                  </span>
                )}
                {isYou && <Badge tone="accent">{t("room.seatYou")}</Badge>}
                {isHost && !isYou && <Badge tone="brass">{t("room.seatOwner")}</Badge>}
                {isSwapSource && (
                  <Badge
                    tone="accent"
                    icon={<Shuffle className="size-2.5" style={{ color: "var(--accent-deep)" }} />}
                  >
                    {t("room.seatTile.pickTarget")}
                  </Badge>
                )}
                {!isYou && !isHost && !isBot && !isSwapSource && mode !== "neutral" && (
                  <Badge tone={mode === "us" ? "teamA" : "teamB"}>
                    {mode === "us" ? t("room.seatTile.partner") : t("room.seatTile.opponent")}
                  </Badge>
                )}
                {waitingToReturn && !isBot && (
                  <span data-testid={`waiting-to-return-${seatIndex}`}>
                    <Badge
                      tone="neutral"
                      icon={<Hourglass className="size-2.5 animate-pulse" aria-hidden="true" />}
                    >
                      {t("room.seatTile.waitingToReturn")}
                    </Badge>
                  </span>
                )}
              </div>
            </div>
          </>
        ) : (
          <>
            <div
              className="bg-surface-elevated mt-1 flex size-9 items-center justify-center rounded-full sm:mt-1.5 sm:size-11"
              style={{ border: `2px dashed ${tokens.edgeSoft}` }}
            >
              <span
                className="size-2.5 rounded-full opacity-45 animate-[pulse-dot_1.6s_ease-in-out_infinite]"
                style={{ background: tokens.text }}
              />
            </div>
            <div className="text-center">
              <div className="text-ink-dim text-xs font-semibold sm:text-sm">
                {swapMode ? t("room.seatTile.moveHere") : t("room.seatTile.takeSeat")}
              </div>
              <div className="text-ink-mute mt-0.5 text-2xs sm:text-xs">
                {mode === "us"
                  ? t("room.seatTile.partnerSeat")
                  : mode === "them"
                    ? t("room.seatTile.opponentSeat")
                    : t("room.seatTile.openSeat")}
              </div>
            </div>
          </>
        )}
      </button>

      {showAddBot && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onAddBot?.();
          }}
          disabled={isPending}
          aria-label={t("room.addBot")}
          title={t("room.addBot")}
          data-testid={`add-bot-seat-${seatIndex}`}
          className="bg-bot-soft border-bot-edge text-bot hover:border-bot hover:text-bot-deep absolute top-2 right-2 z-10 inline-flex size-8 items-center justify-center rounded-md border disabled:opacity-40 md:size-7"
        >
          <Bot className="size-5 -translate-y-[5%] md:size-3.5" />
        </button>
      )}

      {showRemoveBot && (
        <div className="absolute top-2 right-2 z-10 flex gap-1 opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              onRemoveBot?.();
            }}
            aria-label={t("room.removeBotIconLabel", { name: displayName })}
            title={t("room.removeBotIconLabel", { name: displayName })}
            data-testid={`remove-bot-${seatIndex}`}
            className="bg-surface-elevated text-danger border-danger/30 hover:border-danger/60 inline-flex size-7 items-center justify-center rounded-md border disabled:opacity-40"
          >
            <UserX className="size-3.5" />
          </button>
        </div>
      )}

      {showOwnerControls && player && (
        <div className="absolute top-2 right-2 z-10 flex gap-1 opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
          {onPromote && (
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                onPromote();
              }}
              aria-label={t("room.promoteIconLabel", { username: player.username })}
              title={t("room.promoteIconLabel", { username: player.username })}
              data-testid={`promote-seat-${seatIndex}`}
              className="bg-surface-elevated border-border text-brass-deep inline-flex size-7 items-center justify-center rounded-md border hover:border-brass disabled:opacity-40"
            >
              <Crown className="size-3.5" />
            </button>
          )}
          {onKick && (
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                onKick();
              }}
              aria-label={t("room.kickIconLabel", { username: player.username })}
              title={t("room.kickIconLabel", { username: player.username })}
              data-testid={`kick-player-${seatIndex}`}
              className="bg-surface-elevated text-danger border-danger/30 hover:border-danger/60 inline-flex size-7 items-center justify-center rounded-md border disabled:opacity-40"
            >
              <UserX className="size-3.5" />
            </button>
          )}
        </div>
      )}

      {showSwapHint && (
        <div className="pointer-events-none absolute inset-0 z-20 flex items-center justify-center opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
          <Badge tone="accent" icon={<Shuffle />} className="bg-surface shadow-sm">
            {t("room.seatTile.swapHere")}
          </Badge>
        </div>
      )}
    </div>
  );
}

type ModeTokens = { fill: string; edge: string; edgeSoft: string; text: string };

const MODE_TOKENS: Record<SeatMode, ModeTokens> = {
  us: {
    fill: "var(--team-a-tint)",
    edge: "var(--team-a-edge)",
    edgeSoft: "var(--team-a-edge-soft)",
    text: "var(--team-a)",
  },
  them: {
    fill: "var(--team-b-tint)",
    edge: "var(--team-b-edge)",
    edgeSoft: "var(--team-b-edge-soft)",
    text: "var(--team-b)",
  },
  neutral: {
    fill: "var(--surface)",
    edge: "var(--brass)",
    edgeSoft: "var(--border)",
    text: "var(--brass-deep)",
  },
};
