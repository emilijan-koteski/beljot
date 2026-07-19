import { type CSSProperties, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { MOTION } from "@/shared/lib/motion";
import { Z } from "@/shared/lib/zLayers";
import { type Suit, type TeamString, teamStringForIndex } from "@/shared/types/matchTypes";
import type { HandScoredPayload } from "@/shared/types/wsEvents";

import { TEAM_GOLD, TEAM_SILVER, type TeamGradient } from "../lib/tableTheme";
import { ringDrainStyle } from "../lib/turnCountdown";
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
  /** Seat that called the trump (used for "you took trump on X" wording). */
  trumpCallerSeat?: number | null;
  /**
   * True once the local player has clicked Continue and the server is waiting
   * for the other players (server-gated hand-complete pause). Swaps the button
   * to a disabled "waiting" label until the next hand is dealt.
   */
  acknowledged?: boolean;
  /** Match target shown in the brass strip — defaults to the full 1001 race. */
  matchTarget?: number;
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

const AUTO_CONTINUE_MS = MOTION.SCORE_REVEAL_AUTO_CONTINUE;

// Continue button corner radius — must equal ClassicButton's borderRadius (8)
// so the traced border curves exactly like the gold button edge. The rect is
// inset from the edge, so its corner radius is BUTTON_RADIUS - inset to stay
// concentric (a rounded rect inset by d has corner radius R - d).
const BUTTON_RADIUS = 8;

/**
 * Countdown border that traces the Continue button's perimeter, sweeping full →
 * empty over the auto-continue window — the "loader around the button" the
 * informational reveals use (an AutoCloseRing's ring around its X), adapted to a
 * wide button. Two strokes mirror AutoCloseRing: a faint full-perimeter track
 * (the path the loader has already swept past) and the silver progress on top
 * that retracts. Colours match AutoCloseRing so it reads identically against the
 * green (ghost) button. Purely decorative; the timer in ScoreReveal owns the
 * actual fire.
 *
 * Measures the button via the SVG's own rect (it fills the relatively-positioned
 * wrapper) so the viewBox maps 1 user-unit → 1 px and the rounded corners stay
 * true. `pathLength={1}` normalises the dash math regardless of the perimeter.
 *
 * `sweep` carries the ring-drain animation style (mount-anchored, same
 * duration as the auto-continue fire timer) so the border reads empty exactly
 * when the auto-ack fires; undefined renders a static full border
 * (reduced motion).
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
          {/* Faint full-perimeter track — the swept-past path. */}
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
          {/* Silver progress that retracts over the track (matches AutoCloseRing). */}
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
 * End-of-hand score reveal — felt-panel breakdown of card points + decls
 * + bonuses, ending in a brass-tinted match-score strip.
 *
 * Continue is enabled from the start. A countdown border around the button runs
 * the auto-continue window: if the player never clicks, the dialog acknowledges
 * itself (so one AFK player can't strand the table on the score screen). Once
 * acknowledged the button swaps to a disabled "waiting" label until the server
 * deals the next hand.
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
}: ScoreRevealProps) {
  const { t } = useTranslation();
  const prefersReducedMotion = useReducedMotion();

  // Auto-continue: if the player never clicks, the dialog acknowledges itself
  // after AUTO_CONTINUE_MS so an AFK player can't strand the whole table on the
  // score screen (each client auto-acks → the server deals the next hand once
  // everyone is ready). Mirrors the 8 s auto-close on the informational
  // reveals; the border around the button visualises the countdown. Once
  // acknowledged (this click, or the parent already advanced) the timer is
  // cancelled — see the `acknowledged` dependency.
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

  // Mount-anchored drain sweep for the button border: starts on the same
  // commit as the fire timer above and shares its duration, so the border
  // reads empty exactly when the auto-ack fires.
  const borderSweep = useMemo(() => ringDrainStyle(AUTO_CONTINUE_MS, AUTO_CONTINUE_MS, 1), []);

  // Reduced-motion fallback: no drain animation, but the border must still
  // reflect the countdown — step the dashoffset once per second, anchored to
  // the same mount instant as the fire timer.
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

  // Title + outcome subtitle per design ("Hand N — results / Pulled it off ·
  // you took trump on Spades"). When handNumber / trumpSuit aren't passed
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
  // · you took trump on Spades"; went down = "Went down · all points to us/them".
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
              className="rounded-lg px-3 py-2.5 sm:px-4 sm:py-3 mt-4 flex flex-wrap items-center justify-between gap-3 sm:gap-4"
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
                {/* nowrap: the "/ 1001" target must never split off onto its own
                    line when the Continue button squeezes the strip on phones. */}
                <div className="flex items-baseline gap-1.5 sm:gap-2 font-display text-[16px] sm:text-[18px] font-bold tabular-nums whitespace-nowrap">
                  <span style={{ color: teamAGradient[0] }} data-team="teamA">
                    {data.teamAMatchScore}
                  </span>
                  <span style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.4 }}>·</span>
                  <span style={{ color: teamBGradient[0] }} data-team="teamB">
                    {data.teamBMatchScore}
                  </span>
                  <span
                    className="text-[11px] sm:text-[12px] font-body font-normal"
                    style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.5 }}
                  >
                    / {matchTarget}
                  </span>
                </div>
              </div>
              {/* ml-auto keeps the CTA pinned right if the strip ever wraps on
                  ultra-narrow screens (justify-between alone would drop a lone
                  wrapped item to the left edge). */}
              <span className="relative inline-flex ml-auto">
                <ClassicButton
                  variant="ghost"
                  onClick={onContinue}
                  disabled={acknowledged}
                  data-testid="score-reveal-continue"
                >
                  {acknowledged ? t("match.scoreReveal.waiting") : t("match.scoreReveal.continue")}
                </ClassicButton>
                {/* Loader traces the button's border (not an inline icon) —
                    matches the ring-around-the-control pattern of the other
                    reveals. Hidden once acknowledged (then we're just waiting). */}
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
        className="font-body text-[13px] sm:text-[14px]"
        style={{
          color: "var(--ink-light, #f5f2e8)",
          opacity: bold ? 1 : 0.85,
          fontWeight: bold ? 700 : 400,
        }}
      >
        {label}
      </span>
      <span
        className={`font-display text-right tabular-nums ${bold ? "text-[16px] sm:text-[18px]" : "text-[14px] sm:text-[15px]"}`}
        style={{
          color: "var(--ink-light, #f5f2e8)",
          fontWeight: bold ? 700 : 500,
        }}
        data-team="teamA"
      >
        {teamAValue}
      </span>
      <span
        className={`font-display text-right tabular-nums ${bold ? "text-[16px] sm:text-[18px]" : "text-[14px] sm:text-[15px]"}`}
        style={{
          color: "var(--ink-light, #f5f2e8)",
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
  // Same grid as ScoreRow so the bonus lands under the earning team's column
  // (team A = left, team B = right) instead of always hugging the right edge.
  const value = (
    <span
      className="font-display text-right tabular-nums text-sm sm:text-[15px]"
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
        color: "var(--ink-light, #f5f2e8)",
        opacity: 0.85,
      }}
      data-testid={testId}
    >
      <span className="font-body text-[13px] sm:text-[14px]">{label}</span>
      {team === 0 ? value : <span />}
      {team === 1 ? value : <span />}
    </div>
  );
}
