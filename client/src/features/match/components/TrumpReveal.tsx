import { Bot } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Trans, useTranslation } from "react-i18next";

import { Avatar } from "@/shared/components/ui/avatar";
import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { playerDisplayName } from "@/shared/lib/botName";
import { isCardId } from "@/shared/lib/cardId";
import { Z } from "@/shared/lib/zLayers";
import { useAuthStore } from "@/shared/stores/authStore";
import type { PlayerState, Rank, Suit } from "@/shared/types/matchTypes";

import { resolveCardDeck } from "../lib/cardFace";
import { rankNameKey, suitAccent, suitInkOnFelt, suitNameKey } from "../lib/suitArt";
import { seatTeam, teamColors } from "../lib/tableTheme";
import { AutoCloseRing } from "./overlay/AutoCloseRing";
import { PlayingCard } from "./PlayingCard";
import { SuitMark } from "./SuitMark";

interface TrumpRevealProps {
  playerSeat: number;
  myPlayerSeat: number | null;
  cardId: string;
  trumpSuit: Suit;
  players: readonly PlayerState[];
  onComplete: () => void;
}

function parseCardId(id: string) {
  return { rank: id[0] as Rank, suit: id[1] as Suit };
}

/**
 * "Trump taken" wax-seal panel showing the candidate card with the chosen suit stamped on it, or the seal alone when there is no candidate.
 * Glows in the taker's viewer-relative team color and auto-closes after 8 s.
 */
