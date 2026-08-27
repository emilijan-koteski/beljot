import { useEffect } from "react";
import { useTranslation } from "react-i18next";

import type { MatchFilter } from "@/shared/api/matches";
import { useCareerQuery } from "@/shared/hooks/queries/useCareer";
import { useCurrentSeasonQuery } from "@/shared/hooks/queries/useCurrentSeason";
import { useProfileQuery } from "@/shared/hooks/queries/useProfile";
import { xpBarFill } from "@/shared/lib/xpLevel";
import { useAuthStore } from "@/shared/stores/authStore";

import { CardDeckPanel } from "./components/CardDeckPanel";
import { IdentityHero } from "./components/IdentityHero";
import { LinkedAccounts } from "./components/LinkedAccounts";
import { Milestones } from "./components/Milestones";
import { PartnerSpotlight } from "./components/PartnerSpotlight";
import { RankBanner } from "./components/RankBanner";
import { Rivalries } from "./components/Rivalries";
import { SeasonSection } from "./components/SeasonSection";
import { StatsGrid } from "./components/StatsGrid";
import { StreakCallout } from "./components/StreakCallout";
import { MatchHistory } from "./MatchHistory";

export function ProfilePage() {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const { data: profile, isPending, isError } = useProfileQuery(user?.id);
  const career = useCareerQuery(user?.id);
  // The viewer's seasonal standing, for the RankBanner below the hero. The same
  // cache entry the header's rank chip reads, so this costs no extra request —
  // and it is push-invalidated by the event:season_points_awarded handler, so a
  // match settled in another tab moves both readouts together.
  const seasonQuery = useCurrentSeasonQuery();

  // Story 9.7: adopt the profile response's honor into the auth store.
  //
  // event:honor_updated is the live path, but it is delivered over the player's
  // own socket — and the abandoning seat is BY DEFINITION the one with no live
  // socket when handleSeatReconnectTimeout fires, so the player whose honor just
  // got worse is the only one never told. Hub.SendToUser is a silent no-op with
  // no queue, so that event is gone for good. Without this effect the header chip
  // kept rendering the pre-abandonment score until the next token refresh or a
  // reload, which let /profile show a real 44 in the hero's honour band beside a
  // stale 90 in the chip directly above it — two different numbers for the same
  // quantity on one screen. (The band was a standalone HonorPanel section when
  // this was written; the contradiction it closes is unchanged.)
  //
  // GET /users/:id/profile recomputes honor from the stored weights on every
  // read, so it is authoritative by the same rule the WS event is; adopting it
  // here costs no extra request and closes the contradiction at the one place
  // both surfaces are visible together.
  //
  // Same explicit-type guards as the WS handler, for the same reason: Go zero
  // values are real values. A score of 0 ("Problematic") and isNewPlayer false
  // are both legitimate and both falsy, so never test truthiness. The equality
  // check keeps this to a single write - setUser with identical values would
  // re-render forever.
  const honorScore = profile?.honorScore;
  const honorTier = profile?.honorTier;
  const honorIsNew = profile?.isNewPlayer;
  useEffect(() => {
    // `typeof` first because Number.isInteger is not a type guard - TS cannot
    // narrow `number | undefined` from it on its own.
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
        // Honour rides the hero as its bottom band rather than a section of its
        // own (redesign R3). Still gated on `profile`: with no profile there is
        // simply no honour band, so the loading transient can never mix a real
        // value with a 0 fallback — the bug the Story 9.5 review caught on the
        // XP bar.
        honor={
          profile
            ? {
                score: profile.honorScore,
                tier: profile.honorTier,
                completedTotal: profile.honorCompletedTotal,
                abandonedTotal: profile.honorAbandonedTotal,
                isNewPlayer: profile.isNewPlayer,
                trendDelta: profile.honorTrendDelta,
                trendDirection: profile.honorTrendDirection,
              }
            : undefined
        }
      />

      {/* The seasonal rank, with the progress the header chip deliberately
          omits: SP total, distance to the next tier, days left in the window.
          ABOVE the streak callout — the season standing is the longer-running
          fact, and the streak reads as a note on current form beneath it. */}
      <RankBanner season={seasonQuery.data} />

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

      {/* Story 13.3: the prior-season archive. Its current-rank chip is
          suppressed here — the RankBanner above the streak callout is this
          page's rank surface, and two readouts of one standing on one page is
          exactly the contradiction the chip was added to avoid. Renders nothing
          at all for a viewer with no ended seasons, so the page stays
          byte-identical for a player with no season history. */}
      <SeasonSection userId={user?.id} seasonRank={profile?.seasonRank} showCurrentRank={false} />

      <div className="grid grid-cols-1 gap-5 lg:grid-cols-[minmax(0,1fr)_320px] lg:items-start">
        <MatchHistory userId={user?.id} counts={counts} />

        <aside className="flex flex-col gap-3.5 lg:sticky lg:top-20" data-testid="profile-sidebar">
          <LinkedAccounts userId={user?.id} />

          {/* Outside the `career` gate below: the deck is a preference read
              from the auth store, not career data, so it must still render for
              a brand-new account whose career query has nothing to show. */}
          <CardDeckPanel />

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
