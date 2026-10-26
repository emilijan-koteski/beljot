import { type CSSProperties, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { MOTION } from "@/shared/lib/motion";
import { Z } from "@/shared/lib/zLayers";
import { useAuthStore } from "@/shared/stores/authStore";
import { type Suit, type TeamString, teamStringForIndex } from "@/shared/types/matchTypes";
import type { HandScoredPayload } from "@/shared/types/wsEvents";

import { resolveCardDeck } from "../lib/cardFace";
import { suitNameKey } from "../lib/suitArt";
import { TEAM_GOLD, TEAM_SILVER, type TeamGradient } from "../lib/tableTheme";
import { ringDrainStyle } from "../lib/turnCountdown";
import { ClassicButton } from "./overlay/ClassicButton";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";

interface ScoreRevealProps {
  data: HandScoredPayload;
  viewerTeam: TeamString;
  onContinue: () => void;
  handNumber?: number;
  trumpSuit?: Suit | null;
  trumpCallerSeat?: number | null;
  acknowledged?: boolean;
  matchTarget?: number;
  declarationsTransfer?: boolean;
}

function callerTeamString(seat: number): TeamString {
  return seat % 2 === 0 ? "teamA" : "teamB";
}

/** Picks the fall subtitle: every point goes to the winners, or only the card points when declarations do not transfer. */
function failedSubtitleKey(toViewer: boolean, declarationsTransfer: boolean): string {
  if (declarationsTransfer) {
    return toViewer ? "match.scoreReveal.subtitleFailedUs" : "match.scoreReveal.subtitleFailedThem";
  }
  return toViewer
    ? "match.scoreReveal.subtitleFailedCardsUs"
    : "match.scoreReveal.subtitleFailedCardsThem";
}

const AUTO_CONTINUE_MS = MOTION.SCORE_REVEAL_AUTO_CONTINUE;

const BUTTON_RADIUS = 8;

/**
 * Decorative countdown border that drains around the Continue button over the auto-continue window.
 * `sweep` carries the drain animation style; undefined renders a static full border for reduced motion.
 */
function ButtonProgressBorder({ sweep }: { sweep?: CSSProperties }) {
  const svgRef = useRef<SVGSVGElement | null>(null);
  const [size, setSize] = useState({ w: 0, h: 0 });

  useLayoutEffect(() => {
    const el = svgRef.current;
    if (!el) return;
    const measure = () => {
      const r = el.getBoundingClientRect();
      setSize({ w: r.width, h: r.height });
    };
    measure();
    if (typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const { w, h } = size;
  const inset = 1.5;

  return (
    <svg
      ref={svgRef}
      aria-hidden
      data-testid="score-reveal-countdown"
      viewBox={w && h ? `0 0 ${w} ${h}` : undefined}
      style={{
        position: "absolute",
        inset: 0,
        width: "100%",
        height: "100%",
        pointerEvents: "none",
        overflow: "visible",
      }}
    >
      {w > 0 && h > 0 && (
        <>
          <rect
            x={inset}
            y={inset}
            width={w - inset * 2}
            height={h - inset * 2}
            rx={BUTTON_RADIUS - inset}
            ry={BUTTON_RADIUS - inset}
            fill="none"
            stroke="rgba(255,255,255,0.1)"
            strokeWidth={2}
          />
          <rect
            x={inset}
            y={inset}
            width={w - inset * 2}
            height={h - inset * 2}
            rx={BUTTON_RADIUS - inset}
            ry={BUTTON_RADIUS - inset}
            fill="none"
            stroke="#d4d0c4"
            strokeWidth={2}
            strokeLinecap="round"
            pathLength={1}
            strokeDasharray={1}
            style={{ strokeDashoffset: 0, ...sweep }}
          />
        </>
      )}
    </svg>
  );
}

/**
 * End-of-hand score reveal breaking down card points, declarations and bonuses above the match score.
 * A countdown border on Continue auto-acknowledges the dialog so an AFK player cannot stall the table.
 */
export function ScoreReveal({
  data,
  viewerTeam,
  onContinue,
  handNumber,
  trumpSuit,
  trumpCallerSeat,
  acknowledged = false,
  matchTarget = 1001,
  declarationsTransfer = true,
}: ScoreRevealProps) {
  const { t } = useTranslation();
  const prefersReducedMotion = useReducedMotion();

  const firedRef = useRef(false);
  const onContinueRef = useRef(onContinue);
  onContinueRef.current = onContinue;

  useEffect(() => {
    if (acknowledged) return;
    const fire = setTimeout(() => {
      if (firedRef.current) return;
      firedRef.current = true;
      onContinueRef.current();
    }, AUTO_CONTINUE_MS);
    return () => clearTimeout(fire);
  }, [acknowledged]);

  const borderSweep = useMemo(() => ringDrainStyle(AUTO_CONTINUE_MS, AUTO_CONTINUE_MS, 1), []);

  const [reducedPct, setReducedPct] = useState(1);
  useEffect(() => {
    if (!prefersReducedMotion || acknowledged) return;
    const deadline = Date.now() + AUTO_CONTINUE_MS;
    const update = () => setReducedPct(Math.max(0, (deadline - Date.now()) / AUTO_CONTINUE_MS));
    update();
    const id = setInterval(update, 1000);
    return () => clearInterval(id);
  }, [prefersReducedMotion, acknowledged]);
  const reducedSweep: CSSProperties = { strokeDashoffset: 1 - reducedPct };

  const teamAGradient: TeamGradient = viewerTeam === "teamA" ? TEAM_GOLD : TEAM_SILVER;
  const teamBGradient: TeamGradient = viewerTeam === "teamB" ? TEAM_GOLD : TEAM_SILVER;

  const hasDeclarations = data.teamADeclPoints > 0 || data.teamBDeclPoints > 0;

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
  const deck = resolveCardDeck(useAuthStore((s) => s.user?.cardDeckPreference));
  const trumpSuitName = trumpSuit ? t(suitNameKey(trumpSuit, deck)) : null;

  let subtitle: string | null = null;
  if (data.failedContract) {
    const beneficiary: TeamString = data.contractingTeam === 0 ? "teamB" : "teamA";
    subtitle = t(failedSubtitleKey(beneficiary === viewerTeam, declarationsTransfer));
  } else if (trumpSuitName && callerTeam) {
    subtitle = t(
      callerWasViewer
        ? "match.scoreReveal.subtitleHeldYour"
        : "match.scoreReveal.subtitleHeldTheir",
      { suit: trumpSuitName },
    );
  }

  return (
    <div className="fixed inset-0" style={{ zIndex: Z.PROMPT }} data-testid="score-reveal">
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
            <div
              className="grid items-end mb-2"
              style={{ gridTemplateColumns: "1fr 80px 80px", gap: 8 }}
            >
              <span />
              <span
                className="text-2xs font-bold uppercase tracking-wider text-right"
                style={{ color: teamAGradient[0] }}
              >
                {viewerTeam === "teamA" ? t("team.us") : t("team.them")}
              </span>
              <span
                className="text-2xs font-bold uppercase tracking-wider text-right"
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

            <div
              className="rounded-lg px-3 py-2.5 sm:px-4 sm:py-3 mt-4 flex flex-wrap items-center justify-between gap-3 sm:gap-4"
              style={{
                background: "var(--brass-10)",
                border: "1px solid var(--brass-25)",
              }}
              data-testid="row-match-total"
            >
              <div className="flex flex-col gap-0.5">
                <span
                  className="text-2xs uppercase tracking-widest"
                  style={{ color: "var(--brass)" }}
                >
                  {t("match.scoreReveal.matchTotal")}
                </span>
                <div className="flex items-baseline gap-1.5 sm:gap-2 text-base sm:text-lg font-bold tabular-nums whitespace-nowrap">
                  <span style={{ color: teamAGradient[0] }} data-team="teamA">
                    {data.teamAMatchScore}
                  </span>
                  <span style={{ color: "var(--ink-light)", opacity: 0.4 }}>·</span>
                  <span style={{ color: teamBGradient[0] }} data-team="teamB">
                    {data.teamBMatchScore}
                  </span>
                  <span
                    className="text-xs sm:text-xs font-normal"
                    style={{ color: "var(--ink-light)", opacity: 0.5 }}
                  >
                    / {matchTarget}
                  </span>
                </div>
              </div>
              <span className="relative inline-flex ml-auto">
                <ClassicButton
                  variant="ghost"
                  onClick={onContinue}
                  disabled={acknowledged}
                  data-testid="score-reveal-continue"
                >
                  {acknowledged ? t("match.scoreReveal.waiting") : t("match.scoreReveal.continue")}
                </ClassicButton>
                {!acknowledged && (
                  <ButtonProgressBorder sweep={prefersReducedMotion ? reducedSweep : borderSweep} />
                )}
              </span>
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
        className="text-sm sm:text-sm"
        style={{
          color: "var(--ink-light)",
          opacity: bold ? 1 : 0.85,
          fontWeight: bold ? 700 : 400,
        }}
      >
        {label}
      </span>
      <span
        className={`text-right tabular-nums ${bold ? "text-base sm:text-lg" : "text-sm sm:text-base"}`}
        style={{
          color: "var(--ink-light)",
          fontWeight: bold ? 700 : 500,
        }}
        data-team="teamA"
      >
        {teamAValue}
      </span>
      <span
        className={`text-right tabular-nums ${bold ? "text-base sm:text-lg" : "text-sm sm:text-base"}`}
        style={{
          color: "var(--ink-light)",
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
  const value = (
    <span
      className="text-right tabular-nums text-sm sm:text-base"
      style={{ color: teamGradient[0], fontWeight: 500 }}
      data-team={teamString}
    >
      +{amount}
    </span>
  );
  return (
    <div
      className="grid items-center py-1.5"
      style={{
        gridTemplateColumns: "1fr 80px 80px",
        gap: 8,
        color: "var(--ink-light)",
        opacity: 0.85,
      }}
      data-testid={testId}
    >
      <span className="text-sm sm:text-sm">{label}</span>
      {team === 0 ? value : <span />}
      {team === 1 ? value : <span />}
    </div>
  );
}
