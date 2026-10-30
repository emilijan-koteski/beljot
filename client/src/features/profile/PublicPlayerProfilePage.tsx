import { useTranslation } from "react-i18next";
import { Link, useParams } from "react-router";

import { FetchError } from "@/shared/api/axiosClient";
import type { MatchFilter } from "@/shared/api/matches";
import { EmptyState } from "@/shared/components/EmptyState";
import { FriendButton } from "@/shared/components/friends/FriendButton";
import { buttonVariants } from "@/shared/components/ui/button";
import { Skeleton } from "@/shared/components/ui/skeleton";
import { useCareerQuery } from "@/shared/hooks/queries/useCareer";
import { usePublicProfileQuery } from "@/shared/hooks/queries/usePublicProfile";

import { CareerCard } from "./components/CareerCard";
import { careerSlots } from "./components/careerSlots";
import { HonorCard } from "./components/HonorCard";
import { PastSeasons } from "./components/PastSeasons";
import { ProfileHero } from "./components/ProfileHero";
import { ProfileLayout } from "./components/ProfileLayout";
import { ProfileSeasonCard } from "./components/ProfileSeasonCard";
import { MatchHistory } from "./MatchHistory";

/**
 * Parses a canonical positive safe-integer id, rejecting spellings like "1e2" or "0x10"
 * that `Number()` alone would alias to a different player.
 */
function parseCanonicalUserId(id: string | undefined): number | undefined {
  return id !== undefined && /^[1-9][0-9]*$/.test(id) && Number.isSafeInteger(Number(id))
    ? Number(id)
    : undefined;
}

/**
 * Another player's profile at /players/:id, with the friend action and nothing private, kept apart from ProfilePage
 * so self-only side effects never mount. It is keyed by subject id, so moving between cached profiles resets its tabs.
 */
export function PublicPlayerProfilePage() {
  const { t } = useTranslation();
  const { id } = useParams();

  const validId = parseCanonicalUserId(id);

  const { data: profile, isPending, isError, error } = usePublicProfileQuery(validId);
  const career = useCareerQuery(validId);

  const isNotFound =
    validId === undefined || (isError && error instanceof FetchError && error.status === 404);

  if (isNotFound) {
    return (
      <div className="mx-auto max-w-330 px-4 py-8 md:px-7">
        <EmptyState
          testId="public-profile-not-found"
          title={t("publicProfile.notFound.title")}
          action={
            <Link
              to="/lobby"
              className={buttonVariants({ variant: "link", size: "inline" })}
              data-testid="public-profile-not-found-cta"
            >
              {t("publicProfile.notFound.cta")}
            </Link>
          }
        >
          {t("publicProfile.notFound.body")}
        </EmptyState>
      </div>
    );
  }

  if (isPending) {
    return (
      <div className="mx-auto max-w-330 px-4 py-8 md:px-7" data-testid="public-profile-loading">
        <Skeleton className="h-40 rounded-lg" />
        <div className="mt-5 grid grid-cols-2 gap-3 md:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-28 rounded-lg" />
          ))}
        </div>
      </div>
    );
  }

  if (isError || !profile) {
    return (
      <div className="mx-auto max-w-330 px-4 py-8 md:px-7">
        <div
          className="bg-surface border-border rounded-lg border p-10 text-center text-sm"
          data-testid="public-profile-error"
        >
          <p className="text-danger m-0">{t("publicProfile.error")}</p>
        </div>
      </div>
    );
  }

  const games = profile.totalGamesPlayed;
  const wins = profile.wins;
  const losses = profile.losses;
  const abandoned = profile.abandoned;
  const counts: Record<MatchFilter, number> = { all: games, win: wins, loss: losses, abandoned };
  const slots = careerSlots(career, false, {
    text: t("profile.careerError"),
    testId: "public-profile-career-error",
  });

  return (
    <ProfileLayout
      key={validId}
      testId="public-profile-page"
      hero={(layout) => (
        <ProfileHero
          layout={layout}
          username={profile.username}
          avatarUrl={profile.avatarLargeUrl ?? null}
          createdAt={profile.createdAt}
          lastPlayedAt={career.data?.lastPlayedAt}
          level={profile.level}
          xpIntoLevel={profile.xpIntoLevel}
          xpForNextLevel={profile.xpForNextLevel}
          seasonRank={profile.seasonRank}
          honor={{
            score: profile.honorScore,
            tier: profile.honorTier,
            isNewPlayer: profile.isNewPlayer,
          }}
          action={
            validId !== undefined && (
              <div className="flex flex-wrap justify-center gap-2">
                <FriendButton userId={validId} username={profile.username} compact />
              </div>
            )
          }
        />
      )}
      season={<ProfileSeasonCard seasonRank={profile.seasonRank} subjectIsSelf={false} />}
      career={
        <CareerCard
          games={games}
          wins={wins}
          losses={losses}
          abandoned={abandoned}
          streak={career.data?.streak}
          subjectIsSelf={false}
        />
      }
      honor={
        <HonorCard
          subjectIsSelf={false}
          score={profile.honorScore}
          tier={profile.honorTier}
          completedTotal={profile.honorCompletedTotal}
          abandonedTotal={profile.honorAbandonedTotal}
          isNewPlayer={profile.isNewPlayer}
          trendDelta={profile.honorTrendDelta}
          trendDirection={profile.honorTrendDirection}
        />
      }
      highlights={slots.highlights}
      partners={slots.partners}
      rivals={slots.rivals}
      matches={<MatchHistory userId={validId} counts={counts} subjectIsSelf={false} />}
      seasons={(compact) => <PastSeasons userId={validId} compact={compact} />}
    />
  );
}
