import { Bot, ChevronDown } from "lucide-react";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import type { MatchHandView, MatchListItem, MatchOutcome } from "@/shared/api/matches";
import { MatchRulesPopover } from "@/shared/components/roomRules/RoomRules";
import { Badge } from "@/shared/components/ui/badge";
import { useMediaQuery } from "@/shared/hooks/useMediaQuery";
import { botDisplayName } from "@/shared/lib/botName";
import { formatClockTime, formatLocalizedDate } from "@/shared/lib/formatDate";
import { variantLabel } from "@/shared/lib/roomLabels";
import { cn } from "@/shared/lib/utils";

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
  const cfg: Record<MatchOutcome, { tone: "accent" | "danger" | "brass"; labelKey: string }> = {
    win: { tone: "accent", labelKey: "profile.matchHistory.outcomeWin" },
    loss: { tone: "danger", labelKey: "profile.matchHistory.outcomeLoss" },
    abandoned: { tone: "brass", labelKey: "profile.matchHistory.outcomeAbandoned" },
  };
  const c = cfg[outcome];
  return (
    <Badge tone={c.tone} data-testid="match-history-outcome" data-outcome={outcome}>
      {t(c.labelKey)}
    </Badge>
  );
}

/** Hand layout: "auto" picks table or cards from the viewport, "stacked" always uses cards. */
type HandsLayout = "auto" | "stacked";

