import { useEffect } from "react";
import { useTranslation } from "react-i18next";

import { useSfxOnce } from "@/shared/audio/useSfxOnce";
import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { MOTION } from "@/shared/lib/motion";
import { Z } from "@/shared/lib/zLayers";
import { useAuthStore } from "@/shared/stores/authStore";
import type { SeasonSettlement } from "@/shared/stores/matchStore";
import type { TeamString } from "@/shared/types/matchTypes";
import type { MatchAbandonedPayload } from "@/shared/types/wsEvents";

import { TEAM_GOLD, TEAM_SILVER } from "../lib/tableTheme";
import {
  isCountdownUrgent,
  TIMER_RING_INK,
  URGENT_FRACTION,
  useRingDrain,
  useTurnCountdown,
} from "../lib/turnCountdown";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { FinalScore } from "./overlay/FinalScore";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";
import { ResultHeading } from "./overlay/ResultHeading";
import { SeasonSpLine } from "./SeasonSpLine";

interface DisconnectedPlayerInfo {
  name: string;
  expiresAt: string;
}

interface ReconnectOverlayProps {
  disconnectedPlayerName: string;
  disconnectedPlayers?: DisconnectedPlayerInfo[];
  reconnectExpiresAt: string;
  abandonedData?: MatchAbandonedPayload | null;
  viewerTeam?: TeamString | null;
  viewerSeat?: number | null;
  seasonSettlement?: SeasonSettlement | null;
  onReturnToLobby?: () => void;
  totalSeconds?: number;
  soundKey?: string | null;
}

const RECONNECT_TOTAL_SECONDS_DEFAULT = 120;
const RING_DANGER = "var(--danger-light)";
const RING_SIZE = 132;
const RING_STROKE_INSET = 4;
const RING_RADIUS = (RING_SIZE - RING_STROKE_INSET) / 2;
const RING_CIRC = 2 * Math.PI * RING_RADIUS;
const FELT_DARK = "#0e2818";

/**
 * Returns the viewer's win or loss for an abandoned match, based on whether they share the abandoner's team.
 * Returns none for the abandoner or when either seat is unknown or out of range.
 */
