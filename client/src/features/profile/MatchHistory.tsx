import { Bot, ChevronDown } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import type {
  MatchFilter,
  MatchHandView,
  MatchListItem,
  MatchOutcome,
  MatchSort,
} from "@/shared/api/matches";
import { useUserMatchesInfiniteQuery } from "@/shared/hooks/queries/useMatches";
import { useMediaQuery } from "@/shared/hooks/useMediaQuery";
import { botDisplayName } from "@/shared/lib/botName";
import { formatLocalizedDate } from "@/shared/lib/formatDate";

import { HistoryFilters } from "./components/HistoryFilters";
import { SeatChip } from "./components/SeatChip";
import { SectionHeader } from "./components/SectionHeader";
import { formatClockTime } from "./lib/format";

interface MatchHistoryProps {
  userId: number | undefined;
  /** Per-outcome counts for the filter chips, sourced from profile stats. */
  counts: Record<MatchFilter, number>;
}

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

interface HandsGridProps {
  hands: MatchHandView[];
  viewerTeamIndex: 0 | 1;
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

function HandsGrid({ hands, viewerTeamIndex }: HandsGridProps) {
  const { t } = useTranslation();
  // Phones get a stacked per-hand card layout instead of the dense 7-column
  // table — the table needs ~620px and would otherwise force a horizontal
  // scroll inside the (much narrower) expanded row. Rendering exactly one
  // layout (not both behind `hidden`/`md:block`) keeps the per-hand testids
  // unique. `useMediaQuery` returns false without `matchMedia` (tests/SSR), so
  // those environments get the desktop table.
  const isCompact = useMediaQuery("(max-width: 767px)");

  if (hands.length === 0) {
    return (
      <p className="text-ink-dim mt-1 text-sm italic" data-testid="match-history-no-hands">
        {t("profile.matchHistory.noHandDetails")}
      </p>
    );
  }

  const usColor = viewerTeamIndex === 0 ? "var(--team-a)" : "var(--team-b)";
  const themColor = viewerTeamIndex === 0 ? "var(--team-b)" : "var(--team-a)";

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
      </div>
    </div>
  );
}

interface MatchRowProps {
  match: MatchListItem;
  username: string;
  isOpen: boolean;
  onToggle: () => void;
}

function MatchRow({ match, username, isOpen, onToggle }: MatchRowProps) {
  const { t } = useTranslation();

  const teammateSeat = (match.viewerSeat + 2) % 4;
  const opp1Seat = (match.viewerSeat + 1) % 4;
  const opp2Seat = (match.viewerSeat + 3) % 4;
  // Bot seats carry an empty username with isBot:true — render the localized
  // seat-derived bot name (and the bot glyph in the chip avatar) instead of
  // a blank chip.
  const seatChipProps = (seat: number): { name: string; bot: boolean } => {
    const p = match.players.find((pl) => pl.seat === seat);
    if (!p) return { name: "—", bot: false };
    if (p.isBot === true) return { name: botDisplayName(t, p.seat), bot: true };
    return { name: p.username || "—", bot: false };
  };
  const teammate = seatChipProps(teammateSeat);
  const opponent1 = seatChipProps(opp1Seat);
  const opponent2 = seatChipProps(opp2Seat);

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
      <button
        type="button"
        onClick={onToggle}
        className="flex w-full cursor-pointer flex-col gap-3 p-4 text-left md:grid md:grid-cols-[auto_minmax(0,1fr)_auto_auto] md:items-center md:gap-5"
        aria-expanded={isOpen}
        aria-controls={detailId}
        aria-label={
          isOpen ? t("profile.matchHistory.collapseRow") : t("profile.matchHistory.expandRow")
        }
      >
        {/* Date */}
        <div className="flex min-w-22 flex-col gap-0.5">
          <span className="text-ink font-display text-[15px] font-semibold tracking-[-0.1px]">
            {formatLocalizedDate(match.completedAt, t, "short")}
          </span>
          <span className="text-ink-mute text-[11.5px] tabular-nums">
            {formatClockTime(match.completedAt)} ·{" "}
            {formatDuration(match.startedAt, match.completedAt, t)}
          </span>
        </div>

        {/* Players */}
        <div className="flex min-w-0 flex-col gap-1.5">
          <div className="flex flex-wrap items-center gap-2">
            <SeatChip name={username} team={usTeam} you />
            <span className="text-ink-mute text-[10px] tracking-[1px] uppercase">
              {t("profile.matchHistory.with")}
            </span>
            <SeatChip name={teammate.name} bot={teammate.bot} team={usTeam} />
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-ink-mute text-[10px] tracking-[1px] uppercase">
              {t("profile.matchHistory.versus")}
            </span>
            <SeatChip name={opponent1.name} bot={opponent1.bot} team={themTeam} />
            <span className="text-ink-mute text-[10px] tracking-[1px] uppercase">
              {t("profile.matchHistory.and")}
            </span>
            <SeatChip name={opponent2.name} bot={opponent2.bot} team={themTeam} />
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
              className="text-ink-dim inline-flex h-6 items-center gap-1 rounded-full border px-2 text-[10.5px] font-semibold tracking-[0.4px] uppercase"
              style={{ borderColor: "var(--border)", background: "var(--surface-3)" }}
              data-testid="match-history-bots-marker"
            >
              <Bot className="size-3" aria-hidden="true" />
              {t("profile.matchHistory.withBots")}
            </span>
          )}
          <OutcomeChip outcome={match.outcome} />
          <ChevronDown
            className={`text-ink-mute size-4 transition-transform ${isOpen ? "rotate-180" : ""}`}
            aria-hidden="true"
          />
        </div>
      </button>

      {isOpen && (
        <div
          id={detailId}
          data-testid="match-history-detail"
          data-match-id={match.id}
          className="border-border border-t px-4 pt-3 pb-4.5"
          style={{ background: "rgba(14,58,36,0.025)" }}
        >
          <HandsGrid hands={match.hands} viewerTeamIndex={viewerTeamIndex} />
        </div>
      )}
    </li>
  );
}

