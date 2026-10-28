import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { useSfxOnce } from "@/shared/audio/useSfxOnce";
import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { playerDisplayName } from "@/shared/lib/botName";
import { Z } from "@/shared/lib/zLayers";
import type { PlayerState, Rank, Suit } from "@/shared/types/matchTypes";
import { type TeamString, teamStringForIndex } from "@/shared/types/matchTypes";
import type { DeclarationsResolvedPayload } from "@/shared/types/wsEvents";

import { declarationLabelKey } from "../lib/declarations";
import { TEAM_GOLD, TEAM_SILVER } from "../lib/tableTheme";
import { AutoCloseRing } from "./overlay/AutoCloseRing";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { MeldRow } from "./overlay/MeldRow";
import { SummaryStrip } from "./overlay/SummaryStrip";

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
      <div data-team={winnerTeamString}>
        <ClassicPanel
          size="wide"
          glowColor={glowColor}
          className="relative max-h-[calc(100vh-7rem)] md:max-h-[calc(100vh-2rem)]"
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
              className="font-semibold leading-tight"
              style={{
                fontSize: 24,
              }}
              data-testid="declaration-reveal-team"
              data-team={winnerTeamString}
            >
              {isUs ? t("match.declaration.headlineUs") : t("match.declaration.headlineThem")}
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
              return (
                <MeldRow
                  key={i}
                  label={t(
                    `match.declaration.${declarationLabelKey(decl.type, decl.cards.length)}`,
                  )}
                  cards={decl.cards.map(parseCardId)}
                  value={decl.value}
                  testId="declaration-reveal-meld"
                  byline={
                    <span data-testid="declaration-reveal-declarer" data-seat={decl.playerSeat}>
                      {t("match.declaration.byPlayer", { name: username })}
                    </span>
                  }
                />
              );
            })}
          </div>

          <SummaryStrip
            label={t("match.declaration.total")}
            value={`+${totalAwarded}`}
            testId="declaration-reveal-total"
            valueTestId="declaration-reveal-total-value"
          />
        </ClassicPanel>
      </div>
    </div>
  );
}
