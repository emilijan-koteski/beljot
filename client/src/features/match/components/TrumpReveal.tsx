import { Bot } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Trans, useTranslation } from "react-i18next";

import { Avatar } from "@/shared/components/ui/avatar";
import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { playerDisplayName } from "@/shared/lib/botName";
import { Z } from "@/shared/lib/zLayers";
import type { PlayerState, Rank, Suit } from "@/shared/types/matchTypes";

import { seatTeam, teamColors } from "../lib/tableTheme";
import { AutoCloseRing } from "./overlay/AutoCloseRing";
import { PlayingCard } from "./PlayingCard";

interface TrumpRevealProps {
  /** Seat index of the player who took the trump. */
  playerSeat: number;
  /** Local viewer's seat, used to derive viewer-relative team color. When
   *  null (race during initial mount), the panel renders with a brass glow
   *  fallback. */
  myPlayerSeat: number | null;
  cardId: string;
  /** The actually-chosen trump suit. Differs from `cardId`'s suit only in
   *  round-2 free-suit picks; round 1 always has them equal. */
  trumpSuit: Suit;
  players: readonly PlayerState[];
  onComplete: () => void;
}

const SUIT_NAME: Record<Suit, string> = {
  S: "Spades",
  H: "Hearts",
  D: "Diamonds",
  C: "Clubs",
};

const SUIT_GLYPH: Record<Suit, string> = {
  S: "♠",
  H: "♥",
  D: "♦",
  C: "♣",
};

const RANK_NAME: Record<Rank, string> = {
  "7": "Seven",
  "8": "Eight",
  "9": "Nine",
  T: "Ten",
  J: "Jack",
  Q: "Queen",
  K: "King",
  A: "Ace",
};

function parseCardId(id: string) {
  return { rank: id[0] as Rank, suit: id[1] as Suit };
}

function suitNameKey(suit: Suit): "spades" | "hearts" | "diamonds" | "clubs" {
  switch (suit) {
    case "S":
      return "spades";
    case "H":
      return "hearts";
    case "D":
      return "diamonds";
    case "C":
      return "clubs";
  }
}

function rankNameKey(
  rank: Rank,
): "seven" | "eight" | "nine" | "ten" | "jack" | "queen" | "king" | "ace" {
  switch (rank) {
    case "7":
      return "seven";
    case "8":
      return "eight";
    case "9":
      return "nine";
    case "T":
      return "ten";
    case "J":
      return "jack";
    case "Q":
      return "queen";
    case "K":
      return "king";
    case "A":
      return "ace";
  }
}

