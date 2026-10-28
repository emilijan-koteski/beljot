import { useTranslation } from "react-i18next";
import { Link, useParams } from "react-router";

import { FetchError } from "@/shared/api/axiosClient";
import type { MatchFilter } from "@/shared/api/matches";
import { EmptyState } from "@/shared/components/EmptyState";
import { FriendButton } from "@/shared/components/friends/FriendButton";
import { buttonVariants } from "@/shared/components/ui/button";
import { useCareerQuery } from "@/shared/hooks/queries/useCareer";
import { usePublicProfileQuery } from "@/shared/hooks/queries/usePublicProfile";

import { IdentityHero } from "./components/IdentityHero";
import { Milestones } from "./components/Milestones";
import { PartnerSpotlight } from "./components/PartnerSpotlight";
import { Rivalries } from "./components/Rivalries";
import { SeasonSection } from "./components/SeasonSection";
import { StatsGrid } from "./components/StatsGrid";
import { StreakCallout } from "./components/StreakCallout";
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
 * Read-only profile of another player at /players/:id, kept separate from ProfilePage so
 * self-only side effects are never mounted. The page is keyed by subject id so navigating
 * between cached profiles remounts its local state.
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
        <div className="bg-surface h-40 animate-pulse rounded-lg" />
        <div className="mt-5 grid grid-cols-2 gap-3 md:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="bg-surface h-28 animate-pulse rounded-lg" />
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
  const winRate = games === 0 ? null : Math.round((wins / games) * 100);

  const counts: Record<MatchFilter, number> = {
    all: games,
    win: wins,
    loss: losses,
    abandoned,
  };

  return (
    <div
      key={validId}
      className="mx-auto max-w-330 px-4 py-8 pb-32 md:px-7"
      data-testid="public-profile-page"
    >
      <IdentityHero
        username={profile.username}
        avatarUrl={profile.avatarLargeUrl ?? null}
        userId={undefined}
        createdAt={profile.createdAt}
        lastPlayedAt={career.data?.lastPlayedAt}
        games={games}
        wins={wins}
        losses={losses}
        capots={career.data?.capots ?? 0}
        hidePrivatePills
        walletBalance={0}
        loginStreakDays={0}
        level={profile.level}
        xpIntoLevel={profile.xpIntoLevel}
        xpForNextLevel={profile.xpForNextLevel}
        winRate={winRate}
        honor={{
          subjectIsSelf: false,
          score: profile.honorScore,
          tier: profile.honorTier,
          completedTotal: profile.honorCompletedTotal,
          abandonedTotal: profile.honorAbandonedTotal,
          isNewPlayer: profile.isNewPlayer,
          trendDelta: profile.honorTrendDelta,
          trendDirection: profile.honorTrendDirection,
        }}
      />

      {validId !== undefined && <FriendButton userId={validId} username={profile.username} />}

      {career.data && <StreakCallout streak={career.data.streak} subjectIsSelf={false} />}

      <StatsGrid games={games} wins={wins} losses={losses} abandoned={abandoned} />

      <SeasonSection userId={validId} seasonRank={profile.seasonRank} />

      <div className="grid grid-cols-1 gap-5 lg:grid-cols-[minmax(0,1fr)_320px] lg:items-start">
        <MatchHistory userId={validId} counts={counts} subjectIsSelf={false} />

        <aside
          className="flex flex-col gap-3.5 lg:sticky lg:top-20"
          data-testid="public-profile-sidebar"
        >
          {career.isError ? (
            <p className="text-danger text-sm" data-testid="public-profile-career-error">
              {t("profile.careerError")}
            </p>
          ) : career.data ? (
            <>
              <PartnerSpotlight partners={career.data.topPartners} subjectIsSelf={false} />
              <Rivalries rivals={career.data.topRivals} subjectIsSelf={false} />
              <Milestones
                capots={career.data.capots}
                careerPoints={career.data.careerPoints}
                bestHand={career.data.bestHand}
                avgMatchSeconds={career.data.avgMatchSeconds}
              />
            </>
          ) : (
            Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className="bg-surface h-36 animate-pulse rounded-lg" />
            ))
          )}
        </aside>
      </div>
    </div>
  );
}
