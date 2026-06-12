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
 * Pause overlay — felt panel with per-player rows showing pause status.
 *
 *  • Player who actively paused → "Resume" button.
 *  • Player who hasn't used their pause yet → "Pause" stack button.
 *  • Player who already used their pause → quiet "waiting…" line.
 *  • Room owner → additional "Resume All" override (matches the server's
 *    ACTION_OWNER_UNPAUSE privilege; see [server/internal/game/pause.go]).
 *
 * No timer here — pause is open-ended on the server side. Players choose to
 * resume manually.
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
  // Names of everyone currently paused. The subtitle verb must agree with how
  // many there are, so we pick a plural key for 2+ (Slavic locales inflect the
  // verb — "паузираше" → "паузираа"; English reuses the same sentence).
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
      <OverlayBackdrop dim={0.65}>
        <ClassicPanel
          width={440}
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
                    background: isPaused ? "rgba(201,168,118,0.14)" : "rgba(0,0,0,0.18)",
                    border: isPaused
                      ? "1px solid rgba(201,168,118,0.45)"
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
                  <span
                    className="font-display text-sm flex-1"
                    style={{ color: "var(--ink-light, #f5f2e8)" }}
                  >
                    {playerDisplayName(t, player) ?? `Player ${player.seat + 1}`}
                  </span>
                  {isPaused ? (
                    <span
                      className="font-body text-[11px] font-semibold"
                      style={{ color: "var(--brass, #c9a876)" }}
                    >
                      {t("match.pause.pausedNow", { defaultValue: "paused now" })}
                    </span>
                  ) : (
                    <span
                      className="font-body text-[11px]"
                      style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.7 }}
                    >
                      {usedPause
                        ? t("match.pause.pauseUsedRow", { defaultValue: "pause used" })
                        : t("match.pause.pauseAvailable", { defaultValue: "pause available" })}
                    </span>
                  )}
                </div>
              );
            })}
          </div>

          {/* Owner-override + primary action share one row, justified to the
              edges. The override sits on the left in a red-tinted felt ghost
              variant — it stays in the ClassicButton family (felt + serif)
              while a Crown glyph and red border flag it as an admin-only
              action distinct from the player-level Resume on the right. */}
          <div className="flex items-center justify-between gap-3">
            {isRoomOwner ? (
              <ClassicButton
                onClick={onOwnerResume}
                data-testid="pause-owner-resume-button"
                style={{
                  border: "1px solid rgba(255,133,133,0.45)",
                  color: "#ffb1a8",
                  background: "linear-gradient(180deg, rgba(70,32,32,0.55), rgba(40,18,18,0.45))",
                  boxShadow: "inset 0 1px 0 rgba(255,133,133,0.18)",
                  padding: "8px 14px",
                  fontSize: 13,
                }}
              >
                <span className="inline-flex items-center gap-1.5">
                  <Crown size={13} aria-hidden="true" />
                  {t("match.pause.resumeAll")}
                </span>
              </ClassicButton>
            ) : (
              <span aria-hidden />
            )}

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
                className="font-body text-sm italic"
                style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.65 }}
                data-testid="pause-waiting"
              >
                {t("match.pause.waitingToResume")}
              </p>
            )}
          </div>
        </ClassicPanel>
      </OverlayBackdrop>
    </div>
  );
}
