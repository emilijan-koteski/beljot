import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { MOTION, motionDuration } from "@/shared/lib/motion";
import { type TeamString, teamStringForIndex } from "@/shared/types/matchTypes";

import { TEAM_GOLD, TEAM_SILVER, type TeamGradient } from "../lib/tableTheme";

interface ScorePanelProps {
  viewerTeam: TeamString;
  teamAScore: number;
  teamBScore: number;
  teamATricks: number;
  teamBTricks: number;
  teamAHandPotential?: number;
  teamBHandPotential?: number;
  lastTrickBonus?: number;
  lastTrickTeam?: number;
  /** Match target — defaults to 1001 for Bitola. */
  matchTarget?: number;
  /** Current hand number (1-based). When omitted, the header right-side
   *  pill (Hand N · Variant) is hidden — keeps stand-alone test renders
   *  free of game-state coupling. */
  handNumber?: number;
  /** Variant label rendered next to the hand number ("Bitola"). */
  variantLabel?: string;
}

const PANEL_BG = "var(--panel-dark, rgba(20,45,30,0.85))";
const PANEL_BORDER = "1px solid rgba(201,168,118,0.4)";
const INK = "var(--ink-light, #f5f2e8)";
const BRASS_LABEL = "var(--brass, #c9a876)";

interface RowProps {
  label: string;
  rowTestId: string;
  scoreTestId: string;
  potentialTestId: string;
  team: TeamString;
  teamGradient: TeamGradient;
  score: number;
  potential: number;
  tricks: number;
  matchTarget: number;
}

function ScoreRow({
  label,
  rowTestId,
  scoreTestId,
  potentialTestId,
  team,
  teamGradient,
  score,
  potential,
  tricks,
  matchTarget,
}: RowProps) {
  const { t } = useTranslation();
  const showPotential = potential > 0;
  const dot = teamGradient[0];
  // Cap the bar at 100% in the rare case `potential` overshoots the felt-style
  // 162 trick-points cap (declarations + bonuses can push it higher).
  const barPct = Math.max(0, Math.min(100, (potential / 162) * 100));

  return (
    <div className="px-4 py-2.5 flex flex-col gap-1.5" data-testid={rowTestId} data-team={team}>
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <span
            aria-hidden
            style={{
              width: 10,
              height: 10,
              borderRadius: "50%",
              background: dot,
              boxShadow: `0 0 8px ${dot}`,
            }}
          />
          <span
            className="font-body text-xs font-semibold uppercase tracking-wide"
            style={{ color: INK, opacity: 0.85 }}
            data-testid={team === "teamA" ? "score-label-a" : "score-label-b"}
          >
            {label}
          </span>
        </div>
        <span
          className="font-display font-bold tabular-nums motion-safe:transition-all motion-safe:duration-300"
          style={{ color: INK, fontSize: 22, letterSpacing: -0.5 }}
          data-testid={scoreTestId}
        >
          {score}
          <span
            className="text-[11px]"
            style={{ opacity: 0.5, marginLeft: 2 }}
          >{` / ${matchTarget}`}</span>
        </span>
      </div>
      {showPotential && (
        <div
          className="flex items-center gap-2 font-body text-[10.5px] tabular-nums"
          style={{ color: INK, opacity: 0.7 }}
          data-testid={potentialTestId}
        >
          <span>
            +{potential} {t("match.score.thisHand")}
          </span>
          <div
            className="flex-1"
            style={{
              height: 4,
              borderRadius: 2,
              background: "rgba(255,255,255,0.1)",
              overflow: "hidden",
            }}
          >
            <div
              style={{
                width: `${barPct}%`,
                height: "100%",
                background: `linear-gradient(90deg, ${teamGradient[0]}, ${teamGradient[1]})`,
              }}
            />
          </div>
          <span>
            {tricks} {tricks === 1 ? t("match.score.trick") : t("match.score.tricks")}
          </span>
        </div>
      )}
    </div>
  );
}

/**
 * Top-left scoreboard panel — felt-tinted classic chip.
 *
 * Two team rows, each ordered viewer-first: the viewer's row always reads
 * Gold ("Us"); the opposition reads Silver ("Them"). Wire-level data is still
 * keyed by teamA/teamB so existing test selectors and downstream consumers
 * don't drift. A small per-team mini-bar shows hand potential ('this hand'
 * total) so players can read momentum at a glance.
 */
