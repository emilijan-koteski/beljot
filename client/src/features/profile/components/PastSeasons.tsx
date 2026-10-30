import { Medal } from "lucide-react";
import { useTranslation } from "react-i18next";

import { EmptyState } from "@/shared/components/EmptyState";
import { SeasonArchiveRow } from "@/shared/components/season/SeasonArchiveRow";
import { Panel } from "@/shared/components/ui/panel";
import { Skeleton } from "@/shared/components/ui/skeleton";
import { useSeasonArchiveQuery } from "@/shared/hooks/queries/useSeasonArchive";

type Props = {
  userId: number | undefined;
  compact?: boolean;
};

/**
 * Past seasons, newest first, each with its final rank, SP and record; it gates on rows, so a failed refetch never hides it.
 * `compact` is the desktop sidebar panel, which stays away without history, while the full list says so instead.
 */
export function PastSeasons({ userId, compact = false }: Props) {
  const { t } = useTranslation();
  const archive = useSeasonArchiveQuery(userId);
  const items = archive.data?.items ?? [];

  if (compact && items.length === 0) return null;

  let body;
  if (items.length > 0) {
    body = (
      <ul
        data-testid="prior-season-archive"
        aria-label={t("season.archive.listLabel")}
        className="divide-border m-0 flex list-none flex-col divide-y p-0"
      >
        {items.map((row) => (
          <SeasonArchiveRow
            key={row.seasonId}
            seasonName={row.seasonName}
            sp={row.sp}
            tier={row.tier}
            division={row.division}
            gamesPlayed={row.gamesPlayed}
            gamesWon={row.gamesWon}
          />
        ))}
      </ul>
    );
  } else if (archive.isPending) {
    body = <Skeleton className="my-3 h-12 rounded-md" />;
  } else {
    body = (
      <EmptyState testId="profile-seasons-empty" className="my-3">
        {t("profile.seasons.empty")}
      </EmptyState>
    );
  }

  if (compact) {
    return (
      <Panel icon={<Medal />} title={t("season.archive.listLabel")} testId="profile-season-section">
        {body}
      </Panel>
    );
  }
  return (
    <section
      className="bg-surface border-border rounded-xl border px-4 shadow-xs"
      data-testid="profile-season-section"
    >
      {body}
    </section>
  );
}
