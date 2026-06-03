import { useTranslation } from "react-i18next";

import { MOTION } from "@/shared/lib/motion";
import type { PlayerState, Suit } from "@/shared/types/matchTypes";

import { type SeatTeam, teamColors, teamLabelKey } from "../lib/tableTheme";
import { isCountdownUrgent, useTurnCountdown } from "../lib/turnCountdown";
import { TimerRing } from "./TimerRing";

export type SeatOrientation = "bottom" | "left" | "top" | "right";

interface PlayerSeatProps {
  player: PlayerState | null;
  isSelf: boolean;
  isActive: boolean;
  /**
   * Viewer-relative team identity. The viewer's own team always reads as
   * `gold` regardless of teamA/teamB. See [tableTheme.seatTeam].
   */
  seatTeam: SeatTeam;
  cardCount?: number;
  turnExpiresAt?: string | null;
  timerDuration?: number;
  /** This seat is the dealer for the current hand. */
  isDealer?: boolean;
  /**
   * When this seat is the trump caller, the trump suit. When `null` /
   * `undefined`, no caller chip is rendered. (The chip carries the suit
   * glyph, so the suit must be known.)
   */
  trumpCallerSuit?: Suit | null;
  /**
   * Layout for the seat's children. `bottom`/`top` stack vertically; `left`/
   * `right` flow horizontally so the avatar sits on the screen edge while
   * name + card-back stack flow inward.
   */
  orientation?: SeatOrientation;
  /**
   * Phone layout: every seat stacks vertically (avatar → name → card-backs),
   * the avatar shrinks, and the name pill drops the team-label / seconds row
   * (the timer ring around the avatar carries that). Dealer/trump chips keep
   * their size. Defaults to the desktop layout.
   */
  compact?: boolean;
}

const SUIT_GLYPH: Record<Suit, string> = {
  S: "♠",
  H: "♥",
  D: "♦",
  C: "♣",
};

const SUIT_NAME: Record<Suit, string> = {
  S: "Spades",
  H: "Hearts",
  D: "Diamonds",
  C: "Clubs",
};

function suitColorHex(suit: Suit): string {
  return suit === "H" || suit === "D" ? "#c62828" : "#1a1a1a";
}

const AVATAR_DISC_PX = 64;
const AVATAR_DISC_PX_COMPACT = 44;
const FRAME_PAD_PX = 16; // 3 px conic frame + 5 px ring lane (both sides)

const TURN_LIME = "var(--turn-lime, #00e5a0)";
const COLOR_URGENT = "var(--turn-urgent, #ef4444)";
// Hard-coded RGBA so the active glow keeps its alpha — `var()` cannot be
// concatenated with extra hex digits (the CSS parser drops the whole
// declaration on `var(--x, #fff)99`, which had been silently disabling
// the active-seat outer ring).
const TURN_LIME_GLOW = "rgba(0, 229, 160, 0.6)";

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
        background: "linear-gradient(180deg, #fdfaf0, #f0e8d0)",
        color: textColor,
        border: `2px solid ${borderColor}`,
        boxShadow: glowColor
          ? `0 2px 10px rgba(0,0,0,0.55), 0 0 12px ${glowColor}, inset 0 1px 0 rgba(255,255,255,0.6)`
          : "0 2px 10px rgba(0,0,0,0.55), inset 0 1px 0 rgba(255,255,255,0.6)",
        fontFamily: "var(--font-suit)",
        fontWeight: 700,
        lineHeight: 1,
      }}
    >
      {children}
    </div>
  );
}

