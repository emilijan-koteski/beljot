import { useTranslation } from "react-i18next";

import { TierBadge } from "@/shared/components/season/TierBadge";
import { Button } from "@/shared/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/shared/components/ui/dialog";
import {
  formatSpChange,
  SEASON_TIER_COLOR,
  seasonBarFill,
  seasonRankLabel,
  seasonSpOrZero,
  type SeasonTier,
} from "@/shared/lib/seasonTier";
import { cn } from "@/shared/lib/utils";

/** The rank step the bar fills, from the server's current-season read. */
export interface RankStepProgress {
  spIntoStep: number;
  spForNextStep: number;
}

interface RankChangeDialogProps {
  open: boolean;
  direction: "promoted" | "demoted";
  /** An ALREADY-NORMALIZED tier token. */
  tier: SeasonTier;
  /** An already-normalized division: 1–3, or null for a single tier. */
  division: number | null;
  /** The match's signed, applied SP change. */
  spChange: number;
  /** The season total after the match (names the top-of-ladder bar). */
  sp: number;
  /**
   * The bar toward the next rank step, or null to leave it out (the
   * current-season read has not caught up with this match yet).
   */
  progress: RankStepProgress | null;
  onClose: () => void;
}

/**
 * RankChangeDialog — the post-match season rank-change announcement (owner
 * request 2026-09-26, replacing Story 13.5's toasts). The same forced-modal shell
 * as LevelUpDialog: no outside or Escape dismiss (`disablePointerDismissal` + the
 * no-op `onOpenChange`), no close button, closed only through Continue. Shown by
 * RankChangeGate once the player is back in the lobby or room.
 *
 * ONLY CLIMBING IS THEATRICAL (Story 13.5). A PROMOTION gets level-up's
 * celebration: the brass hairline, the gold halo and the glowing tier medallion.
 * A DEMOTION gets the same layout with none of that — a neutral hairline, no
 * halo, an unlit badge and a plain title. Never alarm styling.
 */
export function RankChangeDialog({
  open,
  direction,
  tier,
  division,
  spChange,
  sp,
  progress,
  onClose,
}: RankChangeDialogProps) {
  const { t } = useTranslation();
  const promoted = direction === "promoted";
  const rank = seasonRankLabel(t, tier, division);
  const title = promoted ? t("season.rankUpDialog.title") : t("season.rankDownDialog.title");
  const color = SEASON_TIER_COLOR[tier];

  // Step POSITION and SIZE are non-negative, so the total clamps apply (the
  // signed change above never goes through them), the same as RankBanner.
  const intoStep = progress ? seasonSpOrZero(progress.spIntoStep) : 0;
  const forNextStep = progress ? seasonSpOrZero(progress.spForNextStep) : 0;
  const atTop = progress !== null && forNextStep <= 0;
  const pct = progress ? Math.round(seasonBarFill(intoStep, forNextStep) * 100) : 0;

  return (
    <Dialog open={open} disablePointerDismissal onOpenChange={() => {}}>
      <DialogContent
        showCloseButton={false}
        data-testid="rank-change-dialog"
        data-direction={direction}
        className={cn(
          "block overflow-hidden rounded-[var(--radius-lg)] border border-border bg-surface p-0 ring-0 sm:max-w-[380px]",
          promoted
            ? "shadow-[0_0_0_1px_rgba(232,194,90,0.18)_inset,0_30px_70px_-28px_rgba(14,58,36,0.45)]"
            : "shadow-[0_30px_70px_-28px_rgba(14,58,36,0.35)]",
        )}
      >
        {promoted ? (
          <>
            {/* top brass hairline accent, as on the level-up dialog */}
            <div className="h-[3px] bg-[linear-gradient(90deg,transparent,var(--brass)_28%,var(--team-a-fill)_50%,var(--brass)_72%,transparent)]" />
            {/* faint top halo */}
            <div
              aria-hidden="true"
              data-testid="rank-change-halo"
              className="pointer-events-none absolute inset-x-0 top-0 h-[180px] bg-[radial-gradient(ellipse_70%_100%_at_50%_0%,rgba(232,194,90,0.18),transparent_70%)]"
            />
          </>
        ) : (
          <div className="bg-border h-[3px]" />
        )}

        <div className="relative flex flex-col items-center gap-4 px-[26px] pt-[26px] pb-6 text-center">
          {/* The new rank is the dialog's main fact, but its visible line is a
              plain div and the badge is aria-hidden, so the title's accessible
              name carries it ("Rank Up! Gold 2"). An aria-label, not a hidden
              span: name-from-content drops the space between the two. */}
          <DialogTitle
            aria-label={`${title} ${rank}`}
            className={cn(
              "text-[11px] font-bold tracking-[2px] uppercase",
              promoted ? "text-brass-deep" : "text-ink-dim",
            )}
          >
            {title}
          </DialogTitle>

          <TierBadge tier={tier} size="lg" glow={promoted} data-testid="rank-change-badge" />

          <div>
            <div
              data-testid="rank-change-rank"
              className={cn(
                "font-display text-[34px] leading-none font-bold tracking-[-1px]",
                !promoted && "text-ink",
              )}
              style={promoted ? { color } : undefined}
            >
              {rank}
            </div>
            <DialogDescription
              data-testid="rank-change-sp"
              className="text-ink-dim mt-1.5 text-sm tracking-[0.3px] tabular-nums"
            >
              {t("season.rankDialog.sp", { change: formatSpChange(spChange) })}
            </DialogDescription>
          </div>

          {progress ? (
            <div className="w-full">
              <div className="text-ink-dim mb-1 flex justify-end text-[12.5px] tabular-nums">
                {atTop
                  ? t("season.banner.atTop")
                  : t("season.banner.progress", {
                      current: intoStep.toLocaleString(),
                      next: forNextStep.toLocaleString(),
                    })}
              </div>
              <div
                data-testid="rank-change-progress"
                role="progressbar"
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={pct}
                aria-label={
                  atTop
                    ? t("season.banner.progressLabelTop", { tier: rank, sp: sp.toLocaleString() })
                    : t("season.banner.progressLabel", {
                        tier: rank,
                        current: intoStep.toLocaleString(),
                        next: forNextStep.toLocaleString(),
                      })
                }
                className="bg-surface-sunken h-1.5 overflow-hidden rounded-full"
              >
                <div
                  className="h-full rounded-full transition-[width] duration-500 ease-out"
                  style={{ width: `${pct}%`, background: color }}
                />
              </div>
            </div>
          ) : (
            // The row's height, RESERVED while the season read catches up: the
            // popup is centred, so a bar that popped in later would shift the
            // whole card. An empty track in the same box, announced as nothing.
            <div
              aria-hidden="true"
              data-testid="rank-change-progress-placeholder"
              className="w-full"
            >
              <div className="mb-1 text-[12.5px]">{"\u00a0"}</div>
              <div className="bg-surface-sunken h-1.5 rounded-full" />
            </div>
          )}

          <Button
            onClick={onClose}
            size="cta"
            className="w-full"
            data-testid="rank-change-continue"
          >
            {t("season.rankDialog.continue")}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
