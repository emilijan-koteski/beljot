import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import type { PlayerState, Rank, Suit } from "@/shared/types/matchTypes";
import { type TeamString, teamStringForIndex } from "@/shared/types/matchTypes";
import type { DeclarationsResolvedPayload } from "@/shared/types/wsEvents";

import { declarationLabelKey } from "../lib/declarations";
import { TEAM_GOLD, TEAM_SILVER } from "../lib/tableTheme";
import { AutoCloseRing } from "./overlay/AutoCloseRing";
import { PlayingCard } from "./PlayingCard";

interface DeclarationRevealProps {
  payload: DeclarationsResolvedPayload;
  players: readonly PlayerState[];
  viewerTeam: TeamString;
  onComplete: () => void;
}

function parseCardId(id: string) {
  return { rank: id[0] as Rank, suit: id[1] as Suit };
}

// Same silver-parchment tone the AutoCloseRing uses for the X button — keeps
// the per-meld + total numbers in a team-neutral channel so the team-color
// glow on the panel edge isn't competed with by the digits.
const VALUE_COLOR = "#d4d0c4";

/**
 * Team declarations resolution — full classic-panel reveal showing the
 * winning team's melds. Glow ring follows the winning team's viewer-relative
 * color (Gold = your team won, Silver = opponents). Auto-closes after 8 s
 * or via the X-with-countdown-ring.
 */
