import { useMemo } from "react";
import { useTranslation } from "react-i18next";

import { Z } from "@/shared/lib/zLayers";
import { type TeamString, teamStringForIndex } from "@/shared/types/matchTypes";
import type { MatchEndPayload } from "@/shared/types/wsEvents";

import { TEAM_GOLD, TEAM_SILVER, type TeamGradient } from "../lib/tableTheme";
import { ClassicButton } from "./overlay/ClassicButton";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";

interface MatchResultProps {
  data: MatchEndPayload;
  viewerTeam: TeamString;
  onReturnToLobby: () => void;
  /** Reopens the same room (status completed → waiting) and routes the viewer
   *  back to the room lobby on their original seat, so the group can play
   *  another match without recreating a room. */
  onReturnToRoom: () => void;
  /** Resolved username for `data.surrenderedBySeat`. Optional — falls back to
   *  `game.surrender.unknownProposer` when undefined and outcomeReason is
   *  "surrender" (e.g. a race where matchState was cleared before the overlay
   *  mounted). Has no effect for natural match-ends. */
  surrenderedByUsername?: string;
}

/**
 * End-of-match overlay — gold/silver-glowing classic panel with the winner
 * banner, viewer-first score columns, match duration, and two actions:
 * a primary "Return to room" (reopen + replay with the same group) and a
 * ghost "Return to lobby". Surrender wins also show a "<player> surrendered
 * the match" footnote.
 */
export function MatchResult({
  data,
  viewerTeam,
  onReturnToLobby,
  onReturnToRoom,
  surrenderedByUsername,
}: MatchResultProps) {
  const { t } = useTranslation();

  const winnerTeamString = teamStringForIndex(data.winnerTeam === 0 ? 0 : 1);
  const isUs = winnerTeamString === viewerTeam;
  const winnerGradient: TeamGradient = isUs ? TEAM_GOLD : TEAM_SILVER;
  const glowColor = winnerGradient[0];

  const teamAColumnLabel = viewerTeam === "teamA" ? t("team.us") : t("team.them");
  const teamBColumnLabel = viewerTeam === "teamB" ? t("team.us") : t("team.them");

  const teamAGradient: TeamGradient = viewerTeam === "teamA" ? TEAM_GOLD : TEAM_SILVER;
  const teamBGradient: TeamGradient = viewerTeam === "teamB" ? TEAM_GOLD : TEAM_SILVER;

  const formattedDuration = useMemo(() => {
    const totalSec = data.matchDurationSec;
    const minutes = Math.floor(totalSec / 60);
    const seconds = totalSec % 60;
    return `${minutes}m ${seconds}s`;
  }, [data.matchDurationSec]);

  return (
    <div className="fixed inset-0" style={{ zIndex: Z.PROMPT }} data-testid="match-result">
      <OverlayBackdrop dim={0.7}>
        <ClassicPanel width={520} glowColor={glowColor}>
          <div className="flex flex-col items-center text-center gap-3">
            <span
              className="font-body text-[11px] uppercase tracking-[0.25em]"
              style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.55 }}
              data-testid="match-result-title"
            >
              {t("match.matchResult.title")}
            </span>

            <h2
              className="font-display text-3xl font-semibold"
              style={{ color: glowColor, letterSpacing: -0.5 }}
              data-testid="match-result-winner"
              data-team={winnerTeamString}
            >
              {isUs ? t("match.matchResult.winnerUs") : t("match.matchResult.winnerThem")}
            </h2>

            {data.outcomeReason === "surrender" && (
              <p
                className="font-body text-sm"
                style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.7 }}
                data-testid="match-result-surrender-note"
              >
                {t("match.matchResult.surrenderNote", {
                  username: surrenderedByUsername ?? t("match.surrender.unknownProposer"),
                })}
              </p>
            )}

            {/* Final-score columns — viewer-first ordering preserved. */}
            <div className="flex items-center justify-center gap-6 mt-2 mb-2">
              {viewerTeam === "teamA" ? (
                <>
                  <ScoreColumn
                    team="teamA"
                    label={teamAColumnLabel}
                    score={data.teamAFinalScore}
                    gradient={teamAGradient}
                  />
                  <span
                    className="font-display text-3xl"
                    style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.4 }}
                  >
                    ·
                  </span>
                  <ScoreColumn
                    team="teamB"
                    label={teamBColumnLabel}
                    score={data.teamBFinalScore}
                    gradient={teamBGradient}
                  />
                </>
              ) : (
                <>
                  <ScoreColumn
                    team="teamB"
                    label={teamBColumnLabel}
                    score={data.teamBFinalScore}
                    gradient={teamBGradient}
                  />
                  <span
                    className="font-display text-3xl"
                    style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.4 }}
                  >
                    ·
                  </span>
                  <ScoreColumn
                    team="teamA"
                    label={teamAColumnLabel}
                    score={data.teamAFinalScore}
                    gradient={teamAGradient}
                  />
                </>
              )}
            </div>

            <p
              className="font-body text-sm"
              style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.7 }}
              data-testid="match-result-duration"
            >
              {t("match.matchResult.duration")}: {formattedDuration}
            </p>

            <div className="mt-2 flex w-full flex-col gap-2" data-testid="match-result-actions">
              <ClassicButton
                variant="primary"
                onClick={onReturnToRoom}
                data-testid="match-result-room-btn"
                className="w-full"
              >
                {t("match.matchResult.returnToRoom")}
              </ClassicButton>
              <ClassicButton
                variant="ghost"
                onClick={onReturnToLobby}
                data-testid="match-result-lobby-btn"
                className="w-full"
              >
                {t("match.matchResult.returnToLobby")}
              </ClassicButton>
            </div>
          </div>
        </ClassicPanel>
      </OverlayBackdrop>
    </div>
  );
}

interface ScoreColumnProps {
  team: TeamString;
  label: string;
  score: number;
  gradient: TeamGradient;
}

function ScoreColumn({ team, label, score, gradient }: ScoreColumnProps) {
  const testId = team === "teamA" ? "match-result-team-a-column" : "match-result-team-b-column";
  const scoreTestId = team === "teamA" ? "match-result-team-a-score" : "match-result-team-b-score";
  return (
    <div className="text-center" data-testid={testId} data-team={team}>
      <p
        className="font-body text-xs font-semibold uppercase tracking-wider"
        style={{ color: gradient[0] }}
      >
        {label}
      </p>
      <p
        className="font-display text-5xl font-bold tabular-nums"
        style={{ color: gradient[0] }}
        data-testid={scoreTestId}
      >
        {score}
      </p>
    </div>
  );
}
