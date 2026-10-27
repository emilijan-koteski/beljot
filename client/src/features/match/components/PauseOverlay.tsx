import { Crown } from "lucide-react";
import { useTranslation } from "react-i18next";

import { playerDisplayName } from "@/shared/lib/botName";
import { Z } from "@/shared/lib/zLayers";
import type { PlayerState } from "@/shared/types/matchTypes";

import { seatTeam, teamColors } from "../lib/tableTheme";
import { ClassicButton } from "./overlay/ClassicButton";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";

interface PauseOverlayProps {
  pausedPlayers: [boolean, boolean, boolean, boolean];
  pauseUsed: [boolean, boolean, boolean, boolean];
  players: [PlayerState, PlayerState, PlayerState, PlayerState];
  myPlayerSeat: number;
  isRoomOwner: boolean;
  onResume: () => void;
  onPause: () => void;
  onOwnerResume: () => void;
}

/**
 * Pause overlay listing each player's pause status, with Resume, Pause, and owner-only Resume all controls.
 */
export function PauseOverlay({
  pausedPlayers,
  pauseUsed,
  players,
  myPlayerSeat,
  isRoomOwner,
  onResume,
  onPause,
  onOwnerResume,
}: PauseOverlayProps) {
  const { t } = useTranslation();
  const hasActivePause = pausedPlayers[myPlayerSeat];
  const canStackPause = !hasActivePause && !pauseUsed[myPlayerSeat];
  const pausedNames = pausedPlayers
    .map((isPaused, seat) => (isPaused ? playerDisplayName(t, players[seat]) : null))
    .filter(Boolean);

  return (
    <div
      className="fixed inset-0"
      style={{ zIndex: Z.BLOCKER }}
      data-testid="pause-overlay"
      aria-live="polite"
    >
      <OverlayBackdrop dim="blocker">
        <ClassicPanel
          size="dialog"
          title={t("match.pause.title")}
          subtitle={t(
            pausedNames.length > 1 ? "match.pause.pausedByMany" : "match.pause.pausedBy",
            { player: pausedNames.join(", ") },
          )}
        >
          <div className="flex flex-col gap-2 mb-4">
            {players.map((player) => {
              const team = seatTeam(player.seat, myPlayerSeat);
              const dotColor = teamColors(team)[0];
              const isPaused = pausedPlayers[player.seat];
              const usedPause = pauseUsed[player.seat];
              return (
                <div
                  key={player.seat}
                  className="flex items-center gap-3 px-3 py-2 rounded-lg"
                  style={{
                    background: isPaused ? "var(--brass-10)" : "rgba(0,0,0,0.18)",
                    border: isPaused
                      ? "1px solid var(--brass-40)"
                      : "1px solid rgba(255,255,255,0.05)",
                  }}
                  data-testid={isPaused ? `pause-player-${player.seat}` : undefined}
                >
                  <span
                    aria-hidden
                    className="rounded-full"
                    style={{
                      width: 6,
                      height: 6,
                      background: dotColor,
                      boxShadow: `0 0 6px ${dotColor}`,
                    }}
                  />
                  <span className="text-sm flex-1" style={{ color: "var(--ink-light)" }}>
                    {playerDisplayName(t, player) ?? `Player ${player.seat + 1}`}
                  </span>
                  {isPaused ? (
                    <span className="text-xs font-semibold" style={{ color: "var(--brass)" }}>
                      {t("match.pause.pausedNow", { defaultValue: "paused now" })}
                    </span>
                  ) : (
                    <span className="text-xs" style={{ color: "var(--ink-light)", opacity: 0.7 }}>
                      {usedPause
                        ? t("match.pause.pauseUsedRow", { defaultValue: "pause used" })
                        : t("match.pause.pauseAvailable", { defaultValue: "pause available" })}
                    </span>
                  )}
                </div>
              );
            })}
          </div>

          <div
            className="mt-4 flex flex-wrap items-center justify-end gap-3.5"
            data-testid="pause-actions"
          >
            {hasActivePause ? (
              <ClassicButton variant="primary" onClick={onResume} data-testid="pause-resume-button">
                {t("match.pause.resume")}
              </ClassicButton>
            ) : canStackPause ? (
              <ClassicButton onClick={onPause} data-testid="pause-stack-button">
                {t("match.pause.pauseButton")}
              </ClassicButton>
            ) : (
              <p
                className="mr-auto text-sm italic"
                style={{ color: "var(--ink-light)", opacity: 0.65 }}
                data-testid="pause-waiting"
              >
                {t("match.pause.waitingToResume")}
              </p>
            )}

            {isRoomOwner && (
              <ClassicButton
                variant={hasActivePause ? "ghost" : "primary"}
                onClick={onOwnerResume}
                data-testid="pause-owner-resume-button"
              >
                <span className="inline-flex items-center justify-center gap-1.5">
                  <Crown size={14} aria-hidden="true" />
                  {t("match.pause.resumeAll")}
                </span>
              </ClassicButton>
            )}
          </div>
        </ClassicPanel>
      </OverlayBackdrop>
    </div>
  );
}
