import { useEffect } from "react";
import { useTranslation } from "react-i18next";

import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { MOTION } from "@/shared/lib/motion";
import { Z } from "@/shared/lib/zLayers";
import type { TeamString } from "@/shared/types/matchTypes";
import type { MatchAbandonedPayload } from "@/shared/types/wsEvents";

import { TEAM_GOLD, TEAM_SILVER, type TeamGradient } from "../lib/tableTheme";
import {
  isCountdownUrgent,
  URGENT_FRACTION,
  useRingDrain,
  useTurnCountdown,
} from "../lib/turnCountdown";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";

interface DisconnectedPlayerInfo {
  /** Display name for the chip. */
  name: string;
  /** RFC3339 absolute timestamp when this seat's individual reconnect window
   *  closes. Drives the per-row countdown shown next to the chip. */
  expiresAt: string;
}

interface ReconnectOverlayProps {
  /** Name of the abandoned-state title player. Used only when `abandonedData`
   *  is supplied. For the countdown view, `disconnectedPlayers` drives the
   *  chip text — set this to the seat that's about to abandon (the earliest
   *  expiry) when you have it, otherwise any disconnected name is fine. */
  disconnectedPlayerName: string;
  /** Per-seat disconnect info — one entry per offline player, each with its
   *  own expiry. The center ring counts down to the *earliest* expiry; each
   *  player row counts down to its own. When omitted, the overlay falls back
   *  to a single chip driven by `disconnectedPlayerName` + `reconnectExpiresAt`. */
  disconnectedPlayers?: DisconnectedPlayerInfo[];
  /** Earliest expiry among all offline seats (typically a duplicate of the
   *  minimum of `disconnectedPlayers[*].expiresAt`). Drives the center ring.
   *  Required because some call sites still source it from `matchState.reconnectExpiresAt`
   *  (the server's legacy single-pointer derivation). */
  reconnectExpiresAt: string;
  abandonedData?: MatchAbandonedPayload | null;
  // Viewer team — required whenever `abandonedData` is supplied so the
  // final-score columns can render Us/Them. When the overlay shows only the
  // reconnect countdown (no abandonedData), this prop is unused and may be
  // omitted.
  viewerTeam?: TeamString | null;
  onReturnToLobby?: () => void;
  /** Total reconnect window in seconds — used to drive the progress ring's
   *  sweep. Defaults to 120 (server default). Custom rooms with a different
   *  window can pass it explicitly so the ring still reads "fills full → empty"
   *  rather than starting partially-full. */
  totalSeconds?: number;
}

const RECONNECT_TOTAL_SECONDS_DEFAULT = 120;
// Color tokens lifted from the design's classic-states.jsx countdown rings.
const RING_PARCHMENT = "#d4d0c4";
const RING_DANGER = "#e85a5a";
// Center ring geometry — module constants so the drain hook (which needs the
// circumference) can run at the top of the component, above the
// abandoned-state early return.
const RING_SIZE = 132;
const RING_RADIUS = (RING_SIZE - 4) / 2; // 2 px stroke inset on each side
const RING_CIRC = 2 * Math.PI * RING_RADIUS;
// The ring + countdown text flip to the urgent palette at the same threshold
// the in-game TimerRing uses (URGENT_FRACTION = 1/8 of the original window).
// Re-using the shared helper keeps the per-move ring and the reconnect ring
// in lockstep — tweaking one place updates both.
// Felt panel background tokens — mirrors ClassicPanel internal gradient so the
// ring track sits on a matching field instead of a default surface.
const FELT_DARK = "#0e2818";

function formatCountdown(totalSeconds: number): string {
  if (totalSeconds <= 0) return "0:00";
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
}