export function ScorePanel({
  viewerTeam,
  teamAScore,
  teamBScore,
  teamATricks,
  teamBTricks,
  teamAHandPotential = 0,
  teamBHandPotential = 0,
  lastTrickBonus,
  lastTrickTeam,
  matchTarget = 1001,
  handNumber,
  variantLabel,
}: ScorePanelProps) {
  const { t } = useTranslation();
  const prefersReducedMotion = useReducedMotion();

  const [showBonus, setShowBonus] = useState<{ team: 0 | 1; amount: number } | null>(null);

  useEffect(() => {
    if (lastTrickBonus !== undefined && lastTrickBonus > 0 && lastTrickTeam !== undefined) {
      setShowBonus({ team: lastTrickTeam === 0 ? 0 : 1, amount: lastTrickBonus });
    }
  }, [lastTrickBonus, lastTrickTeam]);

  useEffect(() => {
    if (showBonus === null) return;
    const timer = setTimeout(
      () => setShowBonus(null),
      motionDuration(
        prefersReducedMotion,
        MOTION.SCORE_BONUS_FADE,
        MOTION.SCORE_BONUS_FADE_REDUCED,
      ),
    );
    return () => clearTimeout(timer);
  }, [showBonus, prefersReducedMotion]);

  const teamALabel = viewerTeam === "teamA" ? t("team.us") : t("team.them");
  const teamBLabel = viewerTeam === "teamB" ? t("team.us") : t("team.them");

  // Viewer-relative gradients: viewer's own team is always gold; the other
  // team is always silver, regardless of teamA/teamB identity.
  const teamAGradient: TeamGradient = viewerTeam === "teamA" ? TEAM_GOLD : TEAM_SILVER;
  const teamBGradient: TeamGradient = viewerTeam === "teamB" ? TEAM_GOLD : TEAM_SILVER;

  const bonusTeamString = showBonus !== null ? teamStringForIndex(showBonus.team) : null;
  const bonusGradient: TeamGradient | null =
    bonusTeamString === "teamA"
      ? teamAGradient
      : bonusTeamString === "teamB"
        ? teamBGradient
        : null;

  // Viewer-first compact rows for the mobile scoreboard. Us is always gold,
  // Them always silver — matching the desktop panel's viewer-relative colours.
  const usIsA = viewerTeam === "teamA";
  const mobileRows = [
    {
      key: "us",
      label: t("team.us"),
      dot: TEAM_GOLD[0],
      potential: usIsA ? teamAHandPotential : teamBHandPotential,
      score: usIsA ? teamAScore : teamBScore,
    },
    {
      key: "them",
      label: t("team.them"),
      dot: TEAM_SILVER[0],
      potential: usIsA ? teamBHandPotential : teamAHandPotential,
      score: usIsA ? teamBScore : teamAScore,
    },
  ];

  return (
    <>
      {/* Mobile (<md): ultra-compact scoreboard — one line per team, showing
          this-hand points (+N) and total / target. No heading, momentum bars,
          trick counts or hand/variant metadata; the desktop panel keeps those. */}
      <div
        className="fixed top-3 left-3 z-10 flex flex-col gap-0.5 rounded-lg px-2.5 py-1.5 md:hidden"
        style={{
          background: PANEL_BG,
          border: PANEL_BORDER,
          boxShadow: "0 4px 14px rgba(0,0,0,0.3)",
          backdropFilter: "blur(8px)",
          WebkitBackdropFilter: "blur(8px)",
        }}
        data-testid="score-panel-mobile"
        aria-live="polite"
      >
        {mobileRows.map((r) => (
          <div
            key={r.key}
            className="flex items-center gap-1.5 tabular-nums"
            style={{ color: INK }}
          >
            <span
              aria-hidden
              style={{
                width: 6,
                height: 6,
                borderRadius: "50%",
                background: r.dot,
                boxShadow: `0 0 5px ${r.dot}`,
              }}
            />
            <span
              className="font-body text-[11px] font-semibold uppercase"
              style={{ opacity: 0.85, minWidth: 30 }}
            >
              {r.label}
            </span>
            <span
              className="font-body text-[11px] font-semibold tabular-nums"
              style={{ color: BRASS_LABEL, minWidth: 24, textAlign: "right" }}
            >
              +{r.potential}
            </span>
            <span className="font-display text-[11px] font-bold" style={{ marginLeft: 4 }}>
              {r.score}
              <span style={{ opacity: 0.5 }}>{`/${matchTarget}`}</span>
            </span>
          </div>
        ))}
      </div>

      {/* Desktop (md+): full felt scoreboard panel. */}
      <div
        className="fixed top-4 left-4 z-10 hidden overflow-hidden rounded-xl md:block"
        style={{
          background: PANEL_BG,
          border: PANEL_BORDER,
          boxShadow: "0 8px 24px rgba(0,0,0,0.3)",
          backdropFilter: "blur(8px)",
          WebkitBackdropFilter: "blur(8px)",
          minWidth: 240,
        }}
        data-testid="score-panel"
        aria-live="polite"
      >
        {/* Header eyebrow — brass "SCOREBOARD" on the left, "Hand N · Variant"
          metadata on the right (per design). Both lines share the same brass
          uppercase tracking so they read as one band. */}
        <div
          className="flex items-center justify-between px-4 py-2 font-body text-[10.5px] uppercase tracking-wider"
          style={{
            color: BRASS_LABEL,
            borderBottom: "1px solid rgba(255,255,255,0.06)",
          }}
        >
          <span>{t("match.score.heading", { defaultValue: "Scoreboard" })}</span>
          {(handNumber !== undefined || variantLabel) && (
            <span style={{ opacity: 0.85 }} data-testid="score-meta">
              {handNumber !== undefined && variantLabel
                ? t("match.score.handVariant", {
                    hand: handNumber,
                    variant: variantLabel,
                    defaultValue: `Hand ${handNumber} · ${variantLabel}`,
                  })
                : handNumber !== undefined
                  ? t("match.score.handLabel", {
                      hand: handNumber,
                      defaultValue: `Hand ${handNumber}`,
                    })
                  : variantLabel}
            </span>
          )}
        </div>

        {/* Rows are laid out viewer-first: the viewer's row ("Us") always
          renders above the opposition ("Them"), regardless of which seats
          map to teamA / teamB. The wire-level data-testid contract still
          tags rows by absolute team identity (score-row-a / score-row-b),
          so existing assertions on team-A vs team-B numbers stay valid. */}
        {viewerTeam === "teamA" ? (
          <>
            <ScoreRow
              label={teamALabel}
              rowTestId="score-row-a"
              scoreTestId="score-a"
              potentialTestId="score-a-potential"
              team="teamA"
              teamGradient={teamAGradient}
              score={teamAScore}
              potential={teamAHandPotential}
              tricks={teamATricks}
              matchTarget={matchTarget}
            />
            <div style={{ height: 1, background: "rgba(255,255,255,0.06)" }} />
            <ScoreRow
              label={teamBLabel}
              rowTestId="score-row-b"
              scoreTestId="score-b"
              potentialTestId="score-b-potential"
              team="teamB"
              teamGradient={teamBGradient}
              score={teamBScore}
              potential={teamBHandPotential}
              tricks={teamBTricks}
              matchTarget={matchTarget}
            />
          </>
        ) : (
          <>
            <ScoreRow
              label={teamBLabel}
              rowTestId="score-row-b"
              scoreTestId="score-b"
              potentialTestId="score-b-potential"
              team="teamB"
              teamGradient={teamBGradient}
              score={teamBScore}
              potential={teamBHandPotential}
              tricks={teamBTricks}
              matchTarget={matchTarget}
            />
            <div style={{ height: 1, background: "rgba(255,255,255,0.06)" }} />
            <ScoreRow
              label={teamALabel}
              rowTestId="score-row-a"
              scoreTestId="score-a"
              potentialTestId="score-a-potential"
              team="teamA"
              teamGradient={teamAGradient}
              score={teamAScore}
              potential={teamAHandPotential}
              tricks={teamATricks}
              matchTarget={matchTarget}
            />
          </>
        )}

        {/* Float-up bonus animation */}
        {showBonus !== null && bonusTeamString !== null && bonusGradient !== null && (
          <div
            className={`absolute -top-2 right-3 font-display text-sm font-bold ${
              prefersReducedMotion
                ? ""
                : "motion-safe:animate-out motion-safe:fade-out motion-safe:slide-out-to-top-6 motion-safe:duration-1200 motion-safe:ease-out motion-safe:fill-mode-forwards"
            } pointer-events-none`}
            style={{ color: bonusGradient[0] }}
            data-testid="score-bonus"
            data-team={bonusTeamString}
          >
            +{showBonus.amount}
          </div>
        )}
      </div>
    </>
  );
}