function abandonResult(
  abandonedData: MatchAbandonedPayload,
  viewerSeat: number | null,
): "win" | "loss" | null {
  const abandonerSeat = abandonedData.abandonedByPlayer;
  const abandonerSeatValid =
    Number.isInteger(abandonerSeat) && abandonerSeat >= 0 && abandonerSeat <= 3;
  if (
    !abandonerSeatValid ||
    viewerSeat === null ||
    !Number.isInteger(viewerSeat) ||
    viewerSeat < 0 ||
    viewerSeat > 3 ||
    viewerSeat === abandonerSeat
  ) {
    return null;
  }
  return viewerSeat % 2 === abandonerSeat % 2 ? "loss" : "win";
}

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
  viewerSeat = null,
  seasonSettlement = null,
  onReturnToLobby,
  totalSeconds = RECONNECT_TOTAL_SECONDS_DEFAULT,
  soundKey,
}: ReconnectOverlayProps) {
  const chipPlayers: DisconnectedPlayerInfo[] =
    disconnectedPlayers && disconnectedPlayers.length > 0
      ? disconnectedPlayers
      : [{ name: disconnectedPlayerName, expiresAt: reconnectExpiresAt }];
  const { t } = useTranslation();
  const viewerUsername = useAuthStore((s) => s.user?.username);
  const viewerIsDisconnected =
    viewerUsername !== undefined && chipPlayers.some((p) => p.name === viewerUsername);

  const remainingSeconds = useTurnCountdown(reconnectExpiresAt);
  const prefersReducedMotion = useReducedMotion();
  const ringDrain = useRingDrain(reconnectExpiresAt, totalSeconds, RING_CIRC);

  const result = abandonedData ? abandonResult(abandonedData, viewerSeat) : null;
  useSfxOnce(result === "loss" ? "matchLose" : "matchWin", result !== null ? soundKey : null);

  useEffect(() => {
    if (!abandonedData || !onReturnToLobby) return;
    const timer = setTimeout(() => {
      onReturnToLobby();
    }, MOTION.RECONNECT_REDIRECT);
    return () => clearTimeout(timer);
  }, [abandonedData, onReturnToLobby]);

  const isExpiredWithoutAbandon = !abandonedData && remainingSeconds <= 0;
  useEffect(() => {
    if (!isExpiredWithoutAbandon || !onReturnToLobby) return;
    const timer = setTimeout(() => {
      onReturnToLobby();
    }, MOTION.RECONNECT_REDIRECT);
    return () => clearTimeout(timer);
  }, [isExpiredWithoutAbandon, onReturnToLobby]);

  if (abandonedData) {
    const teamAValue = abandonedData.teamAFinalScore;
    const teamBValue = abandonedData.teamBFinalScore;
    const usValue = viewerTeam === "teamA" ? teamAValue : teamBValue;
    const themValue = viewerTeam === "teamA" ? teamBValue : teamAValue;
    const showResultLine = result !== null;
    const viewerLost = result === "loss";
    const resultColor = result === null ? RING_DANGER : viewerLost ? TEAM_SILVER[0] : TEAM_GOLD[0];

    return (
      <div
        className="fixed inset-0"
        style={{ zIndex: Z.BLOCKER }}
        data-testid="reconnect-overlay"
        aria-live="assertive"
      >
        <OverlayBackdrop dim="blocker">
          <ClassicPanel size="wide" glowColor={resultColor}>
            <div className="flex flex-col items-center text-center gap-3">
              <ResultHeading
                eyebrow={t("match.matchResult.title")}
                color={resultColor}
                data-testid="abandon-title"
                data-result={result ?? undefined}
              >
                {t("match.disconnect.matchAbandoned", {
                  player: disconnectedPlayerName,
                })}
              </ResultHeading>

              <FinalScore
                sides={[
                  { label: t("team.us"), score: usValue, color: TEAM_GOLD[0] },
                  { label: t("team.them"), score: themValue, color: TEAM_SILVER[0] },
                ]}
                testId="abandon-scores"
              >
                <span className="sr-only">
                  {t("match.disconnect.matchAbandonedScores", {
                    us: usValue,
                    them: themValue,
                  })}
                </span>
              </FinalScore>

              {showResultLine && (
                <p
                  className="text-sm"
                  style={{ color: "var(--ink-light)", opacity: 0.8 }}
                  data-testid="abandon-result-line"
                  data-result={viewerLost ? "loss" : "win"}
                >
                  {viewerLost
                    ? t("match.disconnect.abandonCountsLoss")
                    : t("match.disconnect.abandonCountsWin")}
                </p>
              )}

              {seasonSettlement && <SeasonSpLine settlement={seasonSettlement} />}

              <p
                className="text-xs mt-1 animate-pulse"
                style={{ color: "var(--ink-light)", opacity: 0.7 }}
              >
                {t("match.disconnect.returningToLobby")}
              </p>
            </div>
          </ClassicPanel>
        </OverlayBackdrop>
      </div>
    );
  }

  if (isExpiredWithoutAbandon) {
    return (
      <div
        className="fixed inset-0"
        style={{ zIndex: Z.BLOCKER }}
        data-testid="reconnect-overlay"
        aria-live="assertive"
      >
        <OverlayBackdrop dim="blocker">
          <ClassicPanel size="dialog" glowColor={RING_DANGER}>
            <div className="flex flex-col items-center text-center gap-3">
              <span
                className="text-2xs uppercase tracking-caps"
                style={{ color: "var(--ink-light)", opacity: 0.55 }}
              >
                {t("match.matchResult.title")}
              </span>

              <h2
                className="text-2xl font-semibold tracking-tight"
                style={{ color: RING_DANGER }}
                data-testid="match-ended-title"
              >
                {t("match.disconnect.matchEnded", { defaultValue: "Match has ended" })}
              </h2>

              <p className="text-sm" style={{ color: "var(--ink-light)", opacity: 0.7 }}>
                {t("match.disconnect.reconnectFailed")}
              </p>

              <p
                className="text-xs mt-1 animate-pulse"
                style={{ color: "var(--ink-light)", opacity: 0.7 }}
              >
                {t("match.disconnect.returningToLobby")}
              </p>
            </div>
          </ClassicPanel>
        </OverlayBackdrop>
      </div>
    );
  }

  const pct = Math.max(0, Math.min(1, remainingSeconds / totalSeconds));
  const isUrgent = pct <= URGENT_FRACTION;
  const ringColor = isUrgent ? RING_DANGER : TIMER_RING_INK;
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
      <OverlayBackdrop dim="blocker">
        <ClassicPanel
          size="dialog"
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
                  className="font-semibold tabular-nums"
                  style={{
                    fontSize: 36,
                    color: isUrgent ? RING_DANGER : "var(--ink-light)",
                    transition: `color ${MOTION.RING_COLOR_FLIP}ms ease-out`,
                  }}
                  data-testid="reconnect-countdown"
                >
                  {formatCountdown(remainingSeconds)}
                </span>
              </div>
            </div>

            <p
              className="text-2xs uppercase tracking-caps"
              style={{ color: "var(--ink-light)", opacity: 0.55 }}
            >
              {t("match.disconnect.countdownLabel")}
            </p>

            <p
              className="text-center text-xs"
              style={{ color: "var(--ink-light)", opacity: 0.75, maxWidth: 340 }}
              data-testid="reconnect-honor-note"
              data-viewer-dropped={viewerIsDisconnected ? "true" : "false"}
            >
              {viewerIsDisconnected
                ? t("match.disconnect.honorNoteSelf")
                : t("match.disconnect.honorNoteOther")}
            </p>
          </div>
        </ClassicPanel>
      </OverlayBackdrop>
    </div>
  );
}

