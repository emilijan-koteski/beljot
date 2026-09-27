import { useState } from "react";

import { useCurrentSeasonQuery } from "@/shared/hooks/queries/useCurrentSeason";
import { normalizeSeasonDivision, normalizeSeasonTier } from "@/shared/lib/seasonTier";
import { useLevelUpStore } from "@/shared/stores/levelUpStore";
import { type RankChangeInfo, useRankChangeStore } from "@/shared/stores/rankChangeStore";

import { RankChangeDialog, type RankStepProgress } from "./RankChangeDialog";

/**
 * RankChangeGate renders the post-match rank-change dialog whenever a promotion
 * or demotion is pending on rankChangeStore. Mounted once in AppLayout beside
 * LevelUpGate — AppLayout does not wrap the match route, so, like the level-up
 * celebration, it only ever appears once the player is back in the lobby/room.
 *
 * LEVEL-UP GOES FIRST. Both dialogs are forced modals, and one match can bring
 * both; stacking them would put two Continue buttons on screen at once. So this
 * waits while a level-up is pending and opens the moment that one is dismissed.
 *
 * The bar toward the next rank step comes from the current-season read, which
 * event:season_points_awarded invalidated. It is shown only once that read has
 * caught up with THIS match (same SP total and rank as the pending change), so
 * a slow refetch never paints last match's bar under the new rank.
 */
export function RankChangeGate() {
  const pending = useRankChangeStore((s) => s.pending);
  const clear = useRankChangeStore((s) => s.clear);
  const levelUpPending = useLevelUpStore((s) => s.pending !== null);
  const { data: season } = useCurrentSeasonQuery(pending !== null);

  // THE LAST CHANGE SHOWN outlives the store while the dialog animates closed.
  // Continue clears the store at once, but the popup stays mounted for its exit
  // animation; rendering the null fallbacks there would repaint a demotion as a
  // celebratory "Rank Up! · Iron" as it fades. Derived state kept across renders
  // (React's "storing information from previous renders" pattern).
  const [shown, setShown] = useState<RankChangeInfo | null>(pending);
  if (pending !== null && pending !== shown) {
    setShown(pending);
  }
  const info = pending ?? shown;

  const tier = normalizeSeasonTier(info?.rankTier ?? "", info?.newSeasonSp ?? 0);
  const division = normalizeSeasonDivision(tier, info?.rankDivision);

  let progress: RankStepProgress | null = null;
  if (
    info &&
    season &&
    season.sp === info.newSeasonSp &&
    normalizeSeasonTier(season.rankTier, season.sp) === tier &&
    normalizeSeasonDivision(tier, season.rankDivision) === division &&
    Number.isFinite(season.spIntoDivision) &&
    Number.isFinite(season.spForNextDivision)
  ) {
    progress = { spIntoStep: season.spIntoDivision, spForNextStep: season.spForNextDivision };
  }

  return (
    <RankChangeDialog
      open={pending !== null && !levelUpPending}
      direction={info?.direction ?? "promoted"}
      tier={tier}
      division={division}
      spChange={info?.spChange ?? 0}
      sp={info?.newSeasonSp ?? 0}
      progress={progress}
      onClose={clear}
    />
  );
}
