import { Bot } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { useSfxOnce } from "@/shared/audio/useSfxOnce";
import { Avatar } from "@/shared/components/ui/avatar";
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
  isRebelote: boolean;
  onComplete: () => void;
  players?: readonly PlayerState[];
  soundKey?: string | null;
}

const BELOT_BONUS = 20;

function parseCardId(id: string) {
  return { rank: id[0] as Rank, suit: id[1] as Suit };
}

/** Belot / Re-belot announcement toast shown to every player, glowing in the announcer's team color. */
export function BelotReveal({
  playerSeat,
  myPlayerSeat,
  cardId,
  isRebelote,
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

  const labelKey = isRebelote ? "match.belot.reveal.rebelot" : "match.belot.reveal.belot";
  const announcerPlayer = players?.find((p) => p.seat === playerSeat) ?? null;
  const announcer = playerDisplayName(t, announcerPlayer);
  const avatarTeam = team === "gold" ? "A" : "B";

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
              marginBottom: 16,
            }}
            data-testid="belot-reveal-label"
          >
            {t(labelKey)}
          </div>

          <div className="relative mb-4.5 inline-block">
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
            className="flex items-center justify-center gap-2"
            style={{ fontSize: 20, fontWeight: 600 }}
            data-testid="belot-reveal-title"
            data-seat={playerSeat}
          >
            {announcer !== null && announcerPlayer ? (
              <>
                <Avatar
                  name={announcer}
                  avatarUrl={announcerPlayer.isBot === true ? null : announcerPlayer.avatarUrl}
                  team={avatarTeam}
                  size={24}
                  icon={announcerPlayer.isBot === true ? <Bot aria-hidden="true" /> : undefined}
                />
                <span>{announcer}</span>
              </>
            ) : (
              <span>{teamLabel}</span>
            )}
          </div>

          <div
            className="mt-1 text-sm leading-snug font-semibold tabular-nums text-(--felt-cream)"
            data-testid="belot-reveal-bonus"
          >
            +{BELOT_BONUS}
          </div>

          <div className="mt-3.5 flex justify-center">
            <Badge
              size="sm"
              tint={glowColor}
              icon={<span aria-hidden className="size-1.25 rounded-full bg-current" />}
            >
              {teamLabel}
            </Badge>
          </div>
        </ClassicPanel>
      </div>
    </div>
  );
}
