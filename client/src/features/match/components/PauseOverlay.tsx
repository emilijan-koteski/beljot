import { useTranslation } from "react-i18next";

import { playerDisplayName } from "@/shared/lib/botName";
import { Z } from "@/shared/lib/zLayers";
import type { PlayerState } from "@/shared/types/matchTypes";

import { ClassicActions } from "./overlay/ClassicActions";
import { ClassicButton } from "./overlay/ClassicButton";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { CountdownRing } from "./overlay/CountdownRing";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";

type Seats<T> = [T, T, T, T];

interface PauseOverlayProps {
  pausedPlayers: Seats<boolean>;
  pauseExtended: Seats<boolean>;
  pauseExpiresAt: string | null;
  pauseTotalSec: number;
  players: Seats<PlayerState>;
  myPlayerSeat: number;
  onResume: () => void;
  onExtend: () => void;
}

const SECONDS_PER_MINUTE = 60;
const FALLBACK_WINDOW_SEC = 120;

/** One pause window in seconds: the pause's full length split over the opening window and each extension. */
function pauseWindowSec(totalSec: number, extended: Seats<boolean>): number {
  const extensions = extended.filter(Boolean).length;
  return totalSec > 0 ? totalSec / (1 + extensions) : FALLBACK_WINDOW_SEC;
}

/**
 * The pause overlay: a countdown to the automatic resume, who holds the pause, a once-per-pause button to add
 * time, and Resume for a player holding it. Play resumes when every holder resumes or the time runs out.
 */
export function PauseOverlay({
  pausedPlayers,
  pauseExtended,
  pauseExpiresAt,
  pauseTotalSec,
  players,
  myPlayerSeat,
  onResume,
  onExtend,
}: PauseOverlayProps) {
  const { t } = useTranslation();
  const holding = pausedPlayers[myPlayerSeat] === true;
  const canExtend = pauseExtended[myPlayerSeat] === false;
  const minutes = Math.round(pauseWindowSec(pauseTotalSec, pauseExtended) / SECONDS_PER_MINUTE);

  return (
    <div
      className="fixed inset-0"
      style={{ zIndex: Z.BLOCKER }}
      data-testid="pause-overlay"
      aria-live="polite"
    >
      <OverlayBackdrop dim="blocker">
        <ClassicPanel size="dialog" title={t("match.pause.title")}>
          <div className="flex flex-col items-center gap-4">
            {pauseExpiresAt !== null && (
              <div className="flex flex-col items-center gap-2">
                <CountdownRing
                  expiresAt={pauseExpiresAt}
                  totalSeconds={pauseTotalSec > 0 ? pauseTotalSec : FALLBACK_WINDOW_SEC}
                  testId="pause-countdown"
                />
                <p
                  className="m-0 text-2xs tracking-caps uppercase"
                  style={{ color: "var(--ink-light)", opacity: 0.55 }}
                >
                  {t("match.pause.countdownLabel")}
                </p>
              </div>
            )}

            <ul className="m-0 flex w-full list-none flex-col gap-2 p-0">
              {players.map((player) => {
                const held = pausedPlayers[player.seat] === true;
                const extended = pauseExtended[player.seat] === true;
                return (
                  <li
                    key={player.seat}
                    className="flex items-center gap-3 rounded-lg border px-3 py-2"
                    style={{
                      background: held ? "var(--brass-10)" : "var(--shade-20)",
                      borderColor: held ? "var(--brass-40)" : "var(--sheen-6)",
                    }}
                    data-testid={`pause-player-${player.seat}`}
                    data-holding={held ? "true" : "false"}
                  >
                    <span className="flex-1 truncate text-sm" style={{ color: "var(--ink-light)" }}>
                      {playerDisplayName(t, player) ?? `Player ${player.seat + 1}`}
                    </span>
                    {extended && (
                      <span className="text-brass font-mono text-xs font-semibold">
                        {t("match.pause.extendedRow", { minutes })}
                      </span>
                    )}
                    {held && (
                      <span className="text-brass text-xs font-semibold">
                        {t("match.pause.pausedNow")}
                      </span>
                    )}
                  </li>
                );
              })}
            </ul>
          </div>

          <ClassicActions className="mt-5" testId="pause-actions">
            {!holding && (
              <p
                className="mr-auto mb-0 text-sm italic"
                style={{ color: "var(--ink-light)", opacity: 0.65 }}
                data-testid="pause-waiting"
              >
                {t("match.pause.waitingToResume")}
              </p>
            )}
            {canExtend && (
              <ClassicButton onClick={onExtend} data-testid="pause-extend-button">
                {t("match.pause.extend", { minutes })}
              </ClassicButton>
            )}
            {holding && (
              <ClassicButton variant="primary" onClick={onResume} data-testid="pause-resume-button">
                {t("match.pause.resume")}
              </ClassicButton>
            )}
          </ClassicActions>
        </ClassicPanel>
      </OverlayBackdrop>
    </div>
  );
}
