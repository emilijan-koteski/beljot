import { useTranslation } from "react-i18next";

import { useAuthStore } from "@/shared/stores/authStore";
import type { Suit, TeamString } from "@/shared/types/matchTypes";

import { resolveCardDeck } from "../lib/cardFace";
import { suitAccent, suitGlowAlpha, suitNameKey } from "../lib/suitArt";
import { TEAM_GOLD, TEAM_SILVER, type TeamGradient } from "../lib/tableTheme";
import { SuitMark } from "./SuitMark";

interface TrumpIndicatorProps {
  trumpSuit: Suit;
  trumpCallerSeat?: number | null;
  trumpCallerName?: string | null;
  viewerTeam?: TeamString | null;
}

const TEAM_NAME_KEY: Record<TeamString, string> = {
  teamA: "team.a",
  teamB: "team.b",
};

function callerTeam(seat: number): TeamString {
  return seat % 2 === 0 ? "teamA" : "teamB";
}

const LEGACY_INK_CLASS: Record<Suit, string> = {
  S: "text-ink",
  H: "text-red-500",
  D: "text-red-500",
  C: "text-ink",
};

const PANEL_BG = "var(--panel-dark, rgba(20,45,30,0.85))";
const INK = "var(--ink-light, #f5f2e8)";
const BRASS = "var(--brass, #c9a876)";

/**
 * Top-right trump chip with the suit orb, suit name, and caller with an Us/Them pill.
 * Without `viewerTeam` it falls back to absolute Team A/B labels and colors.
 */
export function TrumpIndicator({
  trumpSuit,
  trumpCallerSeat,
  trumpCallerName,
  viewerTeam,
}: TrumpIndicatorProps) {
  const { t } = useTranslation();
  const deck = resolveCardDeck(useAuthStore((s) => s.user?.cardDeckPreference));
  const accent = suitAccent(trumpSuit, deck);
  const glowAlpha = suitGlowAlpha(trumpSuit, deck);

  const team: TeamString | null =
    typeof trumpCallerSeat === "number" ? callerTeam(trumpCallerSeat) : null;

  const callerName = trumpCallerName?.trim() || null;
  const suitName = t(suitNameKey(trumpSuit, deck));

  const teamName = team
    ? viewerTeam
      ? t(team === viewerTeam ? "team.us" : "team.them")
      : t(TEAM_NAME_KEY[team])
    : null;

  const teamGradient: TeamGradient | null = team
    ? viewerTeam
      ? team === viewerTeam
        ? TEAM_GOLD
        : TEAM_SILVER
      : null
    : null;

  const legacyTeamClass = team && !teamGradient ? (team === "teamA" ? "team-a" : "team-b") : null;

  const ariaLabel =
    team && teamName && callerName
      ? t("match.trumpIndicator.labelWithCaller", {
          suit: suitName,
          team: teamName,
          name: callerName,
        })
      : team && teamName
        ? t("match.trumpIndicator.labelWithTeam", { suit: suitName, team: teamName })
        : t("match.trumpIndicator.label", { suit: suitName });

  const containerBorderClass = legacyTeamClass !== null ? `border-2 border-${legacyTeamClass}` : "";

  return (
    <div
      className={`flex min-w-0 items-center gap-3 rounded-xl px-3 py-2 ${containerBorderClass}`}
      style={
        legacyTeamClass
          ? undefined
          : {
              background: PANEL_BG,
              border: "1px solid rgba(201,168,118,0.4)",
              backdropFilter: "blur(8px)",
              WebkitBackdropFilter: "blur(8px)",
              boxShadow: "0 8px 24px rgba(0,0,0,0.3)",
            }
      }
      aria-live="polite"
      aria-label={ariaLabel}
      data-testid="trump-indicator"
      data-team={team ?? undefined}
    >
      <div
        className="rounded-full flex items-center justify-center shrink-0"
        style={{
          width: 44,
          height: 44,
          background: `radial-gradient(circle, ${accent}22, transparent 70%), linear-gradient(180deg, #fdfaf0, #f0e8d0)`,
          border: `2px solid ${accent}`,
          boxShadow: `0 0 16px ${accent}${glowAlpha}, inset 0 1px 0 rgba(255,255,255,0.6)`,
        }}
      >
        <SuitMark
          suit={trumpSuit}
          deck={deck}
          size={22}
          className={`${LEGACY_INK_CLASS[trumpSuit]} font-display font-semibold leading-none`}
        />
      </div>

      <div className="flex flex-col min-w-0">
        <div className="flex items-baseline gap-1.5">
          <span
            className="font-body text-[10.5px] uppercase tracking-wider"
            style={{ color: BRASS, opacity: 0.85 }}
          >
            {t("match.trumpIndicator.trump")}
          </span>
          <span
            className="font-display font-semibold capitalize"
            style={{ color: INK, fontSize: 14 }}
          >
            {suitName}
          </span>
        </div>

        {team && teamName && (
          <div
            className="flex items-center gap-2 mt-0.5"
            style={{ color: INK, opacity: 0.85, fontSize: 11 }}
          >
            {callerName && (
              <span
                className="font-body text-ink max-w-32 truncate"
                data-testid="trump-caller-name"
              >
                {callerName}
              </span>
            )}
            <span
              className={`inline-flex items-center gap-1 rounded-full px-1.5 py-px font-body text-[9.5px] font-bold uppercase tracking-wider ${
                legacyTeamClass !== null ? `text-${legacyTeamClass} bg-${legacyTeamClass}/10` : ""
              }`}
              style={
                teamGradient
                  ? {
                      color: teamGradient[0],
                      background: `${teamGradient[0]}22`,
                      border: `1px solid ${teamGradient[0]}88`,
                    }
                  : undefined
              }
              data-testid="trump-caller-team"
              data-team={team}
            >
              <span
                aria-hidden
                className="rounded-full"
                style={{
                  width: 5,
                  height: 5,
                  background: teamGradient ? teamGradient[0] : undefined,
                  boxShadow: teamGradient ? `0 0 5px ${teamGradient[0]}` : undefined,
                }}
              />
              {teamName}
            </span>
          </div>
        )}
      </div>
    </div>
  );
}
