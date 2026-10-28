import { Bot } from "lucide-react";
import { useTranslation } from "react-i18next";

import { AvatarImage } from "@/shared/components/ui/avatar";
import { botDisplayName } from "@/shared/lib/botName";
import { MOTION } from "@/shared/lib/motion";
import { useAuthStore } from "@/shared/stores/authStore";
import type { PlayerState, Suit } from "@/shared/types/matchTypes";

import {
  CARD_BACK_BACKGROUND,
  cardBox,
  resolveCardDeck,
  SEAT_STACK_CARD_WIDTH,
} from "../lib/cardFace";
import {
  SEAT_BORDER_PX,
  SEAT_DISC_PX,
  SEAT_DISC_PX_COMPACT,
  seatFramePx,
} from "../lib/seatGeometry";
import { suitAccent, suitNameKey } from "../lib/suitArt";
import { type SeatTeam, teamColors, teamLabelKey } from "../lib/tableTheme";
import { isCountdownUrgent, useTurnCountdown } from "../lib/turnCountdown";
import { CardBackFace } from "./CardBackFace";
import { SuitMark } from "./SuitMark";
import { TimerRing } from "./TimerRing";

export type SeatOrientation = "bottom" | "left" | "top" | "right";

interface PlayerSeatProps {
  player: PlayerState | null;
  isSelf: boolean;
  isActive: boolean;
  seatTeam: SeatTeam;
  cardCount?: number;
  turnExpiresAt?: string | null;
  timerDuration?: number;
  isDealer?: boolean;
  trumpCallerSuit?: Suit | null;
  orientation?: SeatOrientation;
  compact?: boolean;
}

const TURN_LIME = "var(--accent-light)";
const COLOR_URGENT = "var(--danger)";
const TURN_LIME_GLOW = "color-mix(in srgb, var(--accent-light) 60%, transparent)";

/**
 * Status indicator chip — 30 px circle with a thick felt-toned border so it
 * reads against the avatar frame. Used by the dealer + trump-caller stack on
 * each seat.
 */
function StatusChip({
  borderColor,
  textColor,
  glowColor,
  children,
  title,
  testId,
}: {
  borderColor: string;
  textColor: string;
  glowColor?: string;
  children: React.ReactNode;
  title: string;
  testId: string;
}) {
  return (
    <div
      title={title}
      data-testid={testId}
      className="rounded-full flex items-center justify-center"
      style={{
        width: 30,
        height: 30,
        background: "linear-gradient(180deg, var(--card-face), var(--card-face-shade))",
        color: textColor,
        border: `2px solid ${borderColor}`,
        boxShadow: glowColor
          ? `var(--shadow-felt-sm), 0 0 12px ${glowColor}, inset 0 1px 0 var(--sheen-70)`
          : "var(--shadow-felt-sm), inset 0 1px 0 var(--sheen-70)",
        fontFamily: "var(--font-suit)",
        fontWeight: 700,
        lineHeight: 1,
      }}
    >
      {children}
    </div>
  );
}

/**
 * Fanned card-back pile with a `×N` count, shown next to opponent avatars.
 * It is also the CardFlight anchor, so it keeps its box even when empty.
 */
function CardBackStack({
  count,
  isHorizontal,
  seat,
  compact = false,
}: {
  count: number;
  isHorizontal: boolean;
  seat: number;
  compact?: boolean;
}) {
  const visible = Math.min(count, 5);
  const cw = compact ? SEAT_STACK_CARD_WIDTH.compact : SEAT_STACK_CARD_WIDTH.desktop;
  const ch = cardBox(cw).height;
  const overlap = compact ? 16 : 26;
  const radius = compact ? 2 : 4;
  return (
    <div
      className="flex items-center"
      style={{
        marginLeft: isHorizontal ? 8 : 0,
        marginTop: isHorizontal ? 0 : compact ? 2 : 4,
        minWidth: compact ? 36 : 56,
        minHeight: ch,
        visibility: visible > 0 ? "visible" : "hidden",
      }}
      data-testid="player-seat-card-stack"
      data-seat-deck={seat}
      id={`player-seat-deck-${seat}`}
    >
      {Array.from({ length: visible }).map((_, i) => (
        <div
          key={i}
          aria-hidden
          style={{
            position: "relative",
            overflow: "hidden",
            width: cw,
            height: ch,
            marginLeft: i === 0 ? 0 : -overlap,
            transform: `rotate(${(i - 2) * 1.5}deg)`,
            background: CARD_BACK_BACKGROUND,
            border: "1px solid var(--brass)",
            borderRadius: radius,
            boxShadow: "var(--shadow-felt-sm)",
          }}
        >
          <CardBackFace width={cw} radius={radius} />
        </div>
      ))}
      <span
        className={`tabular-nums ${compact ? "text-2xs" : "text-xs"}`}
        style={{
          marginLeft: compact ? 4 : 8,
          color: "var(--ink-light)",
          opacity: 0.7,
        }}
        data-testid="player-seat-card-count"
        aria-label={`${count} cards remaining`}
      >
        ×{count}
      </span>
    </div>
  );
}

