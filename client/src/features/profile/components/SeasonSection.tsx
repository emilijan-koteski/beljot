import { useTranslation } from "react-i18next";

import { SeasonArchiveRow } from "@/shared/components/season/SeasonArchiveRow";
import { TierBadge } from "@/shared/components/season/TierBadge";
import { useSeasonArchiveQuery } from "@/shared/hooks/queries/useSeasonArchive";
import {
  normalizeSeasonTier,
  SEASON_TIER_COLOR,
  SEASON_TIER_LINE,
  seasonSpOrZero,
} from "@/shared/lib/seasonTier";
import type { SeasonRank } from "@/shared/types/apiTypes";

import { SectionHeader } from "./SectionHeader";

type Props = {
  /** The SUBJECT's id — the route param on the public page, the viewer's own
   *  id on the self page. `undefined` while unresolved (the archive waits). */
  userId: number | undefined;
  /** The profile response's seasonRank block. `null` = has not played this
   *  season (the chip hides); `undefined` = the profile has not resolved. */
  seasonRank: SeasonRank | null | undefined;
};

/**
 * The profile's season surface (Story 13.3): the current-rank chip plus the
 * prior-season archive, filling the slot both profile pages reserved when
 * Epic 11 shipped.
 *
 * HIDDEN-WHEN-EMPTY, BOTH HALVES INDEPENDENTLY — and that is a product rule,
 * not a styling choice (epic AC: the archive section is omitted from the DOM
 * entirely for players with no played seasons, never shown as an empty state):
 *
 *   - the chip (`profile-season`) renders only when `seasonRank` is non-null;
 *     a played season at 0 SP IS a rank (Iron), so the gate is null, never SP
 *     truthiness.
 *   - the archive (`prior-season-archive`) renders only when the archive query
 *     resolved with rows. Loading and error states render NOTHING rather than
 *     a skeleton or an error line: the section is supplementary history, and a
 *     placeholder here would shift the whole profile for a section most
 *     players do not have yet.
 *
 * When neither half has anything, the component contributes NOTHING to the
 * DOM — the pre-13.3 pages' graceful absence, preserved.
 */
export function SeasonSection({ userId, seasonRank }: Props) {
  const { t } = useTranslation();
  const archive = useSeasonArchiveQuery(userId);

  const items = archive.data?.items ?? [];
  const hasRank = seasonRank !== null && seasonRank !== undefined;
  // CACHED ROWS SURVIVE A FAILED REFETCH. React Query keeps `data` when a
  // background or focus refetch fails, so gating on `isError` made a network
  // blip delete an already-rendered archive and jump the profile's layout. The
  // row count is the only gate that matters: a first load that fails has no
  // rows anyway, which is the same hidden section either way.
  const hasArchive = items.length > 0;

  if (!hasRank && !hasArchive) return null;

  let chip = null;
  if (hasRank) {
    const sp = seasonSpOrZero(seasonRank.sp);
    // Guarded, not trusted: an unrecognised token from a newer server falls
    // back to the SP's own bucket rather than a missing i18n key.
    const tier = normalizeSeasonTier(seasonRank.tier, sp);
    const tierName = t(`season.tier.${tier}`);
    chip = (
      <div
        data-testid="profile-season"
        data-tier={tier}
        className="bg-surface-elevated flex items-center gap-2.5 rounded-[10px] border px-3 py-2"
        style={{ borderColor: SEASON_TIER_LINE[tier] }}
      >
        <span className="sr-only">
          {t("season.archive.currentAria", {
            season: seasonRank.seasonName,
            tier: tierName,
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
            {tierName}
          </span>
          <span className="text-ink-dim text-[11px] leading-tight tabular-nums">
            {t("season.banner.sp", { sp: sp.toLocaleString() })}
            {/* The machine-stable window token, VERBATIM — an identifier, not
                translated copy. */}
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
        sub={t("season.archive.sub")}
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
              gamesPlayed={row.gamesPlayed}
            />
          ))}
        </ul>
      )}
    </section>
  );
}