export function ReconnectOverlay({
  disconnectedPlayerName,
  disconnectedPlayers,
  reconnectExpiresAt,
  abandonedData,
  viewerTeam = null,
  onReturnToLobby,
  totalSeconds = RECONNECT_TOTAL_SECONDS_DEFAULT,
}: ReconnectOverlayProps) {
  const chipPlayers: DisconnectedPlayerInfo[] =
    disconnectedPlayers && disconnectedPlayers.length > 0
      ? disconnectedPlayers
      : [{ name: disconnectedPlayerName, expiresAt: reconnectExpiresAt }];
  const { t } = useTranslation();
  // Center ring tracks the *earliest* expiry — the seat that drives abandon.
  // Per-row countdowns inside `DisconnectedPlayerRow` track each seat's own
  // expiry independently. The shared hook measures against the server's clock
  // (clockSync) with deadline-aligned ticks, same as the in-game TimerRing.
  const remainingSeconds = useTurnCountdown(reconnectExpiresAt);
  const prefersReducedMotion = useReducedMotion();
  // Deadline-anchored sweep — empties exactly at the abandon deadline. Must
  // run above the abandoned-state early return (hooks order); the quantized
  // dashoffset at the render site stays as the reduced-motion fallback.
  const ringDrain = useRingDrain(reconnectExpiresAt, totalSeconds, RING_CIRC);

  // Auto-redirect to lobby after match abandonment
  useEffect(() => {
    if (!abandonedData || !onReturnToLobby) return;
    const timer = setTimeout(() => {
      onReturnToLobby();
    }, MOTION.RECONNECT_REDIRECT);
    return () => clearTimeout(timer);
  }, [abandonedData, onReturnToLobby]);

  // Expired-window fallback redirect — covers the case where the disconnected
  // player's WS reconnects *after* the server already abandoned the match. The
  // session is gone server-side, so HandleReconnect returns silently and the
  // client never receives event:match_abandoned. Without this fallback the
  // overlay would freeze at 0:00 forever. We give the abandon event a brief
  // grace window (RECONNECT_REDIRECT) to arrive in case the server is just
  // slightly slower than the client wallclock, then navigate.
  const isExpiredWithoutAbandon = !abandonedData && remainingSeconds <= 0;
  useEffect(() => {
    if (!isExpiredWithoutAbandon || !onReturnToLobby) return;
    const timer = setTimeout(() => {
      onReturnToLobby();
    }, MOTION.RECONNECT_REDIRECT);
    return () => clearTimeout(timer);
  }, [isExpiredWithoutAbandon, onReturnToLobby]);

  // Abandoned state — gold/silver glow tied to the surviving team would be
  // misleading (no one *won* — the match was abandoned), so the panel uses a
  // red glow consistent with the urgent-state palette in the design system.
  if (abandonedData) {
    const teamAValue = abandonedData.teamAFinalScore;
    const teamBValue = abandonedData.teamBFinalScore;
    const usValue = viewerTeam === "teamA" ? teamAValue : teamBValue;
    const themValue = viewerTeam === "teamA" ? teamBValue : teamAValue;

    return (
      <div
        className="fixed inset-0"
        style={{ zIndex: Z.BLOCKER }}
        data-testid="reconnect-overlay"
        aria-live="assertive"
      >
        <OverlayBackdrop dim={0.7}>
          <ClassicPanel width={520} glowColor={RING_DANGER}>
            <div className="flex flex-col items-center text-center gap-3">
              <span
                className="font-body text-[11px] uppercase tracking-[0.25em]"
                style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.55 }}
              >
                {t("match.matchResult.title")}
              </span>

              <h2
                className="font-display text-2xl font-semibold"
                style={{ color: RING_DANGER, letterSpacing: -0.3 }}
                data-testid="abandon-title"
              >
                {t("match.disconnect.matchAbandoned", {
                  player: disconnectedPlayerName,
                })}
              </h2>

              {/* Final-score columns — viewer-first ordering. The
                  matchAbandonedScores translation is also rendered (visually
                  hidden) inside the abandon-scores wrapper so screen readers
                  and tests get a single readable string. */}
              <div
                className="flex items-center justify-center gap-6 mt-1 mb-1"
                data-testid="abandon-scores"
              >
                <ScoreColumn label={t("team.us")} value={usValue} gradient={TEAM_GOLD} />
                <span
                  className="font-display text-3xl"
                  style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.4 }}
                  aria-hidden
                >
                  ·
                </span>
                <ScoreColumn label={t("team.them")} value={themValue} gradient={TEAM_SILVER} />
                <span className="sr-only">
                  {t("match.disconnect.matchAbandonedScores", {
                    us: usValue,
                    them: themValue,
                  })}
                </span>
              </div>

              <p
                className="font-body text-xs mt-1 animate-pulse"
                style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.7 }}
              >
                {t("match.disconnect.returningToLobby")}
              </p>
            </div>
          </ClassicPanel>
        </OverlayBackdrop>
      </div>
    );
  }

  // Expired-without-abandon fallback — render a generic "match ended" panel
  // so the player isn't stuck staring at 0:00. The redirect effect above will
  // fire onReturnToLobby after RECONNECT_REDIRECT ms.
  if (isExpiredWithoutAbandon) {
    return (
      <div
        className="fixed inset-0"
        style={{ zIndex: Z.BLOCKER }}
        data-testid="reconnect-overlay"
        aria-live="assertive"
      >
        <OverlayBackdrop dim={0.7}>
          <ClassicPanel width={440} glowColor={RING_DANGER}>
            <div className="flex flex-col items-center text-center gap-3">
              <span
                className="font-body text-[11px] uppercase tracking-[0.25em]"
                style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.55 }}
              >
                {t("match.matchResult.title")}
              </span>

              <h2
                className="font-display text-2xl font-semibold"
                style={{ color: RING_DANGER, letterSpacing: -0.3 }}
                data-testid="match-ended-title"
              >
                {t("match.disconnect.matchEnded", { defaultValue: "Match has ended" })}
              </h2>

              <p
                className="font-body text-sm"
                style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.7 }}
              >
                {t("match.disconnect.reconnectFailed")}
              </p>

              <p
                className="font-body text-xs mt-1 animate-pulse"
                style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.7 }}
              >
                {t("match.disconnect.returningToLobby")}
              </p>
            </div>
          </ClassicPanel>
        </OverlayBackdrop>
      </div>
    );
  }

  // Countdown state — felt panel with title/subtitle header, centered ring
  // wrapping the m:ss countdown. Ring sweeps full → empty over the reconnect
  // window; both the ring stroke and the countdown text flip to the urgent
  // red palette when ≤25% of the window remains.
  const pct = Math.max(0, Math.min(1, remainingSeconds / totalSeconds));
  const isUrgent = pct <= URGENT_FRACTION;
  const ringColor = isUrgent ? RING_DANGER : RING_PARCHMENT;
  const ringSize = RING_SIZE;
  const ringRadius = RING_RADIUS;
  const ringCirc = RING_CIRC;

  return (
    <div
      className="fixed inset-0"
      style={{ zIndex: Z.BLOCKER }}
      data-testid="reconnect-overlay"
      aria-live="assertive"
    >
      <OverlayBackdrop dim={0.65}>
        <ClassicPanel
          width={440}
          title={t("match.disconnect.reconnecting")}
          subtitle={
            chipPlayers.length > 1
              ? t("match.disconnect.waitingMessageMany", { count: chipPlayers.length })
              : t("match.disconnect.waitingMessage", {
                  player: chipPlayers[0]?.name ?? disconnectedPlayerName,
                })
          }
        >
          <div className="flex flex-col items-center gap-5">
            {/* Per-seat rows — each player has their own `expiresAt` so the
                row countdown reflects that seat's clock specifically, not the
                center ring's "soonest abandon" view. When the earliest seat
                returns, the center ring jumps to the next-earliest, but each
                row keeps counting on its own. */}
            <div className="flex flex-col gap-2 w-full" data-testid="reconnect-player-name">
              {chipPlayers.map((p, idx) => (
                <DisconnectedPlayerRow
                  key={`${idx}-${p.name}`}
                  name={p.name}
                  expiresAt={p.expiresAt}
                  totalSeconds={totalSeconds}
                />
              ))}
            </div>

            <div className="relative" style={{ width: ringSize, height: ringSize }}>
              <svg
                width={ringSize}
                height={ringSize}
                viewBox={`0 0 ${ringSize} ${ringSize}`}
                aria-hidden
                style={{ transform: "rotate(-90deg)" }}
              >
                <circle
                  cx={ringSize / 2}
                  cy={ringSize / 2}
                  r={ringRadius}
                  fill={FELT_DARK}
                  stroke="rgba(255,255,255,0.08)"
                  strokeWidth={2}
                />
                {/* Keyed by the deadline: when the earliest-expiring seat
                    reconnects, reconnectExpiresAt swaps in place — a running
                    CSS animation keeps its original start time even when
                    animation-delay changes, so the element must be recreated
                    to re-anchor the sweep. */}
                <circle
                  key={reconnectExpiresAt}
                  cx={ringSize / 2}
                  cy={ringSize / 2}
                  r={ringRadius}
                  fill="none"
                  stroke={ringColor}
                  strokeWidth={2}
                  strokeLinecap="round"
                  strokeDasharray={ringCirc}
                  style={{
                    transition: `stroke ${MOTION.RING_COLOR_FLIP}ms ease-out`,
                    strokeDashoffset: ringCirc * (1 - pct),
                    ...(prefersReducedMotion ? undefined : ringDrain),
                  }}
                />
              </svg>
              <div
                className="absolute inset-0 flex items-center justify-center"
                style={{ pointerEvents: "none" }}
              >
                <span
                  className="font-display font-semibold tabular-nums"
                  style={{
                    fontSize: 36,
                    letterSpacing: 0.5,
                    color: isUrgent ? RING_DANGER : "var(--ink-light, #f5f2e8)",
                    transition: `color ${MOTION.RING_COLOR_FLIP}ms ease-out`,
                  }}
                  data-testid="reconnect-countdown"
                >
                  {formatCountdown(remainingSeconds)}
                </span>
              </div>
            </div>

            <p
              className="font-body text-[11px] uppercase tracking-[0.18em]"
              style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.55 }}
            >
              {t("match.disconnect.countdownLabel")}
            </p>
          </div>
        </ClassicPanel>
      </OverlayBackdrop>
    </div>
  );
}

