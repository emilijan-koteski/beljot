import { ChevronDown, Coins } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import { useSfxOnce } from "@/shared/audio/useSfxOnce";
import { HonorShield } from "@/shared/components/HonorShield";
import { MatchPlayerActions } from "@/shared/components/matchStats/MatchPlayerActions";
import { MatchStatsCard } from "@/shared/components/matchStats/MatchStatsCard";
import { useRoomLastMatchQuery } from "@/shared/hooks/queries/useMatches";
import { COIN_GOLD } from "@/shared/lib/coinGold";
import { formatCoins } from "@/shared/lib/formatCoins";
import { HONOR_TIER_COLOR, honorTierForScore, normalizeHonorTier } from "@/shared/lib/honor";
import { Z } from "@/shared/lib/zLayers";
import type { SeasonSettlement } from "@/shared/stores/matchStore";
import { type TeamString, teamStringForIndex } from "@/shared/types/matchTypes";
import type { MatchEndPayload } from "@/shared/types/wsEvents";

import { TEAM_GOLD, TEAM_SILVER, type TeamGradient } from "../lib/tableTheme";
import { ClassicButton } from "./overlay/ClassicButton";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";
import { SeasonSpLine } from "./SeasonSpLine";

interface MatchResultProps {
  data: MatchEndPayload;
  viewerTeam: TeamString;
  onReturnToLobby: () => void;
  onReturnToRoom: () => void;
  surrenderedByUsername?: string;
  coinDelta?: number;
  honorSettlement?: { before: number; after: number; tier: string } | null;
  seasonSettlement?: SeasonSettlement | null;
  roomId?: number;
  soundKey?: string | null;
}

const COIN_LOSS = "var(--danger-light)";

/**
 * End-of-match overlay with the winner banner, viewer-first scores, and the return-to-room and lobby actions.
 */
