import { useTranslation } from "react-i18next";

import { playerDisplayName } from "@/shared/lib/botName";
import { Z } from "@/shared/lib/zLayers";
import type { PlayerState } from "@/shared/types/matchTypes";

import { formatCountdown, useTurnCountdown } from "../lib/turnCountdown";
import { ClassicActions } from "./overlay/ClassicActions";
import { ClassicButton } from "./overlay/ClassicButton";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { CountdownRing } from "./overlay/CountdownRing";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";

type Seats<T> = [T, T, T, T];

interface PauseOverlayProps {
  pausedPlayers: Seats<boolean>;
  pauseUsed: Seats<boolean>;
  playerPauseExpiresAt: Seats<string | null>;
  pauseWindowSec: number;
  players: Seats<PlayerState>;
  myPlayerSeat: number;
  onResume: () => void;
  onPause: () => void;
}

const FALLBACK_WINDOW_SEC = 120;

/** The end of the running pause that ends last, when play resumes if nobody resumes early. */
function latestExpiry(paused: Seats<boolean>, expiries: Seats<string | null>): string | null {
  let latest: string | null = null;
  paused.forEach((held, seat) => {
    const expiry = expiries[seat];
    if (held && expiry && (latest === null || Date.parse(expiry) > Date.parse(latest))) {
      latest = expiry;
    }
  });
  return latest;
}

/** A running pause's own clock, in its player's row. */
function RowCountdown({ expiresAt, seat }: { expiresAt: string; seat: number }) {
  const remaining = useTurnCountdown(expiresAt);
  return (
    <span
      className="text-brass font-mono text-xs font-semibold tabular-nums"
      data-testid={`pause-player-${seat}-countdown`}
    >
      {formatCountdown(remaining)}
    </span>
  );
}

/**
 * The pause overlay: each player's pause runs on its own clock and only they can end it early.
 * Play resumes once no pause is left running; a player with an unspent pause can start theirs here.
 */
export function PauseOverlay({
  pausedPlayers,
  pauseUsed,
  playerPauseExpiresAt,
  pauseWindowSec,
  players,
  myPlayerSeat,
  onResume,
  onPause,
}: PauseOverlayProps) {
  const { t } = useTranslation();
  const holding = pausedPlayers[myPlayerSeat] === true;
  const canPause = pauseUsed[myPlayerSeat] === false;
  const windowSec = pauseWindowSec > 0 ? pauseWindowSec : FALLBACK_WINDOW_SEC;
  const resumesAt = latestExpiry(pausedPlayers, playerPauseExpiresAt);

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
            {resumesAt !== null && (
              <div className="flex flex-col items-center gap-2">
                <CountdownRing
                  expiresAt={resumesAt}
                  totalSeconds={windowSec}
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
                const expiry = playerPauseExpiresAt[player.seat];
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
                    {held && (
                      <span className="text-brass text-xs font-semibold">
                        {t("match.pause.pausedNow")}
                      </span>
                    )}
                    {held && expiry && <RowCountdown expiresAt={expiry} seat={player.seat} />}
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
            {canPause && (
              <ClassicButton onClick={onPause} data-testid="pause-own-button">
                {t("match.pause.pauseButton")}
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
