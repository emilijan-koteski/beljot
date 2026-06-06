import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { MOTION, motionDuration } from "@/shared/lib/motion";
import { type Suit, type TeamString, teamStringForIndex } from "@/shared/types/matchTypes";
import type { HandScoredPayload } from "@/shared/types/wsEvents";

import { TEAM_GOLD, TEAM_SILVER, type TeamGradient } from "../lib/tableTheme";
import { ClassicButton } from "./overlay/ClassicButton";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";

interface ScoreRevealProps {
  data: HandScoredPayload;
  viewerTeam: TeamString;
  onContinue: () => void;
  /** Hand number (1-based) used in the title — "Hand N — results". */
  handNumber?: number;
  /** Trump suit for the just-finished hand (used in the subtitle). */
  trumpSuit?: Suit | null;
  /** Seat that called the trump (used for "your team called X" wording). */
  trumpCallerSeat?: number | null;
}

const SUIT_NAME_KEY: Record<Suit, string> = {
  S: "match.suits.spades",
  H: "match.suits.hearts",
  D: "match.suits.diamonds",
  C: "match.suits.clubs",
};

function callerTeamString(seat: number): TeamString {
  return seat % 2 === 0 ? "teamA" : "teamB";
}

// Re-exported constants for tests / reads — see `MOTION` for canonical values.
const ENABLE_DELAY_MS = MOTION.SCORE_REVEAL_ENABLE_DELAY;
const ENABLE_DELAY_MS_REDUCED = MOTION.SCORE_REVEAL_ENABLE_DELAY_REDUCED;

/**
 * End-of-hand score reveal — felt-panel breakdown of card points + decls
 * + bonuses, ending in a brass-tinted match-score strip.
 *
 * Continue is disabled for the first ~2 s so the player has time to read the
 * breakdown, then enabled. The reveal stays up indefinitely until the player
 * clicks Continue — same dismissal model as MatchResult, so an AFK player
 * doesn't miss their own hand recap.
 */
export function ScoreReveal({
  data,
  viewerTeam,
  onContinue,
  handNumber,
  trumpSuit,
  trumpCallerSeat,
}: ScoreRevealProps) {
  const { t } = useTranslation();
  const [continueEnabled, setContinueEnabled] = useState(false);
  const prefersReducedMotion = useReducedMotion();

  const enableDelay = motionDuration(
    prefersReducedMotion,
    ENABLE_DELAY_MS,
    ENABLE_DELAY_MS_REDUCED,
  );

  useEffect(() => {
    const t = setTimeout(() => setContinueEnabled(true), enableDelay);
    return () => clearTimeout(t);
  }, [enableDelay]);

  const teamAGradient: TeamGradient = viewerTeam === "teamA" ? TEAM_GOLD : TEAM_SILVER;
  const teamBGradient: TeamGradient = viewerTeam === "teamB" ? TEAM_GOLD : TEAM_SILVER;

  const hasDeclarations = data.teamADeclPoints > 0 || data.teamBDeclPoints > 0;

  // Title + contract subtitle per design ("Hand N — results / Contract held ·
  // your team called Spades"). When handNumber / trumpSuit aren't passed
  // (legacy callers, test renders) we fall back to a plain "Hand Score".
  const title =
    handNumber !== undefined
      ? t("match.scoreReveal.titleWithHand", {
          hand: handNumber,
          defaultValue: `Hand ${handNumber} — results`,
        })
      : t("match.scoreReveal.title");

  const callerTeam: TeamString | null =
    typeof trumpCallerSeat === "number" ? callerTeamString(trumpCallerSeat) : null;
  const callerWasViewer = callerTeam !== null && callerTeam === viewerTeam;
  const trumpSuitName = trumpSuit ? t(SUIT_NAME_KEY[trumpSuit]) : null;

  // Subtitle carries the hand-outcome callout per design. When the taker's
  // team went down, it names who collects the points — split into Us/Them
  // keys (mirroring capot.bonusUs/bonusThem) rather than interpolating the
  // team LABEL, because "all points to {{team}}" reads ungrammatically after
  // the preposition in mk/hr/sr: the nominative label ("за Тие" / "za Oni")
  // must be the oblique pronoun ("за нив" / "za njih"). Held = "Pulled it off
  // · your team took Spades"; went down = "Went down · all points to us/them".
  let subtitle: string | null = null;
  if (data.failedContract) {
    const beneficiary: TeamString = data.contractingTeam === 0 ? "teamB" : "teamA";
    subtitle = t(
      beneficiary === viewerTeam
        ? "match.scoreReveal.subtitleFailedUs"
        : "match.scoreReveal.subtitleFailedThem",
    );
  } else if (trumpSuitName && callerTeam) {
    subtitle = t(
      callerWasViewer
        ? "match.scoreReveal.subtitleHeldYour"
        : "match.scoreReveal.subtitleHeldTheir",
      { suit: trumpSuitName },
    );
  }

  return (
    <div className="fixed inset-0 z-30" data-testid="score-reveal">
      <OverlayBackdrop dim={0.6}>
        <div
          className={
            prefersReducedMotion
              ? ""
              : "motion-safe:animate-in motion-safe:zoom-in-95 motion-safe:fade-in motion-safe:duration-300"
          }
        >
          <ClassicPanel
            width={560}
            title={<span data-testid="score-reveal-title">{title}</span>}
            subtitle={subtitle ?? undefined}
          >
            {/* Header eyebrow row — Us / Them column labels */}
            <div
              className="grid items-end mb-2"
              style={{ gridTemplateColumns: "1fr 80px 80px", gap: 8 }}
            >
              <span />
              <span
                className="font-display text-[11px] font-bold uppercase tracking-wider text-right"
                style={{ color: teamAGradient[0] }}
              >
                {viewerTeam === "teamA" ? t("team.us") : t("team.them")}
              </span>
              <span
                className="font-display text-[11px] font-bold uppercase tracking-wider text-right"
                style={{ color: teamBGradient[0] }}
              >
                {viewerTeam === "teamB" ? t("team.us") : t("team.them")}
              </span>
            </div>

            <div className="flex flex-col">
              <ScoreRow
                label={t("match.scoreReveal.cardPoints")}
                teamAValue={data.teamACardPoints}
                teamBValue={data.teamBCardPoints}
                testId="row-card-points"
              />
              {hasDeclarations && (
                <ScoreRow
                  label={t("match.scoreReveal.declarationPoints")}
                  teamAValue={data.teamADeclPoints}
                  teamBValue={data.teamBDeclPoints}
                  testId="row-decl-points"
                />
              )}
              {data.lastTrickBonus > 0 && (
                <BonusRow
                  label={t("match.scoreReveal.lastTrickBonus")}
                  amount={data.lastTrickBonus}
                  team={data.lastTrickTeam === 0 ? 0 : 1}
                  teamGradient={data.lastTrickTeam === 0 ? teamAGradient : teamBGradient}
                  testId="row-last-trick"
                />
              )}
              {data.capot && data.capotTeam !== null && (
                <BonusRow
                  label={t("match.scoreReveal.capotBonus")}
                  amount={data.capotBonus}
                  team={data.capotTeam === 0 ? 0 : 1}
                  teamGradient={data.capotTeam === 0 ? teamAGradient : teamBGradient}
                  testId="row-capot-bonus"
                />
              )}
              <ScoreRow
                label={t("match.scoreReveal.handTotal")}
                teamAValue={data.teamAHandTotal}
                teamBValue={data.teamBHandTotal}
                testId="row-hand-total"
                bold
                topBorder
              />
            </div>

            {/* Match-score brass strip — per design, the strip hosts the
                "Match score" eyebrow + the {Us · Them / 1001} totals on the
                left and the Continue button on the right. Combining them
                keeps the dialog footprint tight. */}
            <div
              className="rounded-lg px-4 py-3 mt-4 flex items-center justify-between gap-4"
              style={{
                background: "rgba(201,168,118,0.1)",
                border: "1px solid rgba(201,168,118,0.3)",
              }}
              data-testid="row-match-total"
            >
              <div className="flex flex-col gap-0.5">
                <span
                  className="font-body text-[10.5px] uppercase tracking-widest"
                  style={{ color: "var(--brass, #c9a876)" }}
                >
                  {t("match.scoreReveal.matchTotal")}
                </span>
                <div className="flex items-baseline gap-2 font-display text-lg font-bold tabular-nums">
                  <span style={{ color: teamAGradient[0] }} data-team="teamA">
                    {data.teamAMatchScore}
                  </span>
                  <span style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.4 }}>·</span>
                  <span style={{ color: teamBGradient[0] }} data-team="teamB">
                    {data.teamBMatchScore}
                  </span>
                  <span
                    className="text-[12px] font-body font-normal"
                    style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.5 }}
                  >
                    / 1001
                  </span>
                </div>
              </div>
              <ClassicButton
                variant="primary"
                onClick={onContinue}
                disabled={!continueEnabled}
                data-testid="score-reveal-continue"
              >
                {t("match.scoreReveal.continue")}
              </ClassicButton>
            </div>
          </ClassicPanel>
        </div>
      </OverlayBackdrop>
    </div>
  );
}