/** Fanned face-down card-back pile shown next to non-self avatars, with a
 *  trailing `×N` count so a glance reveals exactly how many cards the seat
 *  still holds (5 in trump-bidding, 8 once dealing finishes, then drops as
 *  cards are played).
 *
 *  Doubles as the geometric anchor for the CardFlight overlay — opponent
 *  throws launch from this rect and trick-collect flights converge here when
 *  this seat wins. The outer `data-testid="player-seat-deck-{seat}"` exposes
 *  it for `getBoundingClientRect()` lookups. */
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
  // Extra-small card backs on phones so the seat stays tight.
  const cw = compact ? 15 : 26;
  const ch = compact ? 22 : 38;
  const overlap = compact ? 12 : 22;
  // Render the wrapper div even when count === 0 so the CardFlight overlay
  // can always measure this seat's deck rect (used as the destination on a
  // collect flight that converges on this seat as winner). Without this,
  // the last trick of a hand — where every player has zero cards left —
  // would have no anchor and the collect animation would silently bail.
  return (
    <div
      className="flex items-center"
      style={{
        marginLeft: isHorizontal ? 8 : 0,
        marginTop: isHorizontal ? 0 : compact ? 2 : 4,
        // Reserve enough box for the card-back fan + count badge so the rect
        // measurement returns a non-zero size even with no cards visible.
        minWidth: compact ? 34 : 56,
        minHeight: compact ? 22 : 38,
        // When there are no cards to show, hide the children visually but
        // keep the box (and its rect) addressable.
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
            width: cw,
            height: ch,
            marginLeft: i === 0 ? 0 : -overlap,
            transform: `rotate(${(i - 2) * 1.5}deg)`,
            background: "linear-gradient(135deg, #2a1a10 0%, #4a2818 50%, #2a1a10 100%)",
            border: `${compact ? 1 : 1.5}px solid var(--brass, #c9a876)`,
            borderRadius: compact ? 3 : 4,
            boxShadow: "0 2px 6px rgba(0,0,0,0.45)",
          }}
        />
      ))}
      <span
        className={`font-body tabular-nums ${compact ? "text-[9px]" : "text-[11px]"}`}
        style={{
          marginLeft: compact ? 4 : 8,
          color: "var(--ink-light, #f5f2e8)",
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
 * Player seat — the avatar + name pill + status chips + (for opponents) a
 * card-back stack representing the cards they still hold.
 *
 * Three visual channels stack here:
 *  • Team identity — the conic-gradient frame around the avatar disc (Gold
 *    for the viewer's team, Silver for the opposition; viewer-relative).
 *  • Turn signal — the lime ring + countdown number that appear when this
 *    seat is the active player (urgent-red flip handled by [TimerRing]).
 *  • Hand state — dealer (D) + trump-caller (suit) chips, stacked.
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

  const discPx = compact ? AVATAR_DISC_PX_COMPACT : AVATAR_DISC_PX;
  const framePx = discPx + FRAME_PAD_PX;

  // Hook must run before the empty-seat early return — the seconds drive the
  // external label rendered inside the name pill (next to the team label).
  // Returns 0 when there's no expiry, so an empty/inactive seat costs nothing.
  const secondsLeft = useTurnCountdown(turnExpiresAt ?? null);

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
            background: "rgba(255,255,255,0.04)",
            border: "1px dashed rgba(255,255,255,0.18)",
          }}
        >
          <span className="text-text-secondary text-xs font-body">?</span>
        </div>
        <span className="text-text-secondary text-sm font-body">{t("match.seat.waiting")}</span>
      </div>
    );
  }

  const isDisconnected = player.connected === false;
  // On phones every seat stacks vertically (avatar on top); only desktop keeps
  // the horizontal left/right edge layout.
  const isHorizontal = !compact && (orientation === "left" || orientation === "right");

  const flexDirection: React.CSSProperties["flexDirection"] = isHorizontal
    ? orientation === "right"
      ? "row-reverse"
      : "row"
    : "column";

  const statusLabel = isDisconnected ? "disconnected" : isActive ? "active" : "waiting";
  const displayName = isSelf ? t("match.seat.you") : player.username || `P${player.seat + 1}`;
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
      borderColor={suitColorHex(trumpCallerSuit)}
      textColor={suitColorHex(trumpCallerSuit)}
      glowColor={`${suitColorHex(trumpCallerSuit)}99`}
      title={`Called ${SUIT_NAME[trumpCallerSuit]}`}
      testId="player-seat-caller-chip"
    >
      <span style={{ fontSize: 18, lineHeight: 1 }}>{SUIT_GLYPH[trumpCallerSuit]}</span>
    </StatusChip>
  ) : null;

  const dealerChipNode = isDealer ? (
    <StatusChip
      borderColor="#1f2e23"
      textColor="#0e3a24"
      title="Dealer this hand"
      testId="player-seat-dealer-chip"
    >
      <span style={{ fontSize: 16 }}>D</span>
    </StatusChip>
  ) : null;

  // Chip anchoring. Both chips share the same `top`, so the pair reads as one
  // chip layered over the other (caller wins z-order). The front/back `right`
  // values differ by 10 px → the two 30 px chips overlap by 20 px, the same
  // staggered "stack" the desktop layout uses. On phones the whole pair tucks
  // inward (front at the avatar's right edge, back 10 px behind-left) so the
  // rightmost chip can't clip off-screen on an edge-hugging seat — instead of
  // fanning the dealer far inward, which left them sitting side-by-side.
  const chipTop = compact ? -4 : -6;
  const chipRightLone = compact ? -2 : -6;
  const chipRightFront = compact ? -2 : -16; // caller (top of z-stack)
  const chipRightBack = compact ? 8 : -6; // dealer (behind, 10px left of caller)

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
      {/* Avatar w/ team-color conic frame + (when active) lime/red countdown ring */}
      <div className="relative shrink-0" style={{ width: framePx, height: framePx }}>
        <div
          className="rounded-full"
          style={{
            position: "absolute",
            inset: 0,
            background: `conic-gradient(from 0deg, ${teamGradient[0]}, ${teamGradient[1]}, ${teamGradient[0]})`,
            padding: 3,
            boxShadow:
              isActive && !isDisconnected
                ? `0 0 0 1.5px ${TURN_LIME}, 0 0 18px ${TURN_LIME_GLOW}`
                : "0 2px 8px rgba(0,0,0,0.3)",
            transition: `box-shadow ${MOTION.SEAT_RING_TRANSITION}ms ease`,
          }}
        >
          <div
            className="rounded-full flex items-center justify-center font-display font-semibold"
            style={{
              width: "100%",
              height: "100%",
              background: "var(--avatar-inner, #1f2e23)",
              color: "var(--ink-light, #f5f2e8)",
              fontSize: discPx * 0.5,
            }}
            data-testid="player-seat-avatar"
          >
            {initial}
          </div>
        </div>
        {/* Countdown ring overlay (lime → urgent red ≤1/8). The numeric label
            is suppressed here so the avatar initial stays legible — the
            seconds are rendered in the name pill team-label row instead. */}
        {showRing && (
          <TimerRing
            turnExpiresAt={turnExpiresAt ?? null}
            totalDuration={timerDuration ?? 0}
            secondsLeft={secondsLeft}
            size={compact ? "seatCompact" : "seat"}
            hideLabel
          />
        )}
        {/* Stackable status chips: caller occupies the left slot, dealer the
            right (positions kept as before). The caller chip wins the z-order
            so its suit glyph hovers above the dealer chip in the overlap
            region — the suit is the more important piece of information once
            trump has been taken. */}
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

      {/* Name pill. On phones it's name-only and tight — the team label and
          seconds counter are dropped (the avatar's timer ring covers timing). */}
      <div
        className={`rounded-lg flex flex-col items-center ${compact ? "px-2 py-0.5" : "gap-0.5 px-3 py-1.5"}`}
        style={{
          minWidth: compact ? undefined : 88,
          background: "var(--panel-dark, rgba(20,45,30,0.85))",
          border:
            isActive && !isDisconnected
              ? `1px solid ${TURN_LIME}`
              : "1px solid rgba(255,255,255,0.08)",
          boxShadow: isActive && !isDisconnected ? "0 0 12px rgba(0, 229, 160, 0.33)" : undefined,
          transition: `border-color ${MOTION.SEAT_RING_TRANSITION}ms ease, box-shadow ${MOTION.SEAT_RING_TRANSITION}ms ease`,
        }}
        data-testid="player-seat-name-pill"
      >
        <span
          className={`font-body font-semibold ${compact ? "text-[11px]" : "text-sm"}`}
          style={{ color: "var(--ink-light, #f5f2e8)" }}
        >
          {displayName}
        </span>
        {!compact && (
          <span
            className="font-body text-[10px] tabular-nums flex items-center gap-1.5"
            style={{ color: "rgba(245,242,232,0.6)" }}
          >
            <span
              className="rounded-full"
              style={{
                width: 5,
                height: 5,
                background: teamGradient[0],
              }}
              aria-hidden
            />
            <span style={{ letterSpacing: 0.6, opacity: 0.7 }}>{teamLabel}</span>
            {showRing && (
              <span
                className="font-body font-semibold tabular-nums"
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

      {/* Card-back stack — opponents only. Rendered even when `cardCount === 0`
          (last trick of a hand) so the CardFlight overlay can always measure
          this seat's deck rect; the inner stack hides itself when empty. */}
      {!isSelf && cardCount !== undefined && (
        <CardBackStack
          count={cardCount}
          isHorizontal={isHorizontal}
          seat={player.seat}
          compact={compact}
        />
      )}

      {/* sr-live region for assistive tech — same wording as before so existing
          screen-reader expectations don't drift. */}
      <div aria-live="polite" className="sr-only">
        {isActive ? "It's this player's turn" : ""}
      </div>
    </div>
  );
}