/**
 * Offline-player row in the reconnect dialog with avatar, name, and that seat's own countdown.
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
  const AVATAR_DISC = 32;
  const AVATAR_FRAME = AVATAR_DISC + 4;
  const remainingSeconds = useTurnCountdown(expiresAt);
  const rowUrgent = isCountdownUrgent(remainingSeconds, totalSeconds);
  return (
    <div
      className="flex items-center gap-3 px-3 py-2 rounded-xl"
      style={{
        background: "rgba(0,0,0,0.22)",
        border: "1px solid var(--brass-40)",
        boxShadow: "inset 0 1px 0 var(--brass-10)",
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
              "conic-gradient(from 0deg, var(--brass-85), rgba(156,125,78,0.85), var(--brass-85))",
            padding: 2,
            boxShadow: "var(--shadow-felt-sm)",
          }}
        >
          <div
            className="rounded-full flex items-center justify-center font-semibold"
            style={{
              width: "100%",
              height: "100%",
              background: "var(--avatar-inner)",
              color: "var(--ink-light)",
              fontSize: AVATAR_DISC * 0.5,
            }}
          >
            {initial}
          </div>
        </div>
        <span
          className="rounded-full animate-pulse absolute"
          style={{
            right: -1,
            bottom: -1,
            width: 12,
            height: 12,
            background: RING_DANGER,
            border: "2px solid var(--avatar-inner)",
            boxShadow: `0 0 6px color-mix(in srgb, ${RING_DANGER} 67%, transparent)`,
          }}
        />
      </div>
      <span
        className="font-semibold flex-1"
        style={{
          fontSize: 16,
          color: "var(--ink-light)",
        }}
      >
        {name}
      </span>
      <span
        className="font-semibold tabular-nums"
        style={{
          fontSize: 14,
          color: rowUrgent ? RING_DANGER : "var(--ink-light)",
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
