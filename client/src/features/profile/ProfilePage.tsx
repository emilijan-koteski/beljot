import { useTranslation } from "react-i18next";

import type { MatchFilter } from "@/shared/api/matches";
import { useCareerQuery } from "@/shared/hooks/queries/useCareer";
import { useProfileQuery } from "@/shared/hooks/queries/useProfile";
import { xpBarFill } from "@/shared/lib/xpLevel";
import { useAuthStore } from "@/shared/stores/authStore";

import { IdentityHero } from "./components/IdentityHero";
import { LinkedAccounts } from "./components/LinkedAccounts";
import { Milestones } from "./components/Milestones";
import { PartnerSpotlight } from "./components/PartnerSpotlight";
import { Rivalries } from "./components/Rivalries";
import { StatsGrid } from "./components/StatsGrid";
import { StreakCallout } from "./components/StreakCallout";
import { MatchHistory } from "./MatchHistory";

export function ProfilePage() {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const { data: profile, isPending, isError } = useProfileQuery(user?.id);
  const career = useCareerQuery(user?.id);

  if (isPending) {
    return (
      <div className="mx-auto max-w-330 px-4 py-8 md:px-7" data-testid="profile-loading">
        <div className="bg-surface h-40 animate-pulse rounded-lg" />
        <div className="mt-5 grid grid-cols-2 gap-3 md:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="bg-surface h-28 animate-pulse rounded-lg" />
          ))}
        </div>
      </div>
    );
  }

  const username = profile?.username ?? user?.username ?? "";
  const createdAt = profile?.createdAt ?? user?.createdAt ?? "";
  const wins = profile?.wins ?? 0;
  const losses = profile?.losses ?? 0;
  const abandoned = profile?.abandoned ?? 0;
  const games = profile?.totalGamesPlayed ?? 0;
  const winRate = games === 0 ? null : Math.round((wins / games) * 100);

  // When the profile query hasn't resolved (e.g. an error after the pending
  // skeleton), fall back to the client XP curve anchored on the auth-store
  // level so the bar stays consistent with the level shown — never "0 / 0 XP".
  const xpFallback = xpBarFill(user?.totalXp ?? 0, user?.level ?? 0);

  const counts: Record<MatchFilter, number> = {
    all: games,
    win: wins,
    loss: losses,
    abandoned,
  };

  return (
    <div className="mx-auto max-w-330 px-4 py-8 pb-32 md:px-7" data-testid="profile-page">
      <IdentityHero
        username={username}
        userId={user?.id}
        usernameChangedAt={profile?.usernameChangedAt}
        createdAt={createdAt}
        lastPlayedAt={career.data?.lastPlayedAt}
        games={games}
        wins={wins}
        losses={losses}
        capots={career.data?.capots ?? 0}
        walletBalance={user?.walletBalance ?? 0}
        loginStreakDays={user?.loginStreakDays ?? 0}
        level={profile?.level ?? user?.level ?? 0}
        xpIntoLevel={profile?.xpIntoLevel ?? xpFallback.xpIntoLevel}
        xpForNextLevel={profile?.xpForNextLevel ?? xpFallback.xpForNextLevel}
        winRate={winRate}
      />

      {career.data && <StreakCallout streak={career.data.streak} />}

      {isError ? (
        <section
          className="bg-surface border-border mb-5 rounded-lg border p-6"
          data-testid="profile-stats"
        >
          <p className="text-destructive text-sm" data-testid="profile-stats-error">
            {t("profile.stats.error")}
          </p>
        </section>
      ) : (
        <StatsGrid games={games} wins={wins} losses={losses} abandoned={abandoned} />
      )}

      <div className="grid grid-cols-1 gap-5 lg:grid-cols-[minmax(0,1fr)_320px] lg:items-start">
        <MatchHistory userId={user?.id} counts={counts} />

        <aside className="flex flex-col gap-3.5 lg:sticky lg:top-20" data-testid="profile-sidebar">
          <LinkedAccounts userId={user?.id} />

          {career.isError ? (
            <p className="text-ink-mute text-sm" data-testid="profile-career-error">
              {t("profile.careerError")}
            </p>
          ) : career.data ? (
            <>
              <PartnerSpotlight partners={career.data.topPartners} />
              <Rivalries rivals={career.data.topRivals} />
              <Milestones
                capots={career.data.capots}
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