interface HandsGridProps {
  hands: MatchHandView[];
  viewerTeamIndex: 0 | 1;
  layout: HandsLayout;
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

/** Renders the capot, last-trick and failed-contract badges for a hand as a fragment. */
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
          className="bg-accent-soft text-accent border-accent/33 rounded-md border px-1.5 py-0.5 text-2xs font-semibold tracking-wider"
          data-testid="match-history-hand-capot"
        >
          {t("profile.matchHistory.hand.note.capot", { points: h.capotBonus })} ·{" "}
          {teamWord(capotIsUs)}
        </span>
      )}
      {!h.capot && h.lastTrickBonus > 0 && (
        <span
          className="bg-surface-3 border-border rounded-md border px-1.5 py-0.5 text-2xs font-semibold tracking-wider"
          style={{ color: lastTrickIsUs ? usColor : themColor }}
          data-testid="match-history-hand-last-trick"
        >
          {t("profile.matchHistory.hand.note.lastTrick", { points: h.lastTrickBonus })} ·{" "}
          {teamWord(lastTrickIsUs)}
        </span>
      )}
      {h.failedContract && (
        <span
          className="bg-danger/10 text-danger border-danger/30 rounded-md border px-1.5 py-0.5 text-2xs font-semibold tracking-wider"
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

  const notesFor = (h: MatchHandView, d: ReturnType<typeof derive>) =>
    handHasNotes(h) && (
      <span className="mt-1 flex flex-wrap gap-1">
        <HandNotes
          hand={h}
          capotIsUs={d.capotIsUs}
          lastTrickIsUs={d.lastTrickIsUs}
          contractingIsUs={d.contractingIsUs}
          usColor={usColor}
          themColor={themColor}
        />
      </span>
    );
  const total = (value: number, won: boolean, color: string) => (
    <span
      className="text-right text-sm font-bold tabular-nums"
      style={{ color: won ? color : "var(--ink-mute)" }}
    >
      {value}
    </span>
  );
  const number = (n: number) => (
    <span className="text-ink-mute font-mono text-2xs font-semibold tabular-nums">
      {String(n).padStart(2, "0")}
    </span>
  );
  const head = "text-brass-deep font-mono text-2xs font-semibold tracking-caps uppercase";

  if (isCompact) {
    const cols = "grid-cols-[1.5rem_minmax(0,1fr)_2.75rem_2.75rem]";
    return (
      <div className="flex flex-col" data-testid="match-history-hands-grid">
        <div className={cn("grid items-center gap-2 px-1 pb-1", cols, head)}>
          <span>{t("profile.matchHistory.hand.col.number")}</span>
          <span>{t("profile.matchHistory.hand.col.hand")}</span>
          <span className="text-right" style={{ color: usColor }}>
            {t("team.us")}
          </span>
          <span className="text-right" style={{ color: themColor }}>
            {t("team.them")}
          </span>
        </div>
        <div className="divide-brass/20 border-brass/20 flex flex-col divide-y border-t">
          {hands.map((h) => {
            const d = derive(h);
            return (
              <div
                key={h.handNumber}
                className={cn("grid items-start gap-2 px-1 py-1.5", cols)}
                data-testid="match-history-hand-row"
                data-hand-number={h.handNumber}
              >
                {number(h.handNumber)}
                <span className="flex min-w-0 flex-col leading-tight">
                  <span className="text-ink-dim text-xs">{d.desc}</span>
                  <span className="text-ink-mute text-2xs tabular-nums">
                    {t("profile.matchHistory.hand.col.cards")} {d.us.card}/{d.them.card} ·{" "}
                    {t("profile.matchHistory.hand.col.declarations")} {d.us.decl}/{d.them.decl}
                  </span>
                  {notesFor(h, d)}
                </span>
                {total(d.us.total, d.usWonHand, usColor)}
                {total(d.them.total, !d.usWonHand, themColor)}
              </div>
            );
          })}
          {hasUnscoredHand && (
            <div
              className={cn("grid items-start gap-2 px-1 py-1.5", cols)}
              data-testid="match-history-unscored-hand"
            >
              {number(hands.length + 1)}
              <span className="text-ink-dim text-xs italic">
                {t("profile.matchHistory.hand.unfinished")}
              </span>
              <span className="text-ink text-right text-sm font-bold tabular-nums">
                {unscoredUs}
              </span>
              <span className="text-ink text-right text-sm font-bold tabular-nums">
                {unscoredThem}
              </span>
            </div>
          )}
        </div>
      </div>
    );
  }

  const cols = "grid-cols-[1.75rem_minmax(8rem,1fr)_3rem_3rem_5rem_6.5rem_minmax(7rem,1fr)]";

  return (
    <div className="overflow-x-auto" data-testid="match-history-hands-grid">
      <div className="min-w-140">
        <div className={cn("grid items-center gap-3 px-1 pb-1", cols, head)}>
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
        <div className="divide-brass/20 border-brass/20 flex flex-col divide-y border-t">
          {hands.map((h) => {
            const d = derive(h);
            return (
              <div
                key={h.handNumber}
                className={cn("grid items-center gap-3 px-1 py-1.5", cols)}
                data-testid="match-history-hand-row"
                data-hand-number={h.handNumber}
              >
                {number(h.handNumber)}
                <span className="text-ink-dim text-xs">{d.desc}</span>
                {total(d.us.total, d.usWonHand, usColor)}
                {total(d.them.total, !d.usWonHand, themColor)}
                <span className="text-ink-dim text-xs tabular-nums">
                  {d.us.card} <span className="text-faint">/</span> {d.them.card}
                </span>
                <span className="text-ink-dim text-xs tabular-nums">
                  {d.us.decl} <span className="text-faint">/</span> {d.them.decl}
                </span>
                <span className="flex flex-wrap gap-1">
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
              className={cn("grid items-center gap-3 px-1 py-1.5", cols)}
              data-testid="match-history-unscored-hand"
            >
              {number(hands.length + 1)}
              <span className="text-ink-dim text-xs italic">
                {t("profile.matchHistory.hand.unfinished")}
              </span>
              <span className="text-ink text-right text-sm font-bold tabular-nums">
                {unscoredUs}
              </span>
              <span className="text-ink text-right text-sm font-bold tabular-nums">
                {unscoredThem}
              </span>
              <span />
              <span />
              <span />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

interface MatchStatsCardProps {
  match: MatchListItem;
  subjectIsSelf?: boolean;
  isOpen: boolean;
  onToggle: () => void;
  handsLayout?: HandsLayout;
  linkPlayers?: boolean;
  footer?: ReactNode;
  rulesZIndex?: number;
}

/**
 * One match as a compact row: outcome, score and date, the four players, then variant, mode and length, opening
 * to a per-hand table. It sits in a list card its caller draws; open state is the caller's too.
 */
export function MatchStatsCard({
  match,
  subjectIsSelf = true,
  isOpen,
  onToggle,
  handsLayout = "auto",
  linkPlayers = true,
  footer,
  rulesZIndex,
}: MatchStatsCardProps) {
  const { t } = useTranslation();

  const teammateSeat = (match.viewerSeat + 2) % 4;
  const opp1Seat = (match.viewerSeat + 1) % 4;
  const opp2Seat = (match.viewerSeat + 3) % 4;
  const seatChipProps = (
    seat: number,
  ): { name: string; bot: boolean; avatarUrl?: string | null; userId?: number } => {
    const p = match.players.find((pl) => pl.seat === seat);
    if (!p) return { name: "—", bot: false };
    if (p.isBot === true) return { name: botDisplayName(t, p.seat), bot: true };
    return {
      name: p.username || "—",
      bot: false,
      avatarUrl: p.avatarUrl,
      userId: linkPlayers && p.userId > 0 && p.username !== "" ? p.userId : undefined,
    };
  };
  const teammate = seatChipProps(teammateSeat);
  const opponent1 = seatChipProps(opp1Seat);
  const opponent2 = seatChipProps(opp2Seat);
  const subject = match.players.find((p) => p.seat === match.viewerSeat);
  const subjectName = subject?.username || "—";

  const viewerTeamIndex: 0 | 1 = match.viewerSeat % 2 === 0 ? 0 : 1;
  const usTeam: "A" | "B" = viewerTeamIndex === 0 ? "A" : "B";
  const themTeam: "A" | "B" = viewerTeamIndex === 0 ? "B" : "A";
  const usColor = viewerTeamIndex === 0 ? "var(--team-a)" : "var(--team-b)";
  const themColor = viewerTeamIndex === 0 ? "var(--team-b)" : "var(--team-a)";
  const usScore = viewerTeamIndex === 0 ? match.teamAScore : match.teamBScore;
  const themScore = viewerTeamIndex === 0 ? match.teamBScore : match.teamAScore;

  const detailId = `match-history-detail-${match.id}`;
  const endedEarly =
    (match.endReason === "abandonment" || match.endReason === "surrender") &&
    match.outcome !== "abandoned";
  const joiner = "text-ink-mute text-2xs tracking-wider lowercase";

  return (
    <li
      className={cn("transition-colors", isOpen && "bg-brass/5")}
      data-testid="match-history-row"
      data-match-id={match.id}
    >
      <div
        onClick={onToggle}
        className="hover:bg-brass/5 flex cursor-pointer flex-col gap-1.5 px-3.5 py-2.5 transition-colors"
        data-testid="match-history-row-header"
      >
        <div className="flex items-center gap-2">
          <OutcomeChip outcome={match.outcome} />
          <span className="border-brass/40 bg-brass/10 inline-flex items-baseline gap-1 rounded-md border px-1.5 py-px tabular-nums">
            <span
              className="text-base leading-tight font-bold"
              style={{ color: usColor }}
              data-team={usTeam === "A" ? "teamA" : "teamB"}
            >
              {usScore}
            </span>
            <span className="text-brass-deep text-xs">–</span>
            <span
              className="text-base leading-tight font-bold"
              style={{ color: themColor }}
              data-team={themTeam === "A" ? "teamA" : "teamB"}
            >
              {themScore}
            </span>
          </span>
          {match.hasBots === true && (
            <Badge
              tone="bot"
              size="sm"
              icon={<Bot aria-hidden="true" />}
              data-testid="match-history-bots-marker"
            >
              {t("bots.tag")}
            </Badge>
          )}
          <span className="text-ink-mute ml-auto text-xs whitespace-nowrap tabular-nums">
            {formatLocalizedDate(match.completedAt, t, "short")} ·{" "}
            {formatClockTime(match.completedAt)}
          </span>
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              onToggle();
            }}
            className="text-brass-deep hover:text-ink focus-visible:ring-ring/70 -mr-1 flex size-6 shrink-0 cursor-pointer items-center justify-center rounded-md transition-colors focus-visible:ring-3 focus-visible:outline-none"
            aria-expanded={isOpen}
            aria-controls={detailId}
            aria-label={
              isOpen ? t("profile.matchHistory.collapseRow") : t("profile.matchHistory.expandRow")
            }
          >
            <ChevronDown
              className={cn("size-4 transition-transform", isOpen && "rotate-180")}
              aria-hidden="true"
            />
          </button>
        </div>

        <div className="flex flex-wrap items-center gap-x-4 gap-y-1.5">
          <div className="flex flex-wrap items-center gap-x-1.5 gap-y-1">
            <SeatChip
              name={subjectName}
              avatarUrl={subject?.avatarUrl}
              team={usTeam}
              you={subjectIsSelf}
            />
            <span className={joiner}>{t("profile.matchHistory.with")}</span>
            <SeatChip
              name={teammate.name}
              bot={teammate.bot}
              avatarUrl={teammate.avatarUrl}
              userId={teammate.userId}
              team={usTeam}
            />
            <span className={joiner}>{t("profile.matchHistory.versus")}</span>
            <SeatChip
              name={opponent1.name}
              bot={opponent1.bot}
              avatarUrl={opponent1.avatarUrl}
              userId={opponent1.userId}
              team={themTeam}
            />
            <span className={joiner}>{t("profile.matchHistory.and")}</span>
            <SeatChip
              name={opponent2.name}
              bot={opponent2.bot}
              avatarUrl={opponent2.avatarUrl}
              userId={opponent2.userId}
              team={themTeam}
            />
          </div>

          <div className="text-ink-mute flex flex-wrap items-center gap-x-1.5 gap-y-0.5 text-2xs">
            <span
              className="font-semibold tracking-wider uppercase"
              data-testid="match-history-variant"
              data-variant={match.variant}
            >
              {variantLabel(t, match.variant)}
            </span>
            <span aria-hidden="true">·</span>
            <span className="tabular-nums">{match.matchMode}</span>
            <span aria-hidden="true">·</span>
            <span className="tabular-nums">
              {formatDuration(match.startedAt, match.completedAt, t)}
            </span>
            <MatchRulesPopover rules={match} zIndex={rulesZIndex} className="-my-1 ml-0.5" />
            {endedEarly && (
              <span
                className="italic"
                data-testid="match-history-ended-early"
                data-end-reason={match.endReason}
              >
                {match.endReason === "abandonment"
                  ? t("profile.matchHistory.endedEarlyAbandonment")
                  : t("profile.matchHistory.endedEarlySurrender")}
              </span>
            )}
          </div>
        </div>
      </div>

      {isOpen && (
        <div
          id={detailId}
          data-testid="match-history-detail"
          data-match-id={match.id}
          className="border-brass/25 flex flex-col gap-3 border-t px-3.5 pt-2 pb-3"
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