export function TrumpReveal({
  playerSeat,
  myPlayerSeat,
  cardId,
  trumpSuit,
  players,
  onComplete,
}: TrumpRevealProps) {
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

  const deck = resolveCardDeck(useAuthStore((s) => s.user?.cardDeckPreference));

  if (!visible || (cardId !== "" && !isCardId(cardId))) {
    return null;
  }

  const picker = players.find((p) => p.seat === playerSeat);
  const pickerName = playerDisplayName(t, picker ?? null);
  const card = cardId === "" ? null : parseCardId(cardId);

  const isFreePick = card !== null && card.suit !== trumpSuit;

  const suitName = t(suitNameKey(trumpSuit, deck));
  const candidateRankName = card ? t(rankNameKey(card.rank)) : "";
  const candidateSuitName = card ? t(suitNameKey(card.suit, deck)) : "";

  const team = myPlayerSeat !== null ? seatTeam(playerSeat, myPlayerSeat) : null;
  const teamGradient = team ? teamColors(team) : null;
  const glowColor = teamGradient ? teamGradient[0] : "var(--brass)";
  const teamLabel = team ? t(team === "gold" ? "team.us" : "team.them") : null;
  const avatarTeam = team === "gold" ? "A" : team === "silver" ? "B" : null;

  const sealRing = suitAccent(trumpSuit, deck);
  const chosenColor = suitInkOnFelt(trumpSuit, deck);

  const eyebrow = isFreePick
    ? t("match.trumpReveal.eyebrowFreeChoice", { defaultValue: "Trump taken · free pick" })
    : t("match.trumpReveal.eyebrow", { defaultValue: "Trump taken" });

  return (
    <div
      className={`absolute inset-0 pointer-events-none ${
        prefersReducedMotion
          ? ""
          : "motion-safe:animate-in motion-safe:fade-in motion-safe:zoom-in-95 motion-safe:duration-200"
      }`}
      style={{ zIndex: Z.REVEAL }}
      data-testid="trump-reveal"
    >
      <div
        className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 pointer-events-auto rounded-xl text-center"
        style={{
          width: 262,
          padding: "20px 24px 22px",
          background: "linear-gradient(180deg, rgba(32,64,43,0.98) 0%, rgba(13,38,23,0.98) 100%)",
          border: "1px solid var(--brass-55)",
          boxShadow: `0 22px 54px rgba(0,0,0,0.62), 0 0 0 2px ${glowColor}66, 0 0 28px ${glowColor}55, inset 0 1px 0 var(--brass-25)`,
          color: "var(--ink-light)",
          fontFamily: "var(--font-body)",
        }}
        data-team={team ?? undefined}
      >
        <div className="absolute top-3 right-3 z-3">
          <AutoCloseRing
            duration={prefersReducedMotion ? 1.5 : 8}
            onClose={handleClose}
            ariaLabel={t("match.trumpReveal.dismiss", { defaultValue: "Dismiss" })}
            testId="trump-reveal-close"
          />
        </div>

        <div
          className="text-[9.5px] uppercase tracking-[0.22em]"
          style={{
            color: "var(--brass)",
            fontFamily: "var(--font-body)",
            padding: "0 18px",
            marginBottom: 16,
          }}
          data-testid="trump-reveal-eyebrow"
        >
          {eyebrow}
        </div>

        <div className="relative mb-4.5 inline-block">
          {card && (
            <>
              <div
                aria-hidden
                className="absolute rounded-lg"
                style={{ inset: -10, boxShadow: `0 0 32px ${glowColor}55`, zIndex: 0 }}
              />
              <div className="relative z-1">
                <PlayingCard card={card} state="default" size="lg" withTransition={false} />
              </div>
            </>
          )}
          <div
            className={
              card
                ? "absolute z-2 flex items-center justify-center"
                : "flex items-center justify-center"
            }
            style={{
              ...(card ? { right: -16, bottom: -14 } : {}),
              width: 60,
              height: 60,
              borderRadius: "50%",
              background: "radial-gradient(circle at 38% 30%, #fffdf6, #efe4c8 68%, #ddcca2)",
              border: `2.5px solid ${sealRing}`,
              boxShadow: card
                ? "0 6px 14px rgba(0,0,0,0.5), inset 0 2px 3px rgba(255,255,255,0.75), inset 0 -3px 6px rgba(0,0,0,0.2)"
                : `0 6px 14px rgba(0,0,0,0.5), 0 0 32px ${glowColor}66, 0 0 0 6px ${glowColor}22, inset 0 2px 3px rgba(255,255,255,0.75), inset 0 -3px 6px rgba(0,0,0,0.2)`,
            }}
            data-testid="trump-reveal-seal"
            data-suit={trumpSuit}
            aria-label={suitName}
          >
            <SuitMark
              suit={trumpSuit}
              deck={deck}
              size={30}
              shadow="0 1px 1px rgba(0,0,0,0.25)"
              style={{ fontFamily: "var(--font-suit)" }}
            />
          </div>
        </div>

        <div
          className="flex items-center justify-center gap-2"
          style={{ fontFamily: "var(--font-body)", fontSize: 20, fontWeight: 600 }}
          data-testid="trump-reveal-taker"
          data-seat={playerSeat}
        >
          {pickerName !== null && picker ? (
            <>
              <Avatar
                name={pickerName}
                avatarUrl={picker.isBot === true ? null : picker.avatarUrl}
                team={avatarTeam}
                size={24}
                icon={picker.isBot === true ? <Bot aria-hidden="true" /> : undefined}
              />
              <span>{pickerName}</span>
            </>
          ) : (
            <span>{t("match.trumpReveal.unknownPlayer", { defaultValue: "Trump taken" })}</span>
          )}
        </div>

        <div
          className="mt-1 leading-snug"
          style={{ fontSize: 13, color: "#e8dfc8" }}
          data-testid="trump-reveal-copy"
        >
          <Trans
            i18nKey={
              card === null
                ? "match.trumpReveal.namedTrump"
                : isFreePick
                  ? "match.trumpReveal.chose"
                  : "match.trumpReveal.isTrump"
            }
            values={{ suit: suitName }}
            components={{ suit: <b style={{ color: chosenColor }} /> }}
          />
        </div>

        {isFreePick && (
          <div
            className="mt-1"
            style={{ fontSize: 11.5, opacity: 0.6 }}
            data-testid="trump-reveal-candidate"
          >
            {t("match.trumpReveal.candidateOnTable", {
              rank: candidateRankName,
              suit: candidateSuitName,
            })}
          </div>
        )}

        {team && teamLabel && (
          <div className="mt-3.5 flex justify-center">
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
        )}
      </div>
    </div>
  );
}
