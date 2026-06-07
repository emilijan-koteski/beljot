import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { Z } from "@/shared/lib/zLayers";
import type { PlayerState, Rank, Suit } from "@/shared/types/matchTypes";

import { seatTeam, teamColors } from "../lib/tableTheme";
import { AutoCloseRing } from "./overlay/AutoCloseRing";
import { PlayingCard } from "./PlayingCard";

interface BelotRevealProps {
  playerSeat: number;
  myPlayerSeat: number;
  cardId: string;
  isKing: boolean;
  onComplete: () => void;
  /** Optional player roster — when provided the announcer's name appears in
   *  the eyebrow line. Tests render the reveal without players, in which
   *  case we just show the team chip. */
  players?: readonly PlayerState[];
}

function parseCardId(id: string) {
  return { rank: id[0] as Rank, suit: id[1] as Suit };
}

/**
 * Belot / Re-belot announcement toast — shown to every player once the
 * trump-K-Q holder elects to announce. Mirrors {@link TrumpReveal}: centred
 * over the table, glows in the announcer's viewer-relative team color, auto-
 * closes after 8 s, can be dismissed early via the X-with-countdown-ring.
 */
export function BelotReveal({
  playerSeat,
  myPlayerSeat,
  cardId,
  isKing,
  onComplete,
  players,
}: BelotRevealProps) {
  const { t } = useTranslation();
  const [visible, setVisible] = useState(true);

  const prefersReducedMotion = useReducedMotion();

  const onCompleteRef = useRef(onComplete);
  useEffect(() => {
    onCompleteRef.current = onComplete;
  }, [onComplete]);

  const handleClose = () => {
    if (!visible) return;
    setVisible(false);
    onCompleteRef.current();
  };

  if (!visible) {
    return null;
  }

  const team = seatTeam(playerSeat, myPlayerSeat);
  const teamGradient = teamColors(team);
  const glowColor = teamGradient[0];
  const teamLabel = t(team === "gold" ? "team.us" : "team.them");

  const labelKey = isKing ? "match.belot.reveal.rebelot" : "match.belot.reveal.belot";
  const titleKey = isKing ? "match.belot.reveal.titleRebelot" : "match.belot.reveal.titleBelot";
  const announcer = players?.find((p) => p.seat === playerSeat)?.username;
  // Mirror TrumpReveal: full sentence on the title row when we know who
  // announced ("{{name}} announced re-belot"), graceful fallback to the
  // team label when the players roster wasn't passed (test renders).
  const titleText = announcer
    ? t(titleKey, { name: announcer })
    : t(team === "gold" ? "team.us" : "team.them");

  return (
    <div
      className={`absolute inset-0 pointer-events-none ${
        prefersReducedMotion
          ? ""
          : "motion-safe:animate-in motion-safe:fade-in motion-safe:zoom-in-95 motion-safe:duration-200"
      }`}
      style={{ zIndex: Z.REVEAL }}
      data-testid="belot-reveal"
    >
      {/* Vertical, fixed-width centred panel mirroring TrumpReveal so the two
          announcements read as one set — X in the top-right corner, content
          stacked and centred. One width serves phone + desktop (the title
          wraps); no horizontal row that crams the chip + bonus on small
          screens. */}
      <div
        className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 pointer-events-auto rounded-[18px] text-center max-w-[calc(100vw-1.5rem)]"
        style={{
          width: 264,
          padding: "20px 24px 22px",
          background: "linear-gradient(180deg, rgba(32,64,43,0.98) 0%, rgba(13,38,23,0.98) 100%)",
          border: "1px solid rgba(201,168,118,0.55)",
          boxShadow: `0 22px 54px rgba(0,0,0,0.62), 0 0 0 2px ${glowColor}66, 0 0 28px ${glowColor}55, inset 0 1px 0 rgba(201,168,118,0.25)`,
          color: "var(--ink-light, #f5f2e8)",
          fontFamily: "var(--font-body)",
        }}
        data-team={team}
      >
        {/* close button with countdown ring — top-right (matches TrumpReveal) */}
        <div className="absolute top-3 right-3 z-3">
          <AutoCloseRing
            duration={prefersReducedMotion ? 1.5 : 8}
            onClose={handleClose}
            ariaLabel={t("match.belot.reveal.dismiss", { defaultValue: "Dismiss" })}
            testId="belot-reveal-close"
          />
        </div>

        {/* eyebrow label (Belote / Rebelote) — padded clear of the close button */}
        <div
          className="text-[9.5px] uppercase tracking-[0.22em]"
          style={{
            color: "var(--brass, #c9a876)",
            fontFamily: "var(--font-body)",
            padding: "0 18px",
            marginBottom: 14,
          }}
          data-testid="belot-reveal-label"
        >
          {t(labelKey)}
        </div>

        {/* hero K/Q card with a soft team-coloured glow */}
        <div className="relative mb-4 inline-block">
          <div
            aria-hidden
            className="absolute rounded-lg"
            style={{ inset: -10, boxShadow: `0 0 30px ${glowColor}55`, zIndex: 0 }}
          />
          <div className="relative z-1">
            <PlayingCard
              card={parseCardId(cardId)}
              state="default"
              size="lg"
              withTransition={false}
            />
          </div>
        </div>

        {/* who announced it */}
        <div
          className="font-semibold leading-snug"
          style={{ fontFamily: "var(--font-body)", fontSize: 18, letterSpacing: 0.2 }}
          data-testid="belot-reveal-title"
          data-seat={playerSeat}
        >
          {titleText}
        </div>

        {/* team chip */}
        <div className="mt-3 flex justify-center">
          <span
            className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[9.5px] font-bold uppercase tracking-wider"
            style={{
              background: `${glowColor}22`,
              border: `1px solid ${glowColor}88`,
              color: glowColor,
            }}
          >
            <span
              aria-hidden
              className="rounded-full"
              style={{ width: 5, height: 5, background: glowColor }}
            />
            {teamLabel}
          </span>
        </div>

        {/* +20 bonus — its own line so it never crowds the chip */}
        <div className="mt-1.5" style={{ fontSize: 11.5, opacity: 0.7 }}>
          {t("match.belot.reveal.bonus", { defaultValue: "+20 to your team's score" })}
        </div>
      </div>
    </div>
  );
}