interface ScoreRowProps {
  label: string;
  teamAValue: number;
  teamBValue: number;
  bold?: boolean;
  topBorder?: boolean;
  testId: string;
}

function ScoreRow({ label, teamAValue, teamBValue, bold, topBorder, testId }: ScoreRowProps) {
  return (
    <div
      className="grid items-center py-2"
      style={{
        gridTemplateColumns: "1fr 80px 80px",
        gap: 8,
        borderTop: topBorder ? "1px solid rgba(255,255,255,0.06)" : undefined,
      }}
      data-testid={testId}
    >
      <span
        className="font-body text-sm"
        style={{
          color: "var(--ink-light, #f5f2e8)",
          opacity: bold ? 1 : 0.85,
          fontWeight: bold ? 700 : 400,
        }}
      >
        {label}
      </span>
      <span
        className="font-display text-right tabular-nums"
        style={{
          color: "var(--ink-light, #f5f2e8)",
          fontSize: bold ? 18 : 15,
          fontWeight: bold ? 700 : 500,
        }}
        data-team="teamA"
      >
        {teamAValue}
      </span>
      <span
        className="font-display text-right tabular-nums"
        style={{
          color: "var(--ink-light, #f5f2e8)",
          fontSize: bold ? 18 : 15,
          fontWeight: bold ? 700 : 500,
        }}
        data-team="teamB"
      >
        {teamBValue}
      </span>
    </div>
  );
}

interface BonusRowProps {
  label: string;
  amount: number;
  team: 0 | 1;
  teamGradient: TeamGradient;
  testId: string;
}

function BonusRow({ label, amount, team, teamGradient, testId }: BonusRowProps) {
  const teamString = teamStringForIndex(team);
  return (
    <div
      className="flex items-center justify-between py-1.5"
      data-testid={testId}
      style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.85 }}
    >
      <span className="font-body text-sm">{label}</span>
      <span
        className="font-display text-base font-semibold tabular-nums"
        style={{ color: teamGradient[0] }}
        data-team={teamString}
      >
        +{amount}
      </span>
    </div>
  );
}
