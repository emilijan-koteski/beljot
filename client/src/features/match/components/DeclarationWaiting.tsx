import { Trans } from "react-i18next";

import { Z } from "@/shared/lib/zLayers";

import { type SeatTeam, teamColors } from "../lib/tableTheme";

interface DeclarationWaitingProps {
  activePlayerName: string | null;
  activePlayerTeam: SeatTeam | null;
}

/**
 * Banner shown to the three seats that are NOT on the clock during the
 * dedicated declaration phase.
 *
 * Bitola never needs this: its declarations happen inside a trick that is
 * visibly progressing, so the table always looks alive. The Croatian phase runs
 * between bidding and trick 1, where an unprompted seat sees eight cards, an
 * empty trick and `trickNumber: 0` — a table that reads as frozen for up to
 * four consecutive turns (two minutes at a 30s timer) before the first card is
 * played. This says what the table is waiting for.
 *
 * Mirrors TrumpPrompt's waiting arm exactly — same panel, same placement, same
 * team-coloured name — so the two "someone else is deciding" states read as one
 * pattern rather than two designs.
 */
export function DeclarationWaiting({
  activePlayerName,
  activePlayerTeam,
}: DeclarationWaitingProps) {
  // Team-coloured name at full strength against translucent surrounding ink —
  // the same treatment TrumpPrompt uses, and for the same reason: dimming the
  // whole line with `opacity` would dim the name too.
  const nameColor = activePlayerTeam
    ? teamColors(activePlayerTeam)[0]
    : "var(--ink-light, #f5f2e8)";

  return (
    <div
      className="absolute inset-0 flex items-center justify-center pointer-events-none"
      style={{ zIndex: Z.PROMPT }}
      data-testid="declaration-waiting"
    >
      <div
        className="mx-4 flex max-w-[24rem] items-center gap-3 rounded-lg px-4 py-3"
        style={{
          background: "var(--panel-dark, rgba(20,45,30,0.85))",
          border: "1px solid rgba(201,168,118,0.4)",
          backdropFilter: "blur(8px)",
          WebkitBackdropFilter: "blur(8px)",
        }}
      >
        <p className="font-body text-sm leading-snug" style={{ color: "rgba(245,242,232,0.85)" }}>
          <Trans
            i18nKey="match.declaration.waiting"
            values={{ name: activePlayerName ?? "" }}
            components={{ name: <strong style={{ color: nameColor, fontWeight: 700 }} /> }}
          />
        </p>
      </div>
    </div>
  );
}