export function MatchResult({
  data,
  viewerTeam,
  onReturnToLobby,
  onReturnToRoom,
  surrenderedByUsername,
  coinDelta,
  honorSettlement,
  seasonSettlement,
  roomId,
  soundKey,
}: MatchResultProps) {
  const { t } = useTranslation();

  const statsQuery = useRoomLastMatchQuery(roomId, roomId !== undefined);
  const [statsOpen, setStatsOpen] = useState(false);

  const stats = statsQuery.data;
  const statsAreThisMatch =
    stats !== undefined &&
    stats.teamAScore === data.teamAFinalScore &&
    stats.teamBScore === data.teamBFinalScore &&
    stats.winnerTeam === data.winnerTeam;

  const showCoins = typeof coinDelta === "number" && coinDelta !== 0;
  const coinWon = (coinDelta ?? 0) > 0;

  const winnerTeamString = teamStringForIndex(data.winnerTeam === 0 ? 0 : 1);
  const isUs = winnerTeamString === viewerTeam;
  useSfxOnce(isUs ? "matchWin" : "matchLose", soundKey);
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
              style={{ color: "var(--ink-light)", opacity: 0.55 }}
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
                style={{ color: "var(--ink-light)", opacity: 0.7 }}
                data-testid="match-result-surrender-note"
              >
                {t("match.matchResult.surrenderNote", {
                  username: surrenderedByUsername ?? t("match.surrender.unknownProposer"),
                })}
              </p>
            )}

            {data.outcomeReason === "target_reached" && (
              <p
                className="font-body text-sm"
                style={{ color: "var(--ink-light)", opacity: 0.7 }}
                data-testid="match-result-target-reached-note"
              >
                {t("match.matchResult.targetReachedNote")}
              </p>
            )}

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
                    style={{ color: "var(--ink-light)", opacity: 0.4 }}
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
                    style={{ color: "var(--ink-light)", opacity: 0.4 }}
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

            {showCoins && (
              <div
                className="font-display inline-flex items-center gap-2 rounded-full px-3.5 py-1.5"
                style={{
                  color: coinWon ? COIN_GOLD : COIN_LOSS,
                  border: `1px solid ${coinWon ? "rgba(212,160,23,0.4)" : "color-mix(in srgb, var(--danger-light) 40%, transparent)"}`,
                  background: coinWon
                    ? "rgba(212,160,23,0.08)"
                    : "color-mix(in srgb, var(--danger-light) 8%, transparent)",
                }}
                data-testid="match-result-coins"
                data-coin-delta={coinDelta}
              >
                <Coins className="h-4 w-4" aria-hidden="true" />
                <span className="text-base font-semibold tabular-nums">
                  {coinWon
                    ? t("match.settlement.won", { amount: formatCoins(coinDelta ?? 0) })
                    : t("match.settlement.lost", { amount: formatCoins(-(coinDelta ?? 0)) })}
                </span>
              </div>
            )}

            {honorSettlement &&
              (() => {
                const tierBefore = honorTierForScore(honorSettlement.before);
                const tierAfter = normalizeHonorTier(honorSettlement.tier, honorSettlement.after);
                if (tierBefore === tierAfter) return null;
                return (
                  <div
                    className="font-display inline-flex items-center gap-2 rounded-full px-3.5 py-1.5"
                    style={{
                      color: HONOR_TIER_COLOR[tierAfter],
                      border: "1px solid var(--brass-40)",
                      background: "var(--brass-10)",
                    }}
                    data-testid="match-result-honor"
                    data-honor-before={honorSettlement.before}
                    data-honor-after={honorSettlement.after}
                    data-tier-before={tierBefore}
                    data-tier-after={tierAfter}
                  >
                    <HonorShield tier={tierAfter} size={16} />
                    <span className="text-base font-semibold">
                      {t("match.settlement.honorTierChanged", {
                        before: t(`profile.honor.tier.${tierBefore}`),
                        after: t(`profile.honor.tier.${tierAfter}`),
                      })}
                    </span>
                  </div>
                );
              })()}

            {seasonSettlement && <SeasonSpLine settlement={seasonSettlement} />}

            <p
              className="font-body text-sm"
              style={{ color: "var(--ink-light)", opacity: 0.7 }}
              data-testid="match-result-duration"
            >
              {t("match.matchResult.duration")}: {formattedDuration}
            </p>

            {statsAreThisMatch && stats !== undefined && (
              <div className="mt-1 w-full" data-testid="match-result-stats">
                <button
                  type="button"
                  onClick={() => setStatsOpen((v) => !v)}
                  className="font-body focus-visible:ring-brass/55 flex w-full cursor-pointer items-center justify-center gap-1.5 rounded-md py-1.5 text-[13px] opacity-70 transition-opacity hover:opacity-100 focus-visible:ring-2 focus-visible:outline-none"
                  style={{ color: "var(--ink-light)" }}
                  aria-expanded={statsOpen}
                  aria-controls="match-result-stats-panel"
                  data-testid="match-result-stats-toggle"
                >
                  {statsOpen ? t("match.matchResult.statsHide") : t("match.matchResult.statsShow")}
                  <ChevronDown
                    className={`size-4 transition-transform ${statsOpen ? "rotate-180" : ""}`}
                    aria-hidden="true"
                  />
                </button>
                {statsOpen && (
                  <div
                    id="match-result-stats-panel"
                    className="parchment-inset mt-2 max-h-[46vh] overflow-y-auto rounded-xl p-2 text-left"
                    data-testid="match-result-stats-panel"
                  >
                    <ul className="m-0 list-none p-0">
                      <MatchStatsCard
                        match={stats}
                        isOpen
                        onToggle={() => setStatsOpen(false)}
                        handsLayout="stacked"
                        linkPlayers={false}
                        footer={<MatchPlayerActions match={stats} allowRemoveFriend={false} />}
                      />
                    </ul>
                  </div>
                )}
              </div>
            )}

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