/**
 * Single offline-player row inside the reconnect dialog. Avatar disc on the
 * left (PlayerSeat-style brass-frame + felt fill + initial), name in the
 * middle, individual countdown on the right showing this seat's own
 * `expiresAt`. Pulsing red dot on the avatar's bottom-right corner as the
 * offline signal. Felt background + brass border keep the row visually inside
 * the ClassicPanel family — no shout-y all-red pill.
 *
 * Each row's countdown is independent of the center ring: the center shows
 * the soonest abandon, but each player has their own clock. When the earliest
 * seat returns, the center jumps to the next-earliest while the still-offline
 * rows keep counting from their own expiries.
 */
function DisconnectedPlayerRow({
  name,
  expiresAt,
  totalSeconds,
}: {
  name: string;
  expiresAt: string;
  totalSeconds: number;
}) {
  const initial = (name || "?").charAt(0).toUpperCase();
  // Match PlayerSeat's avatar recipe so the initial sits in the same place as
  // the table seat avatars: a `disc` size + a 2 px conic frame around it
  // (frame = disc + 4). fontSize is derived from the inner disc radius (the
  // `* 0.5` ratio PlayerSeat uses), no letter-spacing tweak — letter-spacing
  // shifts the bbox and is what was nudging the "C" off-center here.
  const AVATAR_DISC = 32;
  const AVATAR_FRAME = AVATAR_DISC + 4;
  const remainingSeconds = useTurnCountdown(expiresAt);
  // Use the shared `isCountdownUrgent` so per-row coloring flips at the
  // same 1/8-of-window threshold as the center ring (and the in-game
  // per-move TimerRing). For a 120 s reconnect window that's 15 s.
  const rowUrgent = isCountdownUrgent(remainingSeconds, totalSeconds);
  return (
    <div
      className="flex items-center gap-3 px-3 py-2 rounded-xl"
      style={{
        background: "rgba(0,0,0,0.22)",
        border: "1px solid rgba(201,168,118,0.32)",
        boxShadow: "inset 0 1px 0 rgba(201,168,118,0.12)",
      }}
    >
      <div
        className="relative shrink-0"
        style={{ width: AVATAR_FRAME, height: AVATAR_FRAME }}
        aria-hidden
      >
        <div
          className="rounded-full"
          style={{
            position: "absolute",
            inset: 0,
            background:
              "conic-gradient(from 0deg, rgba(201,168,118,0.85), rgba(156,125,78,0.85), rgba(201,168,118,0.85))",
            padding: 2,
            boxShadow: "0 2px 6px rgba(0,0,0,0.3)",
          }}
        >
          <div
            className="rounded-full flex items-center justify-center font-display font-semibold"
            style={{
              width: "100%",
              height: "100%",
              background: "var(--avatar-inner, #1f2e23)",
              color: "var(--ink-light, #f5f2e8)",
              fontSize: AVATAR_DISC * 0.5,
            }}
          >
            {initial}
          </div>
        </div>
        {/* Offline indicator — pulsing red dot tucked into the avatar's
            bottom-right corner. Replaces the loud "DISCONNECTED" text badge
            from the previous design; the dot + dialog title already convey
            the state without crowding the row. */}
        <span
          className="rounded-full animate-pulse absolute"
          style={{
            right: -1,
            bottom: -1,
            width: 12,
            height: 12,
            background: RING_DANGER,
            border: "2px solid var(--avatar-inner, #1f2e23)",
            boxShadow: `0 0 8px ${RING_DANGER}aa`,
          }}
        />
      </div>
      <span
        className="font-display font-semibold flex-1"
        style={{
          fontSize: 16,
          letterSpacing: 0.2,
          color: "var(--ink-light, #f5f2e8)",
        }}
      >
        {name}
      </span>
      <span
        className="font-display font-semibold tabular-nums"
        style={{
          fontSize: 14,
          letterSpacing: 0.4,
          color: rowUrgent ? RING_DANGER : "var(--ink-light, #f5f2e8)",
          opacity: rowUrgent ? 1 : 0.75,
          transition: `color ${MOTION.RING_COLOR_FLIP}ms ease-out, opacity ${MOTION.RING_COLOR_FLIP}ms ease-out`,
          minWidth: 40,
          textAlign: "right",
        }}
        data-testid={`reconnect-row-countdown-${name}`}
      >
        {formatCountdown(remainingSeconds)}
      </span>
    </div>
  );
}

interface ScoreColumnProps {
  label: string;
  value: number;
  gradient: TeamGradient;
}

function ScoreColumn({ label, value, gradient }: ScoreColumnProps) {
  return (
    <div className="text-center">
      <p
        className="font-body text-xs font-semibold uppercase tracking-wider"
        style={{ color: gradient[0] }}
      >
        {label}
      </p>
      <p className="font-display text-4xl font-bold tabular-nums" style={{ color: gradient[0] }}>
        {value}
      </p>
    </div>
  );
}
