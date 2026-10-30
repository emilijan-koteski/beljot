import { useTranslation } from "react-i18next";

import { SeasonArchiveRow } from "@/shared/components/season/SeasonArchiveRow";
import { TierBadge } from "@/shared/components/season/TierBadge";
import { useSeasonArchiveQuery } from "@/shared/hooks/queries/useSeasonArchive";
import {
  normalizeSeasonDivision,
  normalizeSeasonTier,
  SEASON_TIER_COLOR,
  SEASON_TIER_LINE,
  seasonRankLabel,
  seasonSpOrZero,
} from "@/shared/lib/seasonTier";
import type { SeasonRank } from "@/shared/types/apiTypes";

import { SectionHeader } from "./SectionHeader";

type Props = {
  userId: number | undefined;
  seasonRank: SeasonRank | null | undefined;
  showCurrentRank?: boolean;
};

/**
 * Profile season section: current-rank chip and prior-season archive, each hidden when empty.
 * The archive gates on its rows, not on isError, so a failed background refetch never hides it.
 */
export function SeasonSection({ userId, seasonRank, showCurrentRank = true }: Props) {
  const { t } = useTranslation();
  const archive = useSeasonArchiveQuery(userId);

  const items = archive.data?.items ?? [];
  const hasRank = showCurrentRank && seasonRank !== null && seasonRank !== undefined;
  const hasArchive = items.length > 0;

  if (!hasRank && !hasArchive) return null;

  let chip = null;
  if (hasRank) {
    const sp = seasonSpOrZero(seasonRank.sp);
    const tier = normalizeSeasonTier(seasonRank.tier, sp);
    const rankName = seasonRankLabel(t, tier, normalizeSeasonDivision(tier, seasonRank.division));
    chip = (
      <div
        data-testid="profile-season"
        data-tier={tier}
        className="bg-surface-elevated flex items-center gap-2.5 rounded-md border px-3 py-2"
        style={{ borderColor: SEASON_TIER_LINE[tier] }}
      >
        <span className="sr-only">
          {t("season.archive.currentAria", {
            season: seasonRank.seasonName,
            tier: rankName,
            sp: sp.toLocaleString(),
          })}
        </span>
        <TierBadge tier={tier} size="sm" data-testid="profile-season-badge" />
        <div aria-hidden="true" className="flex min-w-0 flex-col">
          <span
            data-testid="profile-season-tier"
            className="text-sm leading-tight font-semibold"
            style={{ color: SEASON_TIER_COLOR[tier] }}
          >
            {rankName}
          </span>
          <span className="text-ink-dim text-xs leading-tight tabular-nums">
            {t("season.banner.sp", { sp: sp.toLocaleString() })}
            <span className="text-ink-mute"> · {seasonRank.seasonName}</span>
          </span>
        </div>
      </div>
    );
  }

  return (
    <section className="mb-5" data-testid="profile-season-section">
      <SectionHeader
        eyebrow={t("season.archive.eyebrow")}
        title={t("season.archive.title")}
        sub={t(chip ? "season.archive.sub" : "season.archive.subArchiveOnly")}
        right={chip}
      />

      {hasArchive && (
        <ul
          data-testid="prior-season-archive"
          aria-label={t("season.archive.listLabel")}
          className="m-0 flex list-none flex-col gap-1.5 p-0"
        >
          {items.map((row) => (
            <SeasonArchiveRow
              key={row.seasonId}
              seasonName={row.seasonName}
              sp={row.sp}
              tier={row.tier}
              division={row.division}
              gamesPlayed={row.gamesPlayed}
            />
          ))}
        </ul>
      )}
    </section>
  );
}