export function DeclarationReveal({
  payload,
  players,
  viewerTeam,
  onComplete,
}: DeclarationRevealProps) {
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

  const firstDeclaration = payload.declarations[0];
  if (!visible || payload.winnerTeam === null || firstDeclaration === undefined) {
    return null;
  }

  const winnerTeamString = teamStringForIndex(payload.winnerTeam === 0 ? 0 : 1);
  const isUs = winnerTeamString === viewerTeam;
  const teamGradient = isUs ? TEAM_GOLD : TEAM_SILVER;
  const glowColor = teamGradient[0];

  // Total awarded — server has already zeroed the losing team's melds, so
  // every entry in the payload counts toward the winner's tally.
  const totalAwarded = payload.declarations.reduce((acc, d) => acc + d.value, 0);

  // Tiebreaker line: only render when there's more than one meld in play. The
  // highest-value meld is what tipped the team-result decision per Beljot
  // rules ("highest meld at the table wins all declarations"); naming it +
  // its declarer makes the win-reason explicit instead of leaving the
  // viewer to infer it from the rows.
  const showTiebreaker = payload.declarations.length > 1;
  const highestMeld = showTiebreaker
    ? payload.declarations.reduce((best, d) => (d.value > best.value ? d : best))
    : null;
  const highestDeclarer =
    highestMeld !== null
      ? (players.find((p) => p.seat === highestMeld.playerSeat)?.username ??
        `#${highestMeld.playerSeat}`)
      : null;
  const highestLabel =
    highestMeld !== null
      ? t(`match.declaration.${declarationLabelKey(highestMeld.type, highestMeld.cards.length)}`)
      : null;

  return (
    <div
      className={`absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 z-50 ${
        prefersReducedMotion
          ? ""
          : "motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-bottom-2 motion-safe:duration-300"
      }`}
      data-testid="declaration-reveal"
    >
      <div
        className="relative rounded-2xl px-4 py-4 md:px-5.5 md:pt-5 md:pb-4.5"
        style={{
          // Desktop keeps the 520 design width; on a phone it caps to the
          // viewport (−2rem) so the centered panel no longer clips its cards
          // and +value column off both edges. maxHeight guards a tall melds
          // list on short screens — the rows scroll, the brass total stays.
          width: "min(520px, calc(100vw - 2rem))",
          maxHeight: "calc(100vh - 2rem)",
          overflowY: "auto",
          background: "linear-gradient(180deg, rgba(30,60,40,0.98) 0%, rgba(14,40,24,0.98) 100%)",
          border: "1px solid rgba(201,168,118,0.55)",
          boxShadow: `0 12px 32px rgba(0,0,0,0.55), 0 0 0 2px ${glowColor}88, 0 0 24px ${glowColor}77, inset 0 1px 0 rgba(201,168,118,0.22)`,
          color: "var(--ink-light, #f5f2e8)",
          fontFamily: "var(--font-body)",
        }}
        data-team={winnerTeamString}
      >
        <div className="absolute top-3 right-3">
          <AutoCloseRing
            duration={prefersReducedMotion ? 1.5 : 8}
            onClose={handleClose}
            ariaLabel={t("match.declaration.dismiss", { defaultValue: "Dismiss" })}
            testId="declaration-reveal-close"
          />
        </div>

        {/* Header: brass eyebrow + big serif title with team-color dot. */}
        <div className="text-center mb-5">
          <div
            className="text-[11px] uppercase mb-2"
            style={{
              color: "var(--brass, #c9a876)",
              fontFamily: "var(--font-body)",
              letterSpacing: 3,
              opacity: 0.85,
            }}
          >
            {t("match.declaration.resolved")}
          </div>
          <div
            className="flex items-center justify-center gap-2.5 font-semibold leading-tight"
            style={{
              fontFamily: "var(--font-body)",
              fontSize: 22,
              letterSpacing: 0.3,
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
              className="text-[12.5px] mt-1.5"
              style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.7 }}
              data-testid="declaration-reveal-tiebreaker"
            >
              {t("match.declaration.tiebreaker", {
                player: highestDeclarer,
                label: highestLabel,
              })}
            </div>
          )}
        </div>

        {/* Meld rows — cards on the left, label + "by Name" in the middle,
            +value on the right. */}
        <div className="flex flex-col gap-3 mb-4">
          {payload.declarations.map((decl, i) => {
            const declarer = players.find((p) => p.seat === decl.playerSeat);
            const username = declarer?.username ?? `#${decl.playerSeat}`;
            const meldLabel = t(
              `match.declaration.${declarationLabelKey(decl.type, decl.cards.length)}`,
            );
            return (
              <div
                key={i}
                className="flex items-center gap-4 rounded-lg px-3.5 py-3"
                style={{
                  background: "rgba(0,0,0,0.22)",
                  border: "1px solid rgba(201,168,118,0.25)",
                }}
              >
                <div className="flex shrink-0">
                  {decl.cards.map((cardId, j) => (
                    <div key={cardId} style={{ marginLeft: j === 0 ? 0 : -16, zIndex: j }}>
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
                      color: "var(--ink-light, #f5f2e8)",
                      fontFamily: "var(--font-body)",
                      fontSize: 15,
                    }}
                    data-testid="declaration-reveal-meld-label"
                  >
                    {meldLabel}
                  </span>
                  <span
                    className="text-[12px]"
                    style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.7 }}
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
                    fontFamily: "var(--font-body)",
                    fontSize: 22,
                  }}
                  data-testid="declaration-reveal-meld-value"
                >
                  +{decl.value}
                </div>
              </div>
            );
          })}
        </div>

        {/* Brass-tinted total strip — same family as the ScoreReveal match
            strip so end-of-hand panels read as one set. */}
        <div
          className="rounded-lg px-4 py-3 flex items-center justify-between"
          style={{
            background: "rgba(201,168,118,0.1)",
            border: "1px solid rgba(201,168,118,0.3)",
          }}
          data-testid="declaration-reveal-total"
        >
          <div className="flex flex-col">
            <span
              className="text-[10px] uppercase"
              style={{
                color: "var(--brass, #c9a876)",
                fontFamily: "var(--font-body)",
                letterSpacing: 1.8,
                opacity: 0.85,
              }}
            >
              {t("match.declaration.total")}
            </span>
            <span
              className="text-[12px] mt-0.5"
              style={{ color: "var(--ink-light, #f5f2e8)", opacity: 0.65 }}
            >
              {t("match.declaration.addedToHand")}
            </span>
          </div>
          <div
            className="font-bold tabular-nums"
            style={{
              color: VALUE_COLOR,
              fontFamily: "var(--font-body)",
              fontSize: 30,
              letterSpacing: -0.5,
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
