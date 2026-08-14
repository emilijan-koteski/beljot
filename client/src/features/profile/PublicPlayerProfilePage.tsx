import { useTranslation } from "react-i18next";
import { Link, useParams } from "react-router";

import { FetchError } from "@/shared/api/axiosClient";
import type { MatchFilter } from "@/shared/api/matches";
import { useCareerQuery } from "@/shared/hooks/queries/useCareer";
import { usePublicProfileQuery } from "@/shared/hooks/queries/usePublicProfile";

import { IdentityHero } from "./components/IdentityHero";
import { Milestones } from "./components/Milestones";
import { PartnerSpotlight } from "./components/PartnerSpotlight";
import { Rivalries } from "./components/Rivalries";
import { StatsGrid } from "./components/StatsGrid";
import { StreakCallout } from "./components/StreakCallout";
import { MatchHistory } from "./MatchHistory";

/**
 * Public read-only view of ANOTHER player's profile (Story 11.3, FR47), reached
 * at /players/:id by any authenticated viewer.
 *
 * It is a SEPARATE page from the self ProfilePage rather than a dual-mode of it
 * (Design Decision D5): that structurally excludes the self-only side effects
 * instead of gating them. Three things the self page does are simply NOT mounted
 * here — the honor auth-store hydration effect (it would overwrite the VIEWER's
 * TopBar honor with the subject's), <LinkedAccounts> (SSO management), and the
 * username edit pencil (userId is never passed) — plus the private wallet/streak
 * pills are suppressed via hidePrivatePills. No viewer data is ever read for the
 * subject's surfaces.
 */
export function PublicPlayerProfilePage() {
  const { t } = useTranslation();
  const { id } = useParams();

  // Guard the route param against a strict canonical positive-integer form
  // BEFORE coercing. `Number()` alone would accept non-canonical spellings
  // ("1e2" → 100, "0x10" → 16, " 3 " → 3, "+5", "5.0") and alias them to a
  // real-but-different subject, and would turn an over-range id into a server
  // 400 that misses the not-found surface. A `/^[1-9][0-9]*$/` match plus a
  // safe-integer bound rejects all of those to the same not-found state as an
  // unknown id (Story 11.3 review).
  const validId =
    id !== undefined && /^[1-9][0-9]*$/.test(id) && Number.isSafeInteger(Number(id))
      ? Number(id)
      : undefined;

  const { data: profile, isPending, isError, error } = usePublicProfileQuery(validId);
  const career = useCareerQuery(validId);

  // Not-found: an invalid id, or a 404 USER_NOT_FOUND from the server (unknown /
  // soft-deleted subject). Distinct from a transient error so the copy can say
  // "no such player" rather than "try again".
  const isNotFound =
    validId === undefined || (isError && error instanceof FetchError && error.status === 404);

  if (isNotFound) {
    return (
      <div className="mx-auto max-w-330 px-4 py-8 md:px-7">
        <div
          className="bg-surface border-border space-y-3 rounded-lg border border-dashed p-10 text-center text-sm"
          data-testid="public-profile-not-found"
        >
          <p className="text-ink font-display text-lg font-semibold">
            {t("publicProfile.notFound.title")}
          </p>
          <p className="text-ink-dim m-0">{t("publicProfile.notFound.body")}</p>
          <Link
            to="/lobby"
            className="text-accent inline-flex items-center underline-offset-2 hover:underline"
            data-testid="public-profile-not-found-cta"
          >
            {t("publicProfile.notFound.cta")}
          </Link>
        </div>
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

  // Any non-404 failure (500, network) after the pending skeleton: there is no
  // subject data to show, so render a generic error rather than a half profile.
  if (isError || !profile) {
    return (
      <div className="mx-auto max-w-330 px-4 py-8 md:px-7">
        <div
          className="bg-surface border-border rounded-lg border p-10 text-center text-sm"
          data-testid="public-profile-error"
        >
          <p className="text-destructive m-0">{t("publicProfile.error")}</p>
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
    <div className="mx-auto max-w-330 px-4 py-8 pb-32 md:px-7" data-testid="public-profile-page">
      <IdentityHero
        username={profile.username}
        // No userId → the username edit pencil stays hidden (read-only switch).
        userId={undefined}
        createdAt={profile.createdAt}
        lastPlayedAt={career.data?.lastPlayedAt}
        games={games}
        wins={wins}
        losses={losses}
        capots={career.data?.capots ?? 0}
        // The wallet + login-streak pills are private and are suppressed; the 0s
        // below are never rendered. Crucially we never read the viewer's authStore
        // for the subject's pills.
        hidePrivatePills
        walletBalance={0}
        loginStreakDays={0}
        level={profile.level}
        xpIntoLevel={profile.xpIntoLevel}
        xpForNextLevel={profile.xpForNextLevel}
        winRate={winRate}
        // Honour rides the hero as its bottom band (same as the self page). The
        // client decides New Player suppression; the server-recomputed score/tier
        // are authoritative for any viewer.
        honor={{
          score: profile.honorScore,
          tier: profile.honorTier,
          completedTotal: profile.honorCompletedTotal,
          abandonedTotal: profile.honorAbandonedTotal,
          isNewPlayer: profile.isNewPlayer,
          trendDelta: profile.honorTrendDelta,
          trendDirection: profile.honorTrendDirection,
        }}
      />

      {/* Story 11.2 insertion point: the "Add Friend" action mounts here once the
          friend model + endpoint exist. Deliberately NOT built in 11.3 — there is
          no friend backend on master, so a button here would be dead/placeholder
          (Design Decision D4). AC4 layout parity holds without it: Add Friend is
          an additive element, not part of the read layout. */}

      {career.data && <StreakCallout streak={career.data.streak} />}

      <StatsGrid games={games} wins={wins} losses={losses} abandoned={abandoned} />

      {/* No seasonal-rank / prior-season section: Epic 13 is unbuilt and there is
          no season data anywhere, so AC5 is satisfied by rendering nothing here. */}

      <div className="grid grid-cols-1 gap-5 lg:grid-cols-[minmax(0,1fr)_320px] lg:items-start">
        {/* subjectIsSelf=false → the subject's seat shows their username, never a
            "YOU" badge (the subject is not the viewer). */}
        <MatchHistory userId={validId} counts={counts} subjectIsSelf={false} />

        <aside
          className="flex flex-col gap-3.5 lg:sticky lg:top-20"
          data-testid="public-profile-sidebar"
        >
          {career.isError ? (
            <p className="text-ink-mute text-sm" data-testid="public-profile-career-error">
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