/**
 * Player seat: avatar with team frame and turn ring, name pill, dealer and trump chips, and opponent card backs.
 */
export function PlayerSeat({
  player,
  isSelf,
  isActive,
  seatTeam,
  cardCount,
  turnExpiresAt,
  timerDuration,
  isDealer = false,
  trumpCallerSuit = null,
  orientation = "bottom",
  compact = false,
}: PlayerSeatProps) {
  const { t } = useTranslation();

  const teamLabel = t(teamLabelKey(seatTeam));
  const teamGradient = teamColors(seatTeam);

  const discPx = compact ? SEAT_DISC_PX_COMPACT : SEAT_DISC_PX;
  const framePx = seatFramePx(compact);

  const secondsLeft = useTurnCountdown(turnExpiresAt ?? null);
  const deck = resolveCardDeck(useAuthStore((s) => s.user?.cardDeckPreference));

  if (!player) {
    return (
      <div
        className="flex flex-col items-center gap-2"
        aria-label={`Empty seat, ${teamLabel}, waiting`}
        data-testid="player-seat-empty"
        data-seat-team={seatTeam}
      >
        <div
          className="rounded-full flex items-center justify-center"
          style={{
            width: framePx,
            height: framePx,
            background: "var(--sheen-6)",
            border: "1px dashed var(--sheen-15)",
          }}
        >
          <span className="text-ink-dim text-xs">?</span>
        </div>
        <span className="text-ink-dim text-sm">{t("match.seat.waiting")}</span>
      </div>
    );
  }

  const isDisconnected = player.connected === false && player.isBot !== true;
  const isHorizontal = !compact && (orientation === "left" || orientation === "right");

  const flexDirection: React.CSSProperties["flexDirection"] = isHorizontal
    ? orientation === "right"
      ? "row-reverse"
      : "row"
    : "column";

  const statusLabel = isDisconnected ? "disconnected" : isActive ? "active" : "waiting";
  const displayName = isSelf
    ? t("match.seat.you")
    : player.isBot === true
      ? botDisplayName(t, player.seat)
      : player.username || `P${player.seat + 1}`;
  const initial = (player.username || "?").charAt(0).toUpperCase();

  const showRing =
    isActive &&
    !isDisconnected &&
    Boolean(turnExpiresAt) &&
    timerDuration !== undefined &&
    timerDuration > 0;

  const secondsUrgent = showRing && isCountdownUrgent(secondsLeft, timerDuration ?? 0);

  const callerChipNode = trumpCallerSuit ? (
    <StatusChip
      borderColor={suitAccent(trumpCallerSuit, deck)}
      textColor={suitAccent(trumpCallerSuit, deck)}
      glowColor={`${suitAccent(trumpCallerSuit, deck)}99`}
      title={t("match.seat.calledSuit", { suit: t(suitNameKey(trumpCallerSuit, deck)) })}
      testId="player-seat-caller-chip"
    >
      <SuitMark suit={trumpCallerSuit} deck={deck} size={20} />
    </StatusChip>
  ) : null;

  const dealerChipNode = isDealer ? (
    <StatusChip
      borderColor="var(--avatar-inner)"
      textColor="var(--ink-on-card)"
      title="Dealer this hand"
      testId="player-seat-dealer-chip"
    >
      <span style={{ fontSize: 16 }}>D</span>
    </StatusChip>
  ) : null;

  const chipTop = compact ? -4 : -6;
  const chipRightLone = compact ? -2 : -6;
  const chipRightFront = compact ? -2 : -16;
  const chipRightBack = compact ? 8 : -6;

  return (
    <div
      className={`inline-flex items-center select-none ${compact ? "gap-1" : "gap-2"}`}
      style={{
        flexDirection,
        opacity: isDisconnected ? 0.5 : 1,
        filter: isDisconnected ? "grayscale(1)" : undefined,
        transform: isSelf && !compact ? "scale(1.08)" : undefined,
      }}
      aria-label={`${displayName}, ${teamLabel}, ${statusLabel}`}
      data-testid={`player-seat-${player.seat}`}
      data-seat-team={seatTeam}
      data-active={isActive ? "true" : "false"}
      data-self={isSelf ? "true" : undefined}
    >
      <div className="relative shrink-0" style={{ width: framePx, height: framePx }}>
        <div
          className="rounded-full"
          style={{
            position: "absolute",
            inset: 0,
            background: `conic-gradient(from 0deg, ${teamGradient[0]}, ${teamGradient[1]}, ${teamGradient[0]})`,
            padding: SEAT_BORDER_PX,
            boxShadow:
              isActive && !isDisconnected
                ? `0 0 0 2px ${TURN_LIME}, 0 0 24px ${TURN_LIME_GLOW}`
                : "var(--shadow-felt-sm)",
            transition: `box-shadow ${MOTION.SEAT_RING_TRANSITION}ms ease`,
          }}
        >
          <div
            className="rounded-full flex items-center justify-center overflow-hidden font-semibold"
            style={{
              width: "100%",
              height: "100%",
              background: "var(--avatar-inner)",
              color: "var(--ink-light)",
              fontSize: discPx * 0.5,
            }}
            data-testid="player-seat-avatar"
          >
            {player.isBot === true ? (
              <Bot
                size={discPx * 0.5}
                aria-hidden="true"
                data-testid="avatar-icon"
                style={{ transform: "translateY(-5%)" }}
              />
            ) : (
              <AvatarImage src={player.avatarUrl} size={discPx} fallback={initial} />
            )}
          </div>
        </div>
        {showRing && (
          <TimerRing
            turnExpiresAt={turnExpiresAt ?? null}
            totalDuration={timerDuration ?? 0}
            secondsLeft={secondsLeft}
            size={compact ? "seatCompact" : "seat"}
            hideLabel
          />
        )}
        {callerChipNode && dealerChipNode && (
          <>
            <div className="absolute" style={{ top: chipTop, right: chipRightFront, zIndex: 2 }}>
              {callerChipNode}
            </div>
            <div className="absolute" style={{ top: chipTop, right: chipRightBack, zIndex: 1 }}>
              {dealerChipNode}
            </div>
          </>
        )}
        {!callerChipNode && dealerChipNode && (
          <div className="absolute" style={{ top: chipTop, right: chipRightLone }}>
            {dealerChipNode}
          </div>
        )}
        {callerChipNode && !dealerChipNode && (
          <div className="absolute" style={{ top: chipTop, right: chipRightLone }}>
            {callerChipNode}
          </div>
        )}
      </div>

      <div
        className={`rounded-lg flex flex-col items-center ${compact ? "px-2 py-0.5" : "gap-0.5 px-3 py-1.5"}`}
        style={{
          minWidth: compact ? undefined : 88,
          background: "var(--panel-dark)",
          border:
            isActive && !isDisconnected ? `1px solid ${TURN_LIME}` : "1px solid var(--sheen-10)",
          boxShadow:
            isActive && !isDisconnected
              ? "0 0 12px color-mix(in srgb, var(--accent) 33%, transparent)"
              : undefined,
          transition: `border-color ${MOTION.SEAT_RING_TRANSITION}ms ease, box-shadow ${MOTION.SEAT_RING_TRANSITION}ms ease`,
        }}
        data-testid="player-seat-name-pill"
      >
        <span
          className={`font-semibold ${compact ? "text-xs" : "text-sm"}`}
          style={{ color: "var(--ink-light)" }}
        >
          {displayName}
        </span>
        {!compact && player.isBot !== true && (
          <span
            className="text-2xs tabular-nums flex items-center gap-1.5"
            style={{ color: "var(--ink-mute)" }}
          >
            <span style={{ letterSpacing: "0.05em" }} data-testid="player-seat-level">
              {t("xp.short", { level: player.level })}
            </span>
            {showRing && (
              <span
                className="font-semibold tabular-nums"
                style={{
                  color: secondsUrgent ? COLOR_URGENT : TURN_LIME,
                  transition: `color ${MOTION.RING_COLOR_FLIP}ms ease`,
                  marginLeft: 4,
                }}
                data-testid="player-seat-timer-seconds"
                aria-label={`${secondsLeft} seconds remaining`}
              >
                {secondsLeft}s
              </span>
            )}
          </span>
        )}
      </div>

      {!isSelf && cardCount !== undefined && (
        <CardBackStack
          count={cardCount}
          isHorizontal={isHorizontal}
          seat={player.seat}
          compact={compact}
        />
      )}

      <div aria-live="polite" className="sr-only">
        {isActive ? "It's this player's turn" : ""}
      </div>
    </div>
  );
}
