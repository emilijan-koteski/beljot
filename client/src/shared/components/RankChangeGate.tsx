import { useState } from "react";

import { useCurrentSeasonQuery } from "@/shared/hooks/queries/useCurrentSeason";
import { normalizeSeasonDivision, normalizeSeasonTier } from "@/shared/lib/seasonTier";
import { useLevelUpStore } from "@/shared/stores/levelUpStore";
import { type RankChangeInfo, useRankChangeStore } from "@/shared/stores/rankChangeStore";

import { RankChangeDialog, type RankStepProgress } from "./RankChangeDialog";

/**
 * Renders the rank-change dialog for a pending promotion or demotion, waiting until any pending level-up is dismissed.
 * Keeps the last change rendered while the dialog animates closed so the exit never repaints fallback values.
 */
export function RankChangeGate() {
  const pending = useRankChangeStore((s) => s.pending);
  const clear = useRankChangeStore((s) => s.clear);
  const levelUpPending = useLevelUpStore((s) => s.pending !== null);
  const { data: season } = useCurrentSeasonQuery(pending !== null);

  const [shown, setShown] = useState<RankChangeInfo | null>(pending);
  if (pending !== null && pending !== shown) {
    setShown(pending);
  }
  const info = pending ?? shown;

  const tier = normalizeSeasonTier(info?.rankTier ?? "", info?.newSeasonSp ?? 0);
  const division = normalizeSeasonDivision(tier, info?.rankDivision);

  const seasonCaughtUpWithMatch =
    info &&
    season &&
    season.sp === info.newSeasonSp &&
    normalizeSeasonTier(season.rankTier, season.sp) === tier &&
    normalizeSeasonDivision(tier, season.rankDivision) === division;

  let progress: RankStepProgress | null = null;
  if (
    seasonCaughtUpWithMatch &&
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
