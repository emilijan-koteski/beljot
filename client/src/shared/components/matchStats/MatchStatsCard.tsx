import { Bot, ChevronDown } from "lucide-react";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import type { MatchHandView, MatchListItem, MatchOutcome } from "@/shared/api/matches";
import { useMediaQuery } from "@/shared/hooks/useMediaQuery";
import { botDisplayName } from "@/shared/lib/botName";
import { formatClockTime, formatLocalizedDate } from "@/shared/lib/formatDate";
import { variantLabel } from "@/shared/lib/roomLabels";

import { SeatChip } from "./SeatChip";

function formatDuration(
  startedAt: string,
  completedAt: string,
  t: (key: string, options?: Record<string, unknown>) => string,
): string {
  const start = new Date(startedAt).getTime();
  const end = new Date(completedAt).getTime();
  if (!Number.isFinite(start) || !Number.isFinite(end) || end < start) {
    return "—";
  }
  const totalSec = Math.floor((end - start) / 1000);
  const pad = (n: number) => n.toString().padStart(2, "0");
  const d = Math.floor(totalSec / 86400);
  const h = Math.floor((totalSec % 86400) / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  if (d > 0) {
    return t("profile.matchHistory.durationDhms", { d, h: pad(h), m: pad(m), s: pad(s) });
  }
  if (totalSec >= 3600) {
    return t("profile.matchHistory.durationHms", { h: pad(h), m: pad(m), s: pad(s) });
  }
  return t("profile.matchHistory.durationMs", { m: pad(m), s: pad(s) });
}

function OutcomeChip({ outcome }: { outcome: MatchOutcome }) {
  const { t } = useTranslation();
  const cfg: Record<MatchOutcome, { bg: string; color: string; border: string; labelKey: string }> =
    {
      win: {
        bg: "var(--accent-soft)",
        color: "var(--accent)",
        border: "rgba(25,101,54,0.33)",
        labelKey: "profile.matchHistory.outcomeWin",
      },
      loss: {
        bg: "rgba(139,42,31,0.10)",
        color: "var(--danger)",
        border: "rgba(139,42,31,0.30)",
        labelKey: "profile.matchHistory.outcomeLoss",
      },
      abandoned: {
        bg: "var(--brass-soft)",
        color: "var(--brass-deep)",
        border: "rgba(201,168,118,0.40)",
        labelKey: "profile.matchHistory.outcomeAbandoned",
      },
    };
  const c = cfg[outcome];
  return (
    <span
      className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[11.5px] font-semibold tracking-[0.2px]"
      style={{ background: c.bg, color: c.color, border: `1px solid ${c.border}` }}
      data-testid="match-history-outcome"
      data-outcome={outcome}
    >
      {t(c.labelKey)}
    </span>
  );
}

/**
 * Which hand layout to render. "auto" picks the dense desktop table or the
 * stacked cards from a viewport media query (the profile page's behaviour);
 * "stacked" forces the cards regardless of viewport — needed inside the
 * end-of-match overlay, whose panel is far narrower than the viewport that
 * media query is measuring.
 */
export type HandsLayout = "auto" | "stacked";

interface HandsGridProps {
  hands: MatchHandView[];
  viewerTeamIndex: 0 | 1;
  layout: HandsLayout;
  // Final match scores, viewer-relative. Used ONLY to detect a hand that was
  // played but never scored: a room playing "dosta" ends the match the instant a
  // team reaches the target, and that cut-short hand deliberately gets no
  // hand_results row. Its points are still inside the final score, so without
  // this the breakdown silently falls short of the total (observed: hands
  // summing to 407 under a displayed 516) and reads as broken arithmetic.
  //
  // Derived from the persisted match rather than from event:match_end, so the
  // profile's match history gets the same accounting — there is no payload
  // there.
  usFinal: number;
  themFinal: number;
}

/** True when a hand has any note badge (capot / last-trick / failed contract). */
function handHasNotes(h: MatchHandView): boolean {
  return h.capot || h.lastTrickBonus > 0 || h.failedContract;
}

interface HandNotesProps {
  hand: MatchHandView;
  capotIsUs: boolean;
  lastTrickIsUs: boolean;
  contractingIsUs: boolean;
  usColor: string;
  themColor: string;
}

/** The capot / last-trick / failed-contract badges for a hand. Shared by the
 *  desktop table cell and the mobile hand card so the wording + testids stay
 *  identical across layouts. Returns a fragment; the caller supplies the
 *  wrapping flex container. */
function HandNotes({
  hand: h,
  capotIsUs,
  lastTrickIsUs,
  contractingIsUs,
  usColor,
  themColor,
}: HandNotesProps) {
  const { t } = useTranslation();
  const teamWord = (isUs: boolean) =>
    isUs ? t("profile.matchHistory.hand.note.us") : t("profile.matchHistory.hand.note.them");
  return (
    <>
      {h.capot && (
        <span
          className="rounded-md px-1.5 py-0.5 text-[10.5px] font-semibold tracking-[0.5px]"
          style={{
            background: "var(--accent-soft)",
            color: "var(--accent)",
            border: "1px solid rgba(25,101,54,0.33)",
          }}
          data-testid="match-history-hand-capot"
        >
          {t("profile.matchHistory.hand.note.capot", { points: h.capotBonus })} ·{" "}
          {teamWord(capotIsUs)}
        </span>
      )}
      {!h.capot && h.lastTrickBonus > 0 && (
        <span
          className="rounded-md px-1.5 py-0.5 text-[10.5px] font-semibold tracking-[0.5px]"
          style={{
            background: "var(--surface-3)",
            color: lastTrickIsUs ? usColor : themColor,
            border: "1px solid var(--border)",
          }}
          data-testid="match-history-hand-last-trick"
        >
          {t("profile.matchHistory.hand.note.lastTrick", { points: h.lastTrickBonus })} ·{" "}
          {teamWord(lastTrickIsUs)}
        </span>
      )}
      {h.failedContract && (
        <span
          className="rounded-md px-1.5 py-0.5 text-[10.5px] font-semibold tracking-[0.5px]"
          style={{
            background: "rgba(139,42,31,0.10)",
            color: "var(--danger)",
            border: "1px solid rgba(139,42,31,0.30)",
          }}
          data-testid="match-history-hand-failed"
        >
          {contractingIsUs
            ? t("profile.matchHistory.hand.note.failedUs")
            : t("profile.matchHistory.hand.note.failedThem")}
        </span>
      )}
    </>
  );
}

function HandsGrid({ hands, viewerTeamIndex, layout, usFinal, themFinal }: HandsGridProps) {
  const { t } = useTranslation();
  // Phones get a stacked per-hand card layout instead of the dense 7-column
  // table — the table needs ~620px and would otherwise force a horizontal
  // scroll inside the (much narrower) expanded row. Rendering exactly one
  // layout (not both behind `hidden`/`md:block`) keeps the per-hand testids
  // unique. `useMediaQuery` returns false without `matchMedia` (tests/SSR), so
  // those environments get the desktop table.
  //
  // The hook runs unconditionally — layout === "stacked" only overrides its
  // ANSWER, because a caller inside a fixed-width overlay knows the viewport
  // query is measuring the wrong box.
  const viewportIsCompact = useMediaQuery("(max-width: 767px)");
  const isCompact = layout === "stacked" || viewportIsCompact;

  if (hands.length === 0) {
    return (
      <p className="text-ink-dim mt-1 text-sm italic" data-testid="match-history-no-hands">
        {t("profile.matchHistory.noHandDetails")}
      </p>
    );
  }

  const usColor = viewerTeamIndex === 0 ? "var(--team-a)" : "var(--team-b)";
  const themColor = viewerTeamIndex === 0 ? "var(--team-b)" : "var(--team-a)";

  // The hand a "dosta" stop cut short: whatever the final score holds that the
  // scored hands do not account for. Both sides are shown even when only one
  // crossed, because the other team's partial points were banked too.
  //
  // Guarded on > 0 so a normal match — where the rows sum exactly — renders
  // nothing at all, and so a mid-hand surrender (which banks nothing) does not
  // grow a phantom row.
  const scoredUs = hands.reduce(
    (n, h) => n + (viewerTeamIndex === 0 ? h.teamAHandTotal : h.teamBHandTotal),
    0,
  );
  const scoredThem = hands.reduce(
    (n, h) => n + (viewerTeamIndex === 0 ? h.teamBHandTotal : h.teamAHandTotal),
    0,
  );
  const unscoredUs = usFinal - scoredUs;
  const unscoredThem = themFinal - scoredThem;
  const hasUnscoredHand = unscoredUs > 0 || unscoredThem > 0;

  // Per-hand viewer-relative figures + flags, shared by both layouts.
  const derive = (h: MatchHandView) => {
    const us =
      viewerTeamIndex === 0
        ? { total: h.teamAHandTotal, card: h.teamACardPoints, decl: h.teamADeclPoints }
        : { total: h.teamBHandTotal, card: h.teamBCardPoints, decl: h.teamBDeclPoints };
    const them =
      viewerTeamIndex === 0
        ? { total: h.teamBHandTotal, card: h.teamBCardPoints, decl: h.teamBDeclPoints }
        : { total: h.teamAHandTotal, card: h.teamACardPoints, decl: h.teamADeclPoints };
    const contractingIsUs = h.contractingTeam === viewerTeamIndex;
    return {
      us,
      them,
      usWonHand: us.total > them.total,
      contractingIsUs,
      lastTrickIsUs: h.lastTrickTeam === viewerTeamIndex,
      capotIsUs: h.capotTeam === viewerTeamIndex,
      desc: h.capot
        ? t("profile.matchHistory.hand.desc.capot")
        : h.failedContract
          ? t("profile.matchHistory.hand.desc.failed")
          : contractingIsUs
            ? t("profile.matchHistory.hand.desc.weCalled")
            : t("profile.matchHistory.hand.desc.theyCalled"),
    };
  };

  // ── Mobile: stacked, self-labelled hand cards (no horizontal scroll). ──
  if (isCompact) {
    return (
      <div className="flex flex-col gap-2" data-testid="match-history-hands-grid">
        {hands.map((h) => {
          const d = derive(h);
          return (
            <div
              key={h.handNumber}
              className="border-border bg-surface-2 rounded-lg border p-3"
              data-testid="match-history-hand-row"
              data-hand-number={h.handNumber}
            >
              <div className="flex items-baseline gap-2">
                <span className="text-ink-mute font-mono text-[11px] font-semibold tabular-nums">
                  {String(h.handNumber).padStart(2, "0")}
                </span>
                <span className="text-ink-dim text-[13px]">{d.desc}</span>
              </div>

              <div className="mt-2 flex items-center gap-5">
                <div className="flex items-baseline gap-1.5">
                  <span
                    className="text-[10px] font-semibold tracking-[1px] uppercase"
                    style={{ color: usColor }}
                  >
                    {t("team.us")}
                  </span>
                  <span
                    className="font-display text-[17px] font-bold tabular-nums"
                    style={{ color: d.usWonHand ? usColor : "var(--ink)" }}
                  >
                    {d.us.total}
                  </span>
                </div>
                <div className="flex items-baseline gap-1.5">
                  <span
                    className="text-[10px] font-semibold tracking-[1px] uppercase"
                    style={{ color: themColor }}
                  >
                    {t("team.them")}
                  </span>
                  <span
                    className="font-display text-[17px] font-bold tabular-nums"
                    style={{ color: !d.usWonHand ? themColor : "var(--ink)" }}
                  >
                    {d.them.total}
                  </span>
                </div>
              </div>

              <div className="text-ink-mute mt-1.5 flex flex-wrap gap-x-4 gap-y-0.5 text-[11px] tabular-nums">
                <span>
                  <span className="text-ink-off tracking-[0.5px] uppercase">
                    {t("profile.matchHistory.hand.col.cards")}
                  </span>{" "}
                  {d.us.card} <span className="text-ink-off">/</span> {d.them.card}
                </span>
                <span>
                  <span className="text-ink-off tracking-[0.5px] uppercase">
                    {t("profile.matchHistory.hand.col.declarations")}
                  </span>{" "}
                  {d.us.decl} <span className="text-ink-off">/</span> {d.them.decl}
                </span>
              </div>

              {handHasNotes(h) && (
                <div className="mt-2 flex flex-wrap gap-1.5">
                  <HandNotes
                    hand={h}
                    capotIsUs={d.capotIsUs}
                    lastTrickIsUs={d.lastTrickIsUs}
                    contractingIsUs={d.contractingIsUs}
                    usColor={usColor}
                    themColor={themColor}
                  />
                </div>
              )}
            </div>
          );
        })}
        {hasUnscoredHand && (
          <div
            className="border-border/70 bg-surface-2/60 rounded-lg border border-dashed p-3"
            data-testid="match-history-unscored-hand"
          >
            <div className="flex items-baseline gap-2">
              <span className="text-ink-mute font-mono text-[11px] font-semibold tabular-nums">
                {String(hands.length + 1).padStart(2, "0")}
              </span>
              <span className="text-ink-dim text-[13px]">
                {t("profile.matchHistory.hand.unfinished")}
              </span>
            </div>
            <div className="mt-2 flex items-center gap-5">
              <div className="flex items-baseline gap-1.5">
                <span
                  className="text-[10px] font-semibold tracking-[1px] uppercase"
                  style={{ color: usColor }}
                >
                  {t("team.us")}
                </span>
                <span className="font-display text-[17px] font-bold tabular-nums">
                  {unscoredUs}
                </span>
              </div>
              <div className="flex items-baseline gap-1.5">
                <span
                  className="text-[10px] font-semibold tracking-[1px] uppercase"
                  style={{ color: themColor }}
                >
                  {t("team.them")}
                </span>
                <span className="font-display text-[17px] font-bold tabular-nums">
                  {unscoredThem}
                </span>
              </div>
            </div>
          </div>
        )}
      </div>
    );
  }

  // ── Desktop: dense 7-column table. ──
  const cols = "44px minmax(120px,1fr) 56px 56px 90px 110px minmax(120px,1fr)";

  return (
    <div className="overflow-x-auto" data-testid="match-history-hands-grid">
      <div className="min-w-155">
        <div
          className="text-brass-deep grid items-center gap-3 px-2.5 py-1.5 font-mono text-[10px] font-semibold tracking-[1.5px] uppercase"
          style={{ gridTemplateColumns: cols }}
        >
          <span>{t("profile.matchHistory.hand.col.number")}</span>
          <span>{t("profile.matchHistory.hand.col.hand")}</span>
          <span className="text-right" style={{ color: usColor }}>
            {t("team.us")}
          </span>
          <span className="text-right" style={{ color: themColor }}>
            {t("team.them")}
          </span>
          <span>{t("profile.matchHistory.hand.col.cards")}</span>
          <span>{t("profile.matchHistory.hand.col.declarations")}</span>
          <span>{t("profile.matchHistory.hand.col.notes")}</span>
        </div>

        {hands.map((h, idx) => {
          const d = derive(h);
          return (
            <div
              key={h.handNumber}
              className="grid items-center gap-3 rounded-lg px-2.5 py-2 text-[13px]"
              style={{
                gridTemplateColumns: cols,
                background: idx % 2 ? "transparent" : "var(--surface-2)",
              }}
              data-testid="match-history-hand-row"
              data-hand-number={h.handNumber}
            >
              <span className="text-ink-mute font-mono text-[11px] font-semibold tabular-nums">
                {String(h.handNumber).padStart(2, "0")}
              </span>
              <span className="text-ink-dim text-xs">{d.desc}</span>
              <span
                className="font-display text-right text-[15px] font-semibold tabular-nums"
                style={{ color: d.usWonHand ? usColor : "var(--ink-mute)" }}
              >
                {d.us.total}
              </span>
              <span
                className="font-display text-right text-[15px] font-semibold tabular-nums"
                style={{ color: !d.usWonHand ? themColor : "var(--ink-mute)" }}
              >
                {d.them.total}
              </span>
              <span className="text-ink-dim text-xs tabular-nums">
                {d.us.card} <span className="text-ink-off">/</span> {d.them.card}
              </span>
              <span className="text-ink-dim text-xs tabular-nums">
                {d.us.decl} <span className="text-ink-off">/</span> {d.them.decl}
              </span>
              <span className="flex flex-wrap gap-1.5">
                <HandNotes
                  hand={h}
                  capotIsUs={d.capotIsUs}
                  lastTrickIsUs={d.lastTrickIsUs}
                  contractingIsUs={d.contractingIsUs}
                  usColor={usColor}
                  themColor={themColor}
                />
              </span>
            </div>
          );
        })}
        {hasUnscoredHand && (
          <div
            className="grid items-center gap-3 rounded-lg px-2.5 py-2 text-[13px]"
            style={{ gridTemplateColumns: cols, background: "transparent" }}
            data-testid="match-history-unscored-hand"
          >
            <span className="text-ink-mute font-mono text-[11px] font-semibold tabular-nums">
              {String(hands.length + 1).padStart(2, "0")}
            </span>
            <span className="text-ink-dim text-xs italic">
              {t("profile.matchHistory.hand.unfinished")}
            </span>
            <span className="font-display text-right font-bold tabular-nums">{unscoredUs}</span>
            <span className="font-display text-right font-bold tabular-nums">{unscoredThem}</span>
            <span />
            <span />
            <span />
          </div>
        )}
      </div>
    </div>
  );
}

interface MatchStatsCardProps {
  match: MatchListItem;
  /** Marks the subject's seat as "YOU" — false on a public profile, where the
   *  subject is someone other than the viewer. */
  subjectIsSelf?: boolean;
  isOpen: boolean;
  onToggle: () => void;
  /** See `HandsLayout`. Defaults to the viewport-driven "auto". */
  handsLayout?: HandsLayout;
  /**
   * Whether seat chips link to player profiles. FALSE inside the in-match
   * overlay: those links are react-router <Link>s, and a click PUSH-navigates
   * away, unmounting MatchPage and skipping the return-to-room / leave-room
   * settlement entirely — the player's membership row is left dangling. On the
   * profile and in the room lobby, navigating away is harmless and wanted.
   */
  linkPlayers?: boolean;
  /** Rendered inside the expanded detail, below the hand breakdown — the
   *  per-player Add-friend / Reinvite row on the room surfaces. */
  footer?: ReactNode;
}

/**
 * One match rendered as a collapsible card: header (date, variant, seats,
 * viewer-first score, outcome) plus an expandable per-hand breakdown.
 *
 * Lives in `shared/` because three surfaces mount it — the profile's match
 * history list, the room lobby's last-match dialog, and the end-of-match
 * overlay — and the hand breakdown must not fork between them. Everything it
 * renders is viewer-relative through `match.viewerSeat`, which the server
 * derives per request, so the card itself needs no notion of who is looking.
 *
 * Open state is LIFTED: the profile keeps a set of open row ids, the dialogs
 * keep a single boolean.
 */
export function MatchStatsCard({
  match,
  subjectIsSelf = true,
  isOpen,
  onToggle,
  handsLayout = "auto",
  linkPlayers = true,
  footer,
}: MatchStatsCardProps) {
  const { t } = useTranslation();

  const teammateSeat = (match.viewerSeat + 2) % 4;
  const opp1Seat = (match.viewerSeat + 1) % 4;
  const opp2Seat = (match.viewerSeat + 3) % 4;
  // Bot seats carry an empty username with isBot:true — render the localized
  // seat-derived bot name (and the bot glyph in the chip avatar) instead of
  // a blank chip. Only real, still-existing users get a `userId`, which turns
  // the chip into a link to their public profile: isBot !== true, userId > 0
  // AND username !== "" (explicit comparisons, never truthiness on Go zero
  // values). A soft-deleted participant arrives as {userId > 0, username: "",
  // isBot: false} — the server hydrates usernames from the users table, which
  // excludes deleted rows — and linking their "—" chip would be a sure 404.
  const seatChipProps = (seat: number): { name: string; bot: boolean; userId?: number } => {
    const p = match.players.find((pl) => pl.seat === seat);
    if (!p) return { name: "—", bot: false };
    if (p.isBot === true) return { name: botDisplayName(t, p.seat), bot: true };
    return {
      name: p.username || "—",
      bot: false,
      userId: linkPlayers && p.userId > 0 && p.username !== "" ? p.userId : undefined,
    };
  };
  const teammate = seatChipProps(teammateSeat);
  const opponent1 = seatChipProps(opp1Seat);
  const opponent2 = seatChipProps(opp2Seat);
  // The subject IS the viewer seat — the server derives it per request, so the
  // card never needs the viewer's identity passed in. Their own chip is never
  // a profile link (you are already looking at their page, or at yourself).
  // Same "—" fallback as every other seat (seatChipProps): a soft-deleted or
  // missing subject must not render a blank chip with a "?" avatar.
  const subjectName = match.players.find((p) => p.seat === match.viewerSeat)?.username || "—";

  const viewerTeamIndex: 0 | 1 = match.viewerSeat % 2 === 0 ? 0 : 1;
  const usTeam: "A" | "B" = viewerTeamIndex === 0 ? "A" : "B";
  const themTeam: "A" | "B" = viewerTeamIndex === 0 ? "B" : "A";
  const usColor = viewerTeamIndex === 0 ? "var(--team-a)" : "var(--team-b)";
  const themColor = viewerTeamIndex === 0 ? "var(--team-b)" : "var(--team-a)";
  const usScore = viewerTeamIndex === 0 ? match.teamAScore : match.teamBScore;
  const themScore = viewerTeamIndex === 0 ? match.teamBScore : match.teamAScore;

  const detailId = `match-history-detail-${match.id}`;

  return (
    <li
      className="bg-surface border-border overflow-hidden rounded-lg border transition-[border-color,box-shadow] hover:border-border-2 hover:shadow-[0_8px_22px_-10px_rgba(14,58,36,0.30)]"
      data-testid="match-history-row"
      data-match-id={match.id}
    >
      {/* The header is a clickable <div>, NOT a <button>: seat chips inside it
          are links to player profiles, and interactive elements must not nest.
          The real toggle (with the aria state) is the chevron <button> at the
          far end; this div is a convenience click target that mirrors it, so
          keyboard users lose nothing. */}
      <div
        onClick={onToggle}
        className="flex cursor-pointer flex-col gap-3 p-4 md:grid md:grid-cols-[auto_minmax(0,1fr)_auto_auto] md:items-center md:gap-5"
        data-testid="match-history-row-header"
      >
        {/* Date + variant. The variant rides the date block rather than the
            outcome group so it reads as "when and what", and so a Croatian row
            is identifiable at a glance (Epic 12: Croatian rooms must be
            identifiable in the lobby, room preview, and match history).
            Localized through the same lobby.card.* keys the lobby and room-page
            badges use, so the three surfaces cannot drift. */}
        <div className="flex min-w-22 flex-col gap-0.5">
          <span className="text-ink font-display text-[15px] font-semibold tracking-[-0.1px]">
            {formatLocalizedDate(match.completedAt, t, "short")}
          </span>
          <span className="text-ink-mute text-[11.5px] tabular-nums">
            {formatClockTime(match.completedAt)} ·{" "}
            {formatDuration(match.startedAt, match.completedAt, t)}
          </span>
          <span
            className="text-ink-mute mt-0.5 text-[10.5px] font-semibold tracking-[0.4px] uppercase"
            data-testid="match-history-variant"
            data-variant={match.variant}
          >
            {variantLabel(t, match.variant)}
          </span>
        </div>

        {/* Players */}
        <div className="flex min-w-0 flex-col gap-1.5">
          <div className="flex flex-wrap items-center gap-2">
            <SeatChip name={subjectName} team={usTeam} you={subjectIsSelf} />
            <span className="text-ink-mute text-[10px] tracking-[1px] uppercase">
              {t("profile.matchHistory.with")}
            </span>
            <SeatChip
              name={teammate.name}
              bot={teammate.bot}
              userId={teammate.userId}
              team={usTeam}
            />
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-ink-mute text-[10px] tracking-[1px] uppercase">
              {t("profile.matchHistory.versus")}
            </span>
            <SeatChip
              name={opponent1.name}
              bot={opponent1.bot}
              userId={opponent1.userId}
              team={themTeam}
            />
            <span className="text-ink-mute text-[10px] tracking-[1px] uppercase">
              {t("profile.matchHistory.and")}
            </span>
            <SeatChip
              name={opponent2.name}
              bot={opponent2.bot}
              userId={opponent2.userId}
              team={themTeam}
            />
          </div>
        </div>

        {/* Score */}
        <div className="bg-surface-elevated border-border flex items-center gap-2.5 self-start rounded-xl border px-3.5 py-2 md:self-center">
          <span
            className="font-display text-[22px] leading-none font-bold tracking-[-0.4px] tabular-nums"
            style={{ color: usColor }}
            data-team={usTeam === "A" ? "teamA" : "teamB"}
          >
            {usScore}
          </span>
          <span className="text-ink-off font-medium">–</span>
          <span
            className="font-display text-[22px] leading-none font-bold tracking-[-0.4px] tabular-nums"
            style={{ color: themColor }}
            data-team={themTeam === "A" ? "teamA" : "teamB"}
          >
            {themScore}
          </span>
        </div>

        {/* Outcome + chevron */}
        <div className="flex items-center gap-2.5 self-start md:self-center">
          {/* Muted early-end marker for the players who did NOT abandon —
              the abandoner's own row (outcome "abandoned") and legacy rows
              without an attributable abandoner already say it via the chip.
              Wording is distinct per reason: abandonment vs surrender.
              Positive matching on the two known reasons only: an older server
              (or cached response) sends no endReason, and unknown future
              values must render nothing. */}
          {(match.endReason === "abandonment" || match.endReason === "surrender") &&
            match.outcome !== "abandoned" && (
              <span
                className="text-ink-mute text-[11px] italic"
                data-testid="match-history-ended-early"
                data-end-reason={match.endReason}
              >
                {match.endReason === "abandonment"
                  ? t("profile.matchHistory.endedEarlyAbandonment")
                  : t("profile.matchHistory.endedEarlySurrender")}
              </span>
            )}
          {match.hasBots === true && (
            <span
              className="inline-flex h-6 items-center gap-1 rounded-full border px-2 text-[10.5px] font-semibold tracking-[0.4px] uppercase"
              style={{
                color: "var(--bot-deep)",
                borderColor: "var(--bot-edge)",
                background: "var(--bot-soft)",
              }}
              data-testid="match-history-bots-marker"
            >
              <Bot className="size-3" aria-hidden="true" />
              {t("profile.matchHistory.withBots")}
            </span>
          )}
          <OutcomeChip outcome={match.outcome} />
          <button
            type="button"
            onClick={(e) => {
              // The header div above also toggles on click — stop the bubble so
              // one press means one toggle.
              e.stopPropagation();
              onToggle();
            }}
            className="text-ink-mute hover:text-ink focus-visible:ring-ring/50 flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-md transition-colors focus-visible:ring-3 focus-visible:outline-none"
            aria-expanded={isOpen}
            aria-controls={detailId}
            aria-label={
              isOpen ? t("profile.matchHistory.collapseRow") : t("profile.matchHistory.expandRow")
            }
          >
            <ChevronDown
              className={`size-4 transition-transform ${isOpen ? "rotate-180" : ""}`}
              aria-hidden="true"
            />
          </button>
        </div>
      </div>

      {isOpen && (
        <div
          id={detailId}
          data-testid="match-history-detail"
          data-match-id={match.id}
          className="border-border border-t px-4 pt-3 pb-4.5"
          style={{ background: "rgba(14,58,36,0.025)" }}
        >
          <HandsGrid
            hands={match.hands}
            viewerTeamIndex={viewerTeamIndex}
            layout={handsLayout}
            usFinal={usScore}
            themFinal={themScore}
          />
          {footer}
        </div>
      )}
    </li>
  );
}
