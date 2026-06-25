import { Bot, Crown, Hourglass, Shuffle, UserX } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Avatar } from "@/shared/components/ui/avatar";
import { Badge } from "@/shared/components/ui/badge";
import { useMediaQuery } from "@/shared/hooks/useMediaQuery";
import { botDisplayName } from "@/shared/lib/botName";
import { cn } from "@/shared/lib/utils";
import type { RoomPlayer } from "@/shared/types/apiTypes";

type SeatMode = "us" | "them" | "neutral";

// Diamond placement — only at sm+, where the parent grid defines the matching
// `grid-template-areas`. Literal class strings so Tailwind's scanner emits them.
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
  /**
   * Reopened-room presence (v2): this seat's human is still away (on the prior
   * match's result dialog) and has not returned/rejoined yet. Shows a "waiting
   * to return" badge; the owner Start button is gated until none remain.
   */
  waitingToReturn?: boolean;
  ownerCanActOnRow: boolean;
  onSelect: () => void;
  onKick?: () => void;
  onPromote?: () => void;
  /** Owner-only: seat a bot on this EMPTY seat (custom waiting rooms only). */
  onAddBot?: () => void;
  /** Owner-only: open the remove-bot confirmation for this bot seat. */
  onRemoveBot?: () => void;
};

/**
 * Per-seat tile in the in-room diamond. Empty seats show a dashed-border pulse
 * dot + "Take this seat" / "Move here" copy; filled seats show an Avatar +
 * name + badges (You / Host / Partner / Opponent / Swap target) + owner-only
 * promote and kick icon buttons in the top corners.
 *
 * Visual mode is viewer-relative: when the viewer is unseated the tile renders
 * neutral parchment, once seated the same-parity tiles become "us" (team A
 * tint, gold edge) and the other parity become "them" (team B tint, silver
 * edge). RoomPage resolves the mode and passes it in; the tile only paints.
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
  ownerCanActOnRow,
  onSelect,
  onKick,
  onPromote,
  onAddBot,
  onRemoveBot,
}: SeatTileProps) {
  const { t } = useTranslation();
  // Size the seated avatar to match the empty-seat placeholder circle
  // (size-9 / sm:size-10) so a filled seat doesn't bulge larger than an empty
  // one. Compact = below the sm breakpoint where the layout stacks.
  const isCompact = useMediaQuery("(max-width: 639px)");
  const avatarSize = isCompact ? 36 : 40;
  const filled = Boolean(player);
  const isBot = player?.isBot === true;
  // Bot identity is seat-derived and rendered client-side — an empty wire
  // username must never leak through as a blank.
  const displayName = isBot ? botDisplayName(t, seatIndex) : (player?.username ?? "");
  // In swap mode the kick/promote overlay steps aside — every other filled
  // seat instead reads as a swap candidate (Shuffle hint on hover), since a
  // click there fires the swap rather than opening owner controls.
  const showOwnerControls = ownerCanActOnRow && filled && !isYou && !swapMode;
  const showRemoveBot = Boolean(onRemoveBot) && isBot && !swapMode;
  const showAddBot = Boolean(onAddBot) && !filled && !swapMode;
  const showSwapHint = swapMode && filled && !isSwapSource && !isYou;

  // Inline styles for the parchment tokens — Tailwind arbitrary values would
  // need 6 distinct classes per tile and the design's color tuples (tint,
  // edge, edge-soft) are easier to read as a map.
  const tokens = MODE_TOKENS[mode];
  const tileStyle: React.CSSProperties = {
    background: filled ? tokens.fill : "transparent",
    border: filled ? `2px solid ${tokens.edge}` : `2px dashed ${tokens.edgeSoft}`,
    boxShadow: isSwapSource
      ? `0 0 0 3px var(--accent-soft), 0 0 0 1.5px var(--accent)`
      : isYou
        ? `0 0 0 1.5px var(--accent), 0 8px 22px -16px var(--accent)`
        : filled
          ? "0 4px 14px -12px rgba(14,58,36,0.20)"
          : "none",
  };

  return (
    <div
      // grid-area is applied only at sm+, where the parent defines the diamond
      // `grid-template-areas`. On phones the parent is a plain 2-col grid with
      // NO named areas — an unconditional `gridArea: "south"` there matches no
      // area and collapses all four tiles into one cell (they overlapped). With
      // it scoped to sm, mobile tiles auto-flow into the 2×2 grid.
      className={cn("group relative", SEAT_AREA_CLASS[cardinal])}
      data-testid={`seat-position-${cardinal}`}
      data-team={seatIndex % 2 === 0 ? "teamA" : "teamB"}
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
              size={avatarSize}
              team={mode === "us" ? "A" : mode === "them" ? "B" : null}
              you={isYou}
              owner={isHost}
              icon={isBot ? <Bot aria-hidden="true" /> : undefined}
            />
            <div className="flex flex-col items-center gap-1">
              <span
                className="font-display text-ink inline-flex items-center gap-1 text-[13px] font-semibold tracking-[-0.2px] sm:text-[14.5px]"
                data-testid={isBot ? `bot-name-${seatIndex}` : undefined}
              >
                {isHost && <Crown className="text-brass-deep size-3.25" aria-hidden="true" />}
                {displayName}
              </span>
              <div className="flex flex-wrap items-center justify-center gap-1">
                {/* Bots never show You/Host badges — the bot marker takes their place. */}
                {isBot && (
                  <span data-testid={`bot-badge-${seatIndex}`}>
                    <Badge tone="neutral" icon={<Bot className="size-2.5" aria-hidden="true" />}>
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
              className="bg-surface-elevated mt-1 flex size-9 items-center justify-center rounded-full sm:mt-1.5 sm:size-10"
              style={{ border: `1.5px dashed ${tokens.edgeSoft}` }}
            >
              <span
                className="size-2.5 rounded-full opacity-45 animate-[pulse-dot_1.6s_ease-in-out_infinite]"
                style={{ background: tokens.text }}
              />
            </div>
            <div className="text-center">
              <div className="font-display text-ink-dim text-[12px] font-semibold sm:text-[13.5px]">
                {swapMode ? t("room.seatTile.moveHere") : t("room.seatTile.takeSeat")}
              </div>
              <div className="text-ink-mute mt-0.5 text-[10.5px] sm:text-[11px]">
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

      {/* Owner-only "add a bot" affordance on empty seats — sits beside the
          regular "take this seat" click target so both remain reachable. */}
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
          // Larger tap target on phones (was size-6.5): too small to hit
          // reliably, so taps fell through to the seat's take-seat handler.
          // Desktop keeps the compact size.
          className="bg-surface-elevated border-border text-ink-dim hover:border-brass hover:text-brass-deep absolute top-2 right-2 z-10 inline-flex size-9 items-center justify-center rounded-md border disabled:opacity-40 md:size-6.5"
        >
          {/* Nudged up 5%: the lucide Bot glyph's visual mass (the robot
              head) sits low in its viewBox, so true geometric centering
              reads as off-center. Same rule as the avatar bot glyphs. */}
          <Bot className="size-4.5 -translate-y-[5%] md:size-3.5" />
        </button>
      )}

      {/* Owner-only remove control on bot seats (confirm dialog, kick pattern). */}
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
            className="bg-surface-elevated text-destructive border-destructive/30 hover:border-destructive/60 inline-flex size-6.5 items-center justify-center rounded-md border disabled:opacity-40"
          >
            <UserX className="size-3.25" />
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
              className="bg-surface-elevated border-border text-brass-deep inline-flex size-6.5 items-center justify-center rounded-md border hover:border-brass disabled:opacity-40"
            >
              <Crown className="size-3.25" />
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
              className="bg-surface-elevated text-destructive border-destructive/30 hover:border-destructive/60 inline-flex size-6.5 items-center justify-center rounded-md border disabled:opacity-40"
            >
              <UserX className="size-3.25" />
            </button>
          )}
        </div>
      )}

      {showSwapHint && (
        <div className="pointer-events-none absolute inset-0 z-20 flex items-center justify-center opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
          <span className="border-accent bg-surface text-accent-deep inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-semibold shadow-[0_4px_12px_-4px_rgba(14,58,36,0.35)]">
            <Shuffle className="size-2.5" style={{ color: "var(--accent-deep)" }} />
            {t("room.seatTile.swapHere")}
          </span>
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
    edge: "var(--border-2)",
    edgeSoft: "var(--border)",
    text: "var(--brass-deep)",
  },
};
