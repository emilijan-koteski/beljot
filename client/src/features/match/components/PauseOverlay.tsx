import { Crown } from "lucide-react";
import { useTranslation } from "react-i18next";

import { playerDisplayName } from "@/shared/lib/botName";
import { Z } from "@/shared/lib/zLayers";
import type { PlayerState } from "@/shared/types/matchTypes";

import { ClassicActions } from "./overlay/ClassicActions";
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

  return (
    <div
      className="fixed inset-0"
      style={{ zIndex: Z.BLOCKER }}
      data-testid="pause-overlay"
      aria-live="polite"
    >
      <OverlayBackdrop dim="blocker">
        <ClassicPanel size="dialog" title={t("match.pause.title")}>
          <div className="flex flex-col gap-2 mb-4">
            {players.map((player) => {
              const isPaused = pausedPlayers[player.seat];
              const usedPause = pauseUsed[player.seat];
              return (
                <div
                  key={player.seat}
                  className="flex items-center gap-3 px-3 py-2 rounded-lg"
                  style={{
                    background: isPaused ? "var(--brass-10)" : "var(--shade-20)",
                    border: isPaused ? "1px solid var(--brass-40)" : "1px solid var(--sheen-6)",
                  }}
                  data-testid={isPaused ? `pause-player-${player.seat}` : undefined}
                >
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

          <ClassicActions className="mt-4" testId="pause-actions">
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
          </ClassicActions>
        </ClassicPanel>
      </OverlayBackdrop>
    </div>
  );
}
