import { Settings } from "lucide-react";
import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import type { MatchFilter } from "@/shared/api/matches";
import { buttonVariants } from "@/shared/components/ui/button";
import { Skeleton } from "@/shared/components/ui/skeleton";
import { useCareerQuery } from "@/shared/hooks/queries/useCareer";
import { useCurrentSeasonQuery } from "@/shared/hooks/queries/useCurrentSeason";
import { useProfileQuery } from "@/shared/hooks/queries/useProfile";
import { xpBarFill } from "@/shared/lib/xpLevel";
import { useAuthStore } from "@/shared/stores/authStore";

import { CareerCard } from "./components/CareerCard";
import { careerSlots } from "./components/careerSlots";
import { HonorCard } from "./components/HonorCard";
import { PastSeasons } from "./components/PastSeasons";
import { ProfileHero } from "./components/ProfileHero";
import { ProfileLayout } from "./components/ProfileLayout";
import { ProfileSeasonCard } from "./components/ProfileSeasonCard";
import { MatchHistory } from "./MatchHistory";

/**
 * Copies the profile's authoritative honor into the auth store, since an abandoning seat
 * never receives event:honor_updated. Writes only on change, with explicit type checks
 * because a score of 0 and isNewPlayer false are real values.
 */
function useAdoptProfileHonor(
  honorScore: number | undefined,
  honorTier: string | undefined,
  honorIsNew: boolean | undefined,
) {
  useEffect(() => {
    if (
      typeof honorScore !== "number" ||
      !Number.isInteger(honorScore) ||
      typeof honorTier !== "string" ||
      honorTier === "" ||
      typeof honorIsNew !== "boolean"
    ) {
      return;
    }
    const { user: current, setUser } = useAuthStore.getState();
    if (!current) return;
    if (
      current.honorScore === honorScore &&
      current.honorTier === honorTier &&
      current.isNewPlayer === honorIsNew
    ) {
      return;
    }
    setUser({ ...current, honorScore, honorTier, isNewPlayer: honorIsNew });
  }, [honorScore, honorTier, honorIsNew]);
}

/**
 * Your own profile: who you are, this season, your career, honor and people, matches and past seasons.
 * Everything you can change about your account lives on the Settings page, linked from the hero.
 */
export function ProfilePage() {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const { data: profile, isPending, isError } = useProfileQuery(user?.id);
  const career = useCareerQuery(user?.id);
  const seasonQuery = useCurrentSeasonQuery();

  useAdoptProfileHonor(profile?.honorScore, profile?.honorTier, profile?.isNewPlayer);

  if (isPending) {
    return (
      <div className="mx-auto max-w-330 px-4 py-8 md:px-7" data-testid="profile-loading">
        <Skeleton className="h-40 rounded-lg" />
        <div className="mt-5 grid grid-cols-2 gap-3 md:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-28 rounded-lg" />
          ))}
        </div>
      </div>
    );
  }

  const wins = profile?.wins ?? 0;
  const losses = profile?.losses ?? 0;
  const abandoned = profile?.abandoned ?? 0;
  const games = profile?.totalGamesPlayed ?? 0;
  const xpFallback = xpBarFill(user?.totalXp ?? 0, user?.level ?? 0);
  const counts: Record<MatchFilter, number> = { all: games, win: wins, loss: losses, abandoned };
  const slots = careerSlots(career, true, {
    text: t("profile.careerError"),
    testId: "profile-career-error",
  });

  return (
    <ProfileLayout
      testId="profile-page"
      hero={(layout) => (
        <ProfileHero
          layout={layout}
          username={profile?.username ?? user?.username ?? ""}
          avatarUrl={profile ? (profile.avatarLargeUrl ?? null) : (user?.avatarUrl ?? null)}
          createdAt={profile?.createdAt ?? user?.createdAt ?? ""}
          lastPlayedAt={career.data?.lastPlayedAt}
          level={profile?.level ?? user?.level ?? 0}
          xpIntoLevel={profile?.xpIntoLevel ?? xpFallback.xpIntoLevel}
          xpForNextLevel={profile?.xpForNextLevel ?? xpFallback.xpForNextLevel}
          seasonRank={profile?.seasonRank}
          honor={
            profile
              ? {
                  score: profile.honorScore,
                  tier: profile.honorTier,
                  isNewPlayer: profile.isNewPlayer,
                }
              : undefined
          }
          walletBalance={user?.walletBalance ?? 0}
          loginStreakDays={user?.loginStreakDays ?? 0}
          action={
            <Link
              to="/settings"
              className={buttonVariants({ variant: "outline", size: "sm", className: "gap-1.5" })}
              data-testid="profile-settings-link"
            >
              <Settings className="size-4" />
              {t("nav.settings")}
            </Link>
          }
        />
      )}
      season={
        <ProfileSeasonCard
          seasonRank={profile?.seasonRank}
          progress={seasonQuery.data}
          subjectIsSelf
        />
      }
      career={
        <CareerCard
          games={games}
          wins={wins}
          losses={losses}
          abandoned={abandoned}
          streak={career.data?.streak}
          statsError={isError}
        />
      }
      honor={
        profile && (
          <HonorCard
            score={profile.honorScore}
            tier={profile.honorTier}
            completedTotal={profile.honorCompletedTotal}
            abandonedTotal={profile.honorAbandonedTotal}
            isNewPlayer={profile.isNewPlayer}
            trendDelta={profile.honorTrendDelta}
            trendDirection={profile.honorTrendDirection}
          />
        )
      }
      highlights={slots.highlights}
      partners={slots.partners}
      rivals={slots.rivals}
      matches={<MatchHistory userId={user?.id} counts={counts} />}
      seasons={(compact) => <PastSeasons userId={user?.id} compact={compact} />}
    />
  );
}
