import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { useSfxOnce } from "@/shared/audio/useSfxOnce";
import { Badge } from "@/shared/components/ui/badge";
import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { playerDisplayName } from "@/shared/lib/botName";
import { Z } from "@/shared/lib/zLayers";
import type { PlayerState, Rank, Suit } from "@/shared/types/matchTypes";

import { seatTeam, teamColors } from "../lib/tableTheme";
import { AutoCloseRing } from "./overlay/AutoCloseRing";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { PlayingCard } from "./PlayingCard";

interface BelotRevealProps {
  playerSeat: number;
  myPlayerSeat: number;
  cardId: string;
  isKing: boolean;
  onComplete: () => void;
  players?: readonly PlayerState[];
  soundKey?: string | null;
}

function parseCardId(id: string) {
  return { rank: id[0] as Rank, suit: id[1] as Suit };
}

/** Belot / Re-belot announcement toast shown to every player, glowing in the announcer's team color. */
export function BelotReveal({
  playerSeat,
  myPlayerSeat,
  cardId,
  isKing,
  onComplete,
  players,
  soundKey,
}: BelotRevealProps) {
  const { t } = useTranslation();
  useSfxOnce("declaration", soundKey);
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
  const announcer = playerDisplayName(t, players?.find((p) => p.seat === playerSeat) ?? null);
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
      <div
        className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 pointer-events-auto"
        data-team={team}
      >
        <ClassicPanel size="notice" glowColor={glowColor} className="relative text-center">
          <div className="absolute top-3 right-3 z-3">
            <AutoCloseRing
              duration={prefersReducedMotion ? 1.5 : 8}
              onClose={handleClose}
              ariaLabel={t("match.belot.reveal.dismiss", { defaultValue: "Dismiss" })}
              testId="belot-reveal-close"
            />
          </div>

          <div
            className="text-2xs uppercase tracking-caps"
            style={{
              color: "var(--brass)",
              padding: "0 18px",
              marginBottom: 14,
            }}
            data-testid="belot-reveal-label"
          >
            {t(labelKey)}
          </div>

          <div className="relative mb-4 inline-block">
            <div
              aria-hidden
              className="absolute rounded-lg"
              style={{ inset: -10, boxShadow: `0 0 32px ${glowColor}55`, zIndex: 0 }}
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

          <div
            className="font-semibold leading-snug"
            style={{ fontSize: 18 }}
            data-testid="belot-reveal-title"
            data-seat={playerSeat}
          >
            {titleText}
          </div>

          <div className="mt-3 flex justify-center">
            <Badge
              size="sm"
              tint={glowColor}
              icon={<span aria-hidden className="size-1.25 rounded-full bg-current" />}
            >
              {teamLabel}
            </Badge>
          </div>

          <div className="mt-1.5" style={{ fontSize: 12, opacity: 0.7 }}>
            {t(team === "gold" ? "match.belot.reveal.bonusUs" : "match.belot.reveal.bonusThem")}
          </div>
        </ClassicPanel>
      </div>
    </div>
  );
}