export function MatchHistory({ userId, counts }: MatchHistoryProps) {
  const { t } = useTranslation();
  const [filter, setFilter] = useState<MatchFilter>("all");
  const [sort, setSort] = useState<MatchSort>("new");
  const [openIds, setOpenIds] = useState<Set<number>>(new Set());

  const query = useUserMatchesInfiniteQuery(userId, { outcome: filter, sort });

  const items = useMemo<MatchListItem[]>(() => {
    if (!query.data) return [];
    return query.data.pages.flatMap((p) => p.items);
  }, [query.data]);

  // The viewer's own username — derived from any row's viewer seat so the
  // "YOU" seat chip shows the real name without an extra dependency.
  const username = useMemo(() => {
    for (const m of items) {
      const me = m.players.find((p) => p.seat === m.viewerSeat);
      if (me?.username) return me.username;
    }
    return "";
  }, [items]);

  const toggleOpen = (id: number) => {
    setOpenIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const header = (
    <SectionHeader
      eyebrow={t("profile.matchHistory.eyebrow")}
      title={t("profile.matchHistory.title")}
      sub={t("profile.matchHistory.sub")}
    />
  );

  // Truly empty (no games at all) — show the onboarding empty state, no filters.
  if (counts.all === 0 && !query.isPending && !query.isError) {
    return (
      <section data-testid="match-history">
        {header}
        <div
          className="bg-surface border-border space-y-3 rounded-lg border border-dashed p-10 text-center text-sm"
          data-testid="match-history-empty"
        >
          <p className="text-ink-dim m-0">{t("profile.matchHistory.empty")}</p>
          <Link
            to="/lobby"
            className="text-accent inline-flex items-center underline-offset-2 hover:underline"
            data-testid="match-history-empty-cta"
          >
            {t("profile.matchHistory.emptyCta")}
          </Link>
        </div>
      </section>
    );
  }

  const filters = (
    <HistoryFilters
      filter={filter}
      onFilterChange={setFilter}
      counts={counts}
      sort={sort}
      onSortChange={setSort}
    />
  );

  let body;
  if (query.isPending) {
    body = (
      <div className="space-y-2.5" data-testid="match-history-loading">
        <div className="bg-surface h-20 animate-pulse rounded-lg" />
        <div className="bg-surface h-20 animate-pulse rounded-lg" />
        <div className="bg-surface h-20 animate-pulse rounded-lg" />
      </div>
    );
  } else if (query.isError) {
    body = (
      <p className="text-destructive text-sm" data-testid="match-history-error">
        {t("profile.matchHistory.error")}
      </p>
    );
  } else if (items.length === 0) {
    body = (
      <div
        className="bg-surface border-border rounded-lg border border-dashed p-10 text-center text-sm"
        data-testid="match-history-empty-filtered"
      >
        <p className="text-ink-dim m-0">{t("profile.matchHistory.emptyFiltered")}</p>
      </div>
    );
  } else {
    const total = query.data?.pages[0]?.total ?? 0;
    const showLoadMore = items.length < total;
    body = (
      <>
        <ul className="m-0 flex list-none flex-col gap-2.5 p-0" data-testid="match-history-list">
          {items.map((match) => (
            <MatchRow
              key={match.id}
              match={match}
              username={username}
              isOpen={openIds.has(match.id)}
              onToggle={() => toggleOpen(match.id)}
            />
          ))}
        </ul>
        {showLoadMore && (
          <button
            type="button"
            onClick={() => query.fetchNextPage()}
            disabled={query.isFetchingNextPage}
            className="bg-surface border-border text-ink hover:bg-surface-elevated mt-2.5 w-full rounded-lg border px-4 py-2 text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-60"
            data-testid="match-history-load-more"
          >
            {query.isFetchingNextPage
              ? t("profile.matchHistory.loading")
              : t("profile.matchHistory.loadMore")}
          </button>
        )}
        <p className="text-ink-mute mt-4 text-center text-xs" data-testid="match-history-count">
          {t("profile.matchHistory.showing", { shown: items.length, total })}
        </p>
      </>
    );
  }

  return (
    <section data-testid="match-history">
      {header}
      {filters}
      {body}
    </section>
  );
}