/**
 * "Trump taken" announcement — the "Wax Seal" panel. The turned-up candidate
 * card is the hero; the *chosen* trump suit is stamped over its corner as an
 * embossed wax seal. One layout serves both rounds: round 1 takes the
 * candidate's suit ("{Suit} is trump this hand"); a round-2 free pick names a
 * different suit ("chose {Suit}" + the candidate that was on the table). The
 * panel glows in the taker's viewer-relative team color (Gold = Us, Silver =
 * Them) and auto-closes after 8 s — early dismissal via the X-with-countdown.
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

  // Hold onComplete in a ref so the AutoCloseRing's callback identity stays
  // stable — preventing the inner countdown from resetting if the parent
  // re-renders mid-reveal (e.g. on a stale matchState push).
  const onCompleteRef = useRef(onComplete);
  useEffect(() => {
    onCompleteRef.current = onComplete;
  }, [onComplete]);

  const handleClose = () => {
    if (!visible) return;
    setVisible(false);
    onCompleteRef.current();
  };

  // Defence in depth: WS dispatch already drops payloads with cardId.length < 2,
  // but parseCardId would silently produce undefined suit/rank if reached with
  // a short string — guard at the boundary.
  if (!visible || cardId.length < 2) {
    return null;
  }

  const picker = players.find((p) => p.seat === playerSeat);
  const pickerName = playerDisplayName(t, picker ?? null);
  const card = parseCardId(cardId);

  // Free-pick (round 2) is detected purely from the wire fields: when the
  // chosen suit differs from the candidate's, the player named a free suit.
  const isFreePick = card.suit !== trumpSuit;

  const suitName = t(`match.suits.${suitNameKey(trumpSuit)}`, {
    defaultValue: SUIT_NAME[trumpSuit],
  });
  const candidateRankName = t(`match.ranks.${rankNameKey(card.rank)}`, {
    defaultValue: RANK_NAME[card.rank],
  });
  const candidateSuitName = t(`match.suits.${suitNameKey(card.suit)}`, {
    defaultValue: SUIT_NAME[card.suit],
  });

  // Viewer-relative team color for the glow, avatar fill, and Us/Them chip.
  // Falls back to brass when the viewer's seat hasn't resolved yet.
  const team = myPlayerSeat !== null ? seatTeam(playerSeat, myPlayerSeat) : null;
  const teamGradient = team ? teamColors(team) : null;
  const glowColor = teamGradient ? teamGradient[0] : "var(--brass, #c9a876)";
  const teamLabel = team ? t(team === "gold" ? "team.us" : "team.them") : null;
  const avatarTeam = team === "gold" ? "A" : team === "silver" ? "B" : null;

  // Wax-seal ring + chosen-suit accent: red for ♥/♦, near-black for ♠/♣.
  const redChosen = trumpSuit === "H" || trumpSuit === "D";
  const sealRing = redChosen ? "#c62828" : "#1a1a1a";
  const chosenColor = redChosen ? "#ff8585" : "#f5f2e8";

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
        className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 pointer-events-auto rounded-[18px] text-center"
        style={{
          width: 262,
          padding: "20px 24px 22px",
          background: "linear-gradient(180deg, rgba(32,64,43,0.98) 0%, rgba(13,38,23,0.98) 100%)",
          border: "1px solid rgba(201,168,118,0.55)",
          boxShadow: `0 22px 54px rgba(0,0,0,0.62), 0 0 0 2px ${glowColor}66, 0 0 28px ${glowColor}55, inset 0 1px 0 rgba(201,168,118,0.25)`,
          color: "var(--ink-light, #f5f2e8)",
          fontFamily: "var(--font-body)",
        }}
        data-team={team ?? undefined}
      >
        {/* close button with countdown ring — top-right */}
        <div className="absolute top-3 right-3 z-3">
          <AutoCloseRing
            duration={prefersReducedMotion ? 1.5 : 8}
            onClose={handleClose}
            ariaLabel={t("match.trumpReveal.dismiss", { defaultValue: "Dismiss" })}
            testId="trump-reveal-close"
          />
        </div>

        {/* eyebrow — padded clear of the close button */}
        <div
          className="text-[9.5px] uppercase tracking-[0.22em]"
          style={{
            color: "var(--brass, #c9a876)",
            fontFamily: "var(--font-body)",
            padding: "0 18px",
            marginBottom: 16,
          }}
          data-testid="trump-reveal-eyebrow"
        >
          {eyebrow}
        </div>

        {/* hero candidate card + chosen-trump wax seal */}
        <div className="relative mb-4.5 inline-block">
          <div
            aria-hidden
            className="absolute rounded-lg"
            style={{ inset: -10, boxShadow: `0 0 32px ${glowColor}55`, zIndex: 0 }}
          />
          <div className="relative z-1">
            <PlayingCard card={card} state="default" size="lg" withTransition={false} />
          </div>
          <div
            className="absolute z-2 flex items-center justify-center"
            style={{
              right: -16,
              bottom: -14,
              width: 60,
              height: 60,
              borderRadius: "50%",
              background: "radial-gradient(circle at 38% 30%, #fffdf6, #efe4c8 68%, #ddcca2)",
              border: `2.5px solid ${sealRing}`,
              boxShadow:
                "0 6px 14px rgba(0,0,0,0.5), inset 0 2px 3px rgba(255,255,255,0.75), inset 0 -3px 6px rgba(0,0,0,0.2)",
            }}
            data-testid="trump-reveal-seal"
            data-suit={trumpSuit}
            aria-label={suitName}
          >
            <span
              style={{
                color: sealRing,
                fontSize: 30,
                lineHeight: 1,
                fontFamily: "var(--font-suit)",
                textShadow: "0 1px 1px rgba(0,0,0,0.25)",
              }}
            >
              {SUIT_GLYPH[trumpSuit]}
            </span>
          </div>
        </div>

        {/* taker */}
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

        {/* what they did — round 1 vs round-2 free pick */}
        <div
          className="mt-1 leading-snug"
          style={{ fontSize: 13, color: "#e8dfc8" }}
          data-testid="trump-reveal-copy"
        >
          {isFreePick ? (
            <Trans
              i18nKey="match.trumpReveal.chose"
              values={{ suit: suitName }}
              components={{ suit: <b style={{ color: chosenColor }} /> }}
            />
          ) : (
            <Trans
              i18nKey="match.trumpReveal.isTrump"
              values={{ suit: suitName }}
              components={{ suit: <b style={{ color: chosenColor }} /> }}
            />
          )}
        </div>

        {/* candidate that was on the table (round 2 only) */}
        {isFreePick && (
          <div
            className="mt-1"
            style={{ fontSize: 11.5, opacity: 0.6 }}
            data-testid="trump-reveal-candidate"
          >
            {t("match.trumpReveal.candidateOnTable", {
              rank: candidateRankName,
              suit: candidateSuitName,
              defaultValue: `${RANK_NAME[card.rank]} of ${SUIT_NAME[card.suit]} was on the table`,
            })}
          </div>
        )}

        {/* team chip */}
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
