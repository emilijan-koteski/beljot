import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import { SeasonStandingCard } from "@/shared/components/season/SeasonStandingCard";
import { buttonVariants } from "@/shared/components/ui/button";
import { Eyebrow } from "@/shared/components/ui/eyebrow";
import { seasonSpOrZero } from "@/shared/lib/seasonTier";
import type { CurrentSeasonResponse, SeasonRank } from "@/shared/types/apiTypes";

type ProfileSeasonCardProps = {
  seasonRank: SeasonRank | null | undefined;
  progress?: CurrentSeasonResponse;
  subjectIsSelf: boolean;
};

/**
 * This season on a profile: rank and SP, the climb to the next rank on your own, then place, record and win rate.
 * Your own profile invites a first match when you have not played; someone else's simply leaves it out.
 */
export function ProfileSeasonCard({ seasonRank, progress, subjectIsSelf }: ProfileSeasonCardProps) {
  const { t } = useTranslation();
  const seasonName = seasonRank?.seasonName ?? progress?.seasonName ?? "";

  if (!seasonRank && !subjectIsSelf) return null;

  const header = (
    <div className="flex items-center justify-between gap-2" data-testid="profile-season">
      <Eyebrow>{t("profile.seasonCard.title", { season: seasonName })}</Eyebrow>
      <Link
        to="/leaderboard"
        className={buttonVariants({ variant: "link", size: "inline", className: "text-xs" })}
      >
        {t("season.leaderboard.title")}
      </Link>
    </div>
  );

  return (
    <SeasonStandingCard
      standing={
        seasonRank
          ? {
              position: seasonSpOrZero(seasonRank.position),
              sp: seasonRank.sp,
              tier: seasonRank.tier,
              division: seasonRank.division,
              gamesPlayed: seasonSpOrZero(seasonRank.gamesPlayed),
              gamesWon: seasonSpOrZero(seasonRank.gamesWon),
            }
          : null
      }
      total={seasonSpOrZero(seasonRank?.players)}
      progress={subjectIsSelf ? progress : undefined}
      past={false}
      layout="stack"
      header={header}
    />
  );
}
