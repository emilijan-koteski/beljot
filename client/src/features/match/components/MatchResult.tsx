import { ChevronDown, Coins } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import { HonorShield } from "@/shared/components/HonorShield";
import { MatchPlayerActions } from "@/shared/components/matchStats/MatchPlayerActions";
import { MatchStatsCard } from "@/shared/components/matchStats/MatchStatsCard";
import { useRoomLastMatchQuery } from "@/shared/hooks/queries/useMatches";
import { COIN_GOLD } from "@/shared/lib/coinGold";
import { formatCoins } from "@/shared/lib/formatCoins";
import { HONOR_TIER_COLOR, honorTierForScore, normalizeHonorTier } from "@/shared/lib/honor";
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
  /** Story 9.2: the viewer's net coin change for this match (winner's pot
   *  share minus stake, or −stake for a loser). Undefined for free (0 buy-in)
   *  matches and omitted while the settlement event is still in flight; a 0
   *  delta (e.g. a lone winner who only recovers their stake) renders nothing. */
  coinDelta?: number;
  /** Honour movement for this match (honour redesign R7). Undefined for a New
   *  Player, whose score is deliberately not shown, and while the honour event is
   *  still in flight. The chip itself renders only when the movement CROSSED a
   *  tier boundary (user decision 2026-07-31) — a few points inside the same
   *  band is noise, a change of standing is news. */
  honorSettlement?: { before: number; after: number; tier: string } | null;
  /** The room this match was played in. Enables the collapsible per-hand
   *  breakdown — without it there is no room to read the stats from, and the
   *  section simply never renders. */
  roomId?: number;
}

// Loss accent — the soft red shared with the surrender overlay, kept local
// since MatchResult is otherwise gold/silver. Wins use the off-theme COIN_GOLD.
const COIN_LOSS = "#ff8585";

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
  coinDelta,
  honorSettlement,
  roomId,
}: MatchResultProps) {
  const { t } = useTranslation();

  // The just-finished match, read back over REST (there is no match-stats WS
  // event). Enabled as soon as the overlay mounts rather than on expand, so the
  // toggle only ever appears when there is something behind it. The row is
  // already persisted by the time match_end reaches us (live_match.go writes
  // before it broadcasts), so this is a read, not a poll.
  const statsQuery = useRoomLastMatchQuery(roomId, roomId !== undefined);
  // Collapsed by default — the score and the two actions are what this dialog
  // is for; the breakdown is an optional second look.
  const [statsOpen, setStatsOpen] = useState(false);

  // Only ever paint a row that IS the match that just ended. The query key is
  // per-ROOM, so its cache entry describes match N-1 from the moment match N
  // starts — the room lobby populated it before anyone sat down. `useWsDispatch`
  // removes that entry on match_end and the query is `staleTime: 0`, but neither
  // helps an overlay mounted from a cache this component cannot see (a
  // reconnect, a second tab, a future caller). Both the persisted row and the
  // match_end payload are projections of the same final GameState — TeamScores
  // and WinnerTeam, verbatim — so equality here is exact, never a heuristic.
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

            {/* A "dosta" room's match ends the moment a team reaches the target,
                so the player is looking at a finished match with cards still in
                their hand and a score that jumped. Without this line that reads
                as a bug rather than as the rule the room was created with. The
                second sentence pre-empts the follow-up question: the hand
                breakdown below is one hand short, on purpose. */}
            {data.outcomeReason === "target_reached" && (
              <p
                className="font-body text-sm"
                style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.7 }}
                data-testid="match-result-target-reached-note"
              >
                {t("match.matchResult.targetReachedNote")}
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

            {/* Coin outcome (Story 9.2) — the won/lost stake, shown here in the
                result dialog instead of a transient toast. Gold for a win, red
                for a loss; hidden for free matches and net-zero deltas. */}
            {showCoins && (
              <div
                className="font-display inline-flex items-center gap-2 rounded-full px-3.5 py-1.5"
                style={{
                  color: coinWon ? COIN_GOLD : COIN_LOSS,
                  border: `1px solid ${coinWon ? "rgba(212,160,23,0.4)" : "rgba(255,133,133,0.4)"}`,
                  background: coinWon ? "rgba(212,160,23,0.08)" : "rgba(255,133,133,0.08)",
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

            {/* Honour movement (honour redesign R7), shown ONLY when the match
                moved the score across a tier boundary (user decision 2026-07-31):
                a few points inside the same band is noise every finished match
                produces, but a change of STANDING — Fair to Trusted, Trusted to
                Fair — is news, and it reads as tier names, not numbers.

                The before-tier is bucketed from the before-score with the same
                bands the server uses; the after-tier prefers the server's token.
                No scolding copy on the way down — the demotion is the
                consequence, and adding a sentence would make it a telling-off.

                Tier colour comes from HONOR_TIER_COLOR, whose values re-root inside
                .game-table, so this reads correctly on dark felt with no branch. */}
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
                      border: "1px solid rgba(201,168,118,0.35)",
                      background: "rgba(201,168,118,0.08)",
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

            <p
              className="font-body text-sm"
              style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.7 }}
              data-testid="match-result-duration"
            >
              {t("match.matchResult.duration")}: {formattedDuration}
            </p>

            {/* Per-hand breakdown — the SAME card the profile and the room
                lobby render, on a parchment inset because it is built for the
                parchment palette and this panel sits on dark felt.

                Three props are load-bearing here and nowhere else:
                • `handsLayout="stacked"` — HandsGrid picks its layout from a
                  VIEWPORT media query, and its desktop table is `min-w-155`
                  (620px), which overflows this 520px panel on any desktop.
                • `linkPlayers={false}` — a seat chip link would PUSH-navigate
                  out of the match, unmounting MatchPage before the player has
                  returned to the room or left it.
                • `allowRemoveFriend={false}` — that confirm is a z-50 dialog
                  and this panel is Z.PROMPT (74); it would open behind us. */}
            {statsAreThisMatch && stats !== undefined && (
              <div className="mt-1 w-full" data-testid="match-result-stats">
                <button
                  type="button"
                  onClick={() => setStatsOpen((v) => !v)}
                  className="font-body focus-visible:ring-brass/60 flex w-full cursor-pointer items-center justify-center gap-1.5 rounded-md py-1.5 text-[13px] opacity-70 transition-opacity hover:opacity-100 focus-visible:ring-2 focus-visible:outline-none"
                  style={{ color: "var(--ink-light, #f5f2e8)" }}
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
