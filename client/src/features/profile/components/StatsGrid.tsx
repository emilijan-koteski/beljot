import { useTranslation } from "react-i18next";

import { SectionHeader } from "./SectionHeader";

type StatsGridProps = {
  games: number;
  wins: number;
  losses: number;
  abandoned: number;
};

type Tone = "neutral" | "accent" | "muted";

function StatTile({
  testId,
  label,
  value,
  tone,
}: {
  testId: string;
  label: string;
  value: number;
  tone: Tone;
}) {
  const valueColor =
    tone === "accent" ? "var(--accent)" : tone === "muted" ? "var(--ink-dim)" : "var(--ink)";
  return (
    <div
      className="bg-surface border-border flex flex-col gap-1 rounded-lg border p-5"
      data-testid={testId}
      data-value={String(value)}
    >
      <span className="text-brass-deep font-mono text-[11px] font-semibold tracking-[2px] uppercase">
        {label}
      </span>
      <span
        className="font-display mt-1 text-[44px] leading-none font-bold tracking-[-1.4px] tabular-nums"
        style={{ color: valueColor }}
      >
        {value}
      </span>
    </div>
  );
}

function OutcomeSplitBar({ wins, losses, abandoned }: Omit<StatsGridProps, "games">) {
  const { t } = useTranslation();
  const total = wins + losses + abandoned;
  if (total === 0) return null;
  const pct = (n: number) => `${(n / total) * 100}%`;

  return (
    <div
      className="bg-surface border-border mt-3 flex flex-col gap-2.5 rounded-lg border px-5 py-4"
      data-testid="profile-outcome-split"
    >
      <div className="text-ink-dim flex flex-wrap items-center gap-3.5 text-xs">
        <span className="text-brass-deep font-mono text-[10.5px] font-semibold tracking-[2px] uppercase">
          {t("profile.stats.outcomeSplit")}
        </span>
        <LegendDot color="var(--accent)" label={t("profile.stats.legendWins", { count: wins })} />
        <LegendDot
          color="var(--ink-off)"
          label={t("profile.stats.legendLosses", { count: losses })}
        />
        <LegendDot
          color="var(--brass-deep)"
          label={t("profile.stats.legendAbandoned", { count: abandoned })}
        />
      </div>
      <div
        className="flex h-3.5 overflow-hidden rounded-full"
        style={{ background: "var(--surface-3)" }}
      >
        <span style={{ width: pct(wins), background: "var(--accent)" }} />
        <span style={{ width: pct(losses), background: "var(--ink-off)" }} />
        <span style={{ width: pct(abandoned), background: "var(--brass-deep)" }} />
      </div>
    </div>
  );
}

function LegendDot({ color, label }: { color: string; label: string }) {
  return (
    <span className="inline-flex items-center gap-1.5">
      <span className="size-2 rounded-xs" style={{ background: color }} />
      <span className="text-ink-dim">{label}</span>
    </span>
  );
}

/**
 * Career stats section: four warm tiles (games / wins / losses / abandoned)
 * plus a stacked outcome-split bar that visually anchors the numbers.
 */
export function StatsGrid({ games, wins, losses, abandoned }: StatsGridProps) {
  const { t } = useTranslation();
  return (
    <section className="mb-5" data-testid="profile-stats">
      <SectionHeader
        eyebrow={t("profile.stats.eyebrow")}
        title={t("profile.statsHeading")}
        sub={t("profile.stats.sub")}
      />
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <StatTile
          testId="profile-stat-games-played"
          label={t("profile.stats.totalGamesPlayed")}
          value={games}
          tone="neutral"
        />
        <StatTile
          testId="profile-stat-wins"
          label={t("profile.stats.wins")}
          value={wins}
          tone="accent"
        />
        <StatTile
          testId="profile-stat-losses"
          label={t("profile.stats.losses")}
          value={losses}
          tone="muted"
        />
        <StatTile
          testId="profile-stat-abandoned"
          label={t("profile.stats.abandoned")}
          value={abandoned}
          tone="muted"
        />
      </div>
      <OutcomeSplitBar wins={wins} losses={losses} abandoned={abandoned} />
    </section>
  );
}
