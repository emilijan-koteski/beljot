import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { useSfxOnce } from "@/shared/audio/useSfxOnce";
import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { playerDisplayName } from "@/shared/lib/botName";
import { Z } from "@/shared/lib/zLayers";
import type { PlayerState, Rank, Suit } from "@/shared/types/matchTypes";
import { type TeamString, teamStringForIndex } from "@/shared/types/matchTypes";
import type { DeclarationsResolvedPayload } from "@/shared/types/wsEvents";

import { CARD_SIZES } from "../lib/cardFace";
import { declarationLabelKey } from "../lib/declarations";
import { TEAM_GOLD, TEAM_SILVER } from "../lib/tableTheme";
import { AutoCloseRing } from "./overlay/AutoCloseRing";
import { PlayingCard } from "./PlayingCard";

const CARD_OVERLAP = Math.round(CARD_SIZES.sm.width * 0.4);

interface DeclarationRevealProps {
  payload: DeclarationsResolvedPayload;
  players: readonly PlayerState[];
  viewerTeam: TeamString;
  onComplete: () => void;
  soundKey?: string | null;
}

function parseCardId(id: string) {
  return { rank: id[0] as Rank, suit: id[1] as Suit };
}

const VALUE_COLOR = "#d4d0c4";

/** Reveal panel showing the winning team's melds, glowing in that team's viewer-relative color. */
export function DeclarationReveal({
  payload,
  players,
  viewerTeam,
  onComplete,
  soundKey,
}: DeclarationRevealProps) {
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

  const firstDeclaration = payload.declarations[0];
  if (!visible || payload.winnerTeam === null || firstDeclaration === undefined) {
    return null;
  }

  const winnerTeamString = teamStringForIndex(payload.winnerTeam === 0 ? 0 : 1);
  const isUs = winnerTeamString === viewerTeam;
  const teamGradient = isUs ? TEAM_GOLD : TEAM_SILVER;
  const glowColor = teamGradient[0];

  const totalAwarded = payload.declarations.reduce((acc, d) => acc + d.value, 0);

  const showTiebreaker = payload.contested && payload.declarations.length > 0;
  const highestMeld = showTiebreaker
    ? payload.declarations.reduce((best, d) => (d.value > best.value ? d : best))
    : null;
  const highestDeclarer =
    highestMeld !== null
      ? (playerDisplayName(t, players.find((p) => p.seat === highestMeld.playerSeat) ?? null) ??
        `#${highestMeld.playerSeat}`)
      : null;
  const highestLabel =
    highestMeld !== null
      ? t(`match.declaration.${declarationLabelKey(highestMeld.type, highestMeld.cards.length)}`)
      : null;

  return (
    <div
      className={`absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 ${
        prefersReducedMotion
          ? ""
          : "motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-bottom-2 motion-safe:duration-300"
      }`}
      style={{ zIndex: Z.REVEAL }}
      data-testid="declaration-reveal"
    >
      <div
        className="relative rounded-2xl px-4 py-4 md:px-5.5 md:pt-5 md:pb-4.5 max-h-[calc(100vh-7rem)] md:max-h-[calc(100vh-2rem)]"
        style={{
          width: "min(520px, calc(100vw - 2rem))",
          overflowY: "auto",
          background: "linear-gradient(180deg, rgba(30,60,40,0.98) 0%, rgba(14,40,24,0.98) 100%)",
          border: "1px solid var(--brass-55)",
          boxShadow: `var(--shadow-felt-lg), 0 0 0 2px ${glowColor}88, 0 0 24px ${glowColor}77, inset 0 1px 0 var(--brass-25)`,
          color: "var(--ink-light)",
        }}
        data-team={winnerTeamString}
      >
        <div className="absolute top-3 right-3 z-3">
          <AutoCloseRing
            duration={prefersReducedMotion ? 1.5 : 8}
            onClose={handleClose}
            ariaLabel={t("match.declaration.dismiss", { defaultValue: "Dismiss" })}
            testId="declaration-reveal-close"
          />
        </div>

        <div className="text-center mb-5">
          <div
            className="text-2xs tracking-caps uppercase mb-2"
            style={{
              color: "var(--brass)",
              opacity: 0.85,
            }}
          >
            {t("match.declaration.resolved")}
          </div>
          <div
            className="flex items-center justify-center gap-2.5 font-semibold leading-tight"
            style={{
              fontSize: 24,
            }}
            data-testid="declaration-reveal-team"
            data-team={winnerTeamString}
          >
            <span
              aria-hidden
              className="rounded-full inline-block"
              style={{
                width: 9,
                height: 9,
                background: glowColor,
                boxShadow: `0 0 12px ${glowColor}`,
              }}
            />
            <span>
              {isUs ? t("match.declaration.headlineUs") : t("match.declaration.headlineThem")}
            </span>
          </div>
          {showTiebreaker && highestDeclarer && highestLabel && (
            <div
              className="text-xs mt-1.5"
              style={{ color: "var(--ink-light)", opacity: 0.7 }}
              data-testid="declaration-reveal-tiebreaker"
            >
              {t("match.declaration.tiebreaker", {
                player: highestDeclarer,
                label: highestLabel,
              })}
            </div>
          )}
        </div>

        <div className="flex flex-col gap-3 mb-4">
          {payload.declarations.map((decl, i) => {
            const declarer = players.find((p) => p.seat === decl.playerSeat);
            const username = playerDisplayName(t, declarer) ?? `#${decl.playerSeat}`;
            const meldLabel = t(
              `match.declaration.${declarationLabelKey(decl.type, decl.cards.length)}`,
            );
            return (
              <div
                key={i}
                className="flex items-center gap-4 rounded-lg px-3.5 py-3"
                style={{
                  background: "rgba(0,0,0,0.22)",
                  border: "1px solid var(--brass-25)",
                }}
              >
                <div className="flex shrink-0">
                  {decl.cards.map((cardId, j) => (
                    <div
                      key={cardId}
                      style={{ marginLeft: j === 0 ? 0 : -CARD_OVERLAP, zIndex: j }}
                    >
                      <PlayingCard
                        card={parseCardId(cardId)}
                        state="default"
                        size="sm"
                        withTransition={false}
                      />
                    </div>
                  ))}
                </div>
                <div className="flex flex-col gap-0.5 min-w-0 flex-1">
                  <span
                    className="font-semibold truncate"
                    style={{
                      color: "var(--ink-light)",
                      fontSize: 16,
                    }}
                    data-testid="declaration-reveal-meld-label"
                  >
                    {meldLabel}
                  </span>
                  <span
                    className="text-xs"
                    style={{ color: "var(--ink-light)", opacity: 0.7 }}
                    data-testid="declaration-reveal-declarer"
                    data-seat={decl.playerSeat}
                  >
                    {t("match.declaration.byPlayer", { name: username })}
                  </span>
                </div>
                <div
                  className="font-bold tabular-nums shrink-0"
                  style={{
                    color: VALUE_COLOR,
                    fontSize: 24,
                  }}
                  data-testid="declaration-reveal-meld-value"
                >
                  +{decl.value}
                </div>
              </div>
            );
          })}
        </div>

        <div
          className="rounded-lg px-4 py-3 flex items-center justify-between"
          style={{
            background: "var(--brass-10)",
            border: "1px solid var(--brass-25)",
          }}
          data-testid="declaration-reveal-total"
        >
          <div className="flex flex-col">
            <span
              className="text-2xs tracking-caps uppercase"
              style={{
                color: "var(--brass)",
                opacity: 0.85,
              }}
            >
              {t("match.declaration.total")}
            </span>
            <span className="text-xs mt-0.5" style={{ color: "var(--ink-light)", opacity: 0.65 }}>
              {t("match.declaration.addedToHand")}
            </span>
          </div>
          <div
            className="font-bold tabular-nums"
            style={{
              color: VALUE_COLOR,
              fontSize: 28,
              letterSpacing: "-0.025em",
            }}
            data-testid="declaration-reveal-total-value"
          >
            +{totalAwarded}
          </div>
        </div>
      </div>
    </div>
  );
}
