import { useTranslation } from "react-i18next";

import { Button } from "@/shared/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/shared/components/ui/dialog";
import { XpBar } from "@/shared/components/XpBar";
import { cn } from "@/shared/lib/utils";
import { xpBarFill } from "@/shared/lib/xpLevel";

interface LevelUpDialogProps {
  open: boolean;
  /** The level the player just reached (server-authoritative). */
  level: number;
  /** Lifetime XP after the match — anchors the progress-bar band. */
  newTotalXp: number;
  /** XP earned in the match that triggered the level-up. */
  xpEarned: number;
  onClose: () => void;
}

/**
 * Gold level-badge medallion — the level number struck into a gold coin face,
 * reusing the daily-reward coin construction (radial-gradient face + layered
 * inset shadows) but glyphed with the level instead of the Beljot monogram.
 */
function LevelMedallion({ level, size = 92 }: { level: number; size?: number }) {
  return (
    <span
      aria-hidden="true"
      style={{
        position: "relative",
        width: size,
        height: size,
        display: "inline-flex",
        flexShrink: 0,
      }}
    >
      {/* soft outer glow */}
      <span
        style={{
          position: "absolute",
          inset: -size * 0.28,
          borderRadius: "50%",
          background: "radial-gradient(circle, rgba(232,194,90,0.45) 0%, transparent 68%)",
        }}
      />
      {/* coin body */}
      <span
        style={{
          position: "relative",
          width: size,
          height: size,
          borderRadius: "50%",
          background: "radial-gradient(circle at 38% 30%, #fbeec0 0%, #e8c25a 50%, #a07d1a 100%)",
          border: `${Math.max(2, size * 0.025)}px solid #a07d1a`,
          boxShadow: `inset 0 ${size * 0.03}px ${size * 0.05}px rgba(255,255,255,0.6), inset 0 -${size * 0.04}px ${size * 0.07}px rgba(0,0,0,0.28), 0 ${size * 0.08}px ${size * 0.18}px -${size * 0.06}px rgba(90,67,34,0.7)`,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        {/* engraved inner rim */}
        <span
          style={{
            position: "absolute",
            inset: size * 0.1,
            borderRadius: "50%",
            border: "1px solid #a07d1a88",
            boxShadow: "inset 0 1px 2px rgba(0,0,0,0.18)",
          }}
        />
        <span
          className="font-display"
          style={{
            fontSize: size * 0.42,
            fontWeight: 700,
            color: "#6e520f",
            letterSpacing: -1,
            textShadow: "0 1px 0 rgba(255,255,255,0.55), 0 -1px 1px rgba(0,0,0,0.25)",
          }}
        >
          {level}
        </span>
      </span>
    </span>
  );
}

/**
 * LevelUpDialog — the post-match level-up celebration (Story 9.5 follow-up).
 * Styled to match DailyRewardDialog: a forced modal (no outside/Escape dismiss
 * via `disablePointerDismissal` + the no-op `onOpenChange`, no close button)
 * that closes only through the Continue CTA. Shown by LevelUpGate once the
 * player lands back in the lobby/room after a match that leveled them up —
 * never on the match-end screen itself.
 */
export function LevelUpDialog({ open, level, newTotalXp, xpEarned, onClose }: LevelUpDialogProps) {
  const { t } = useTranslation();
  const { xpIntoLevel, xpForNextLevel, fraction } = xpBarFill(newTotalXp, level);

  return (
    <Dialog open={open} disablePointerDismissal onOpenChange={() => {}}>
      <DialogContent
        showCloseButton={false}
        data-testid="level-up-dialog"
        className={cn(
          "block overflow-hidden rounded-[var(--radius-lg)] border border-border bg-surface p-0 ring-0 sm:max-w-[380px]",
          "shadow-[0_0_0_1px_rgba(232,194,90,0.18)_inset,0_30px_70px_-28px_rgba(14,58,36,0.45)]",
        )}
      >
        {/* top brass hairline accent */}
        <div className="h-[3px] bg-[linear-gradient(90deg,transparent,var(--brass)_28%,var(--team-a-fill)_50%,var(--brass)_72%,transparent)]" />
        {/* faint top halo */}
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-x-0 top-0 h-[180px] bg-[radial-gradient(ellipse_70%_100%_at_50%_0%,rgba(232,194,90,0.18),transparent_70%)]"
        />

        <div className="relative flex flex-col items-center gap-4 px-[26px] pt-[26px] pb-6 text-center">
          <DialogTitle className="text-brass-deep text-[11px] font-bold tracking-[2px] uppercase">
            {t("xp.levelUpDialog.title")}
          </DialogTitle>

          <LevelMedallion level={level} />

          <div>
            <div
              data-testid="level-up-level"
              className="font-display text-team-a text-[34px] leading-none font-bold tracking-[-1px]"
            >
              {t("xp.levelLabel", { level })}
            </div>
            <DialogDescription
              data-testid="level-up-earned"
              className="text-ink-dim mt-1.5 text-sm tracking-[0.3px]"
            >
              {t("xp.levelUpDialog.earned", { amount: xpEarned })}
            </DialogDescription>
          </div>

          <div className="w-full">
            <div className="text-ink-dim mb-1 flex justify-end text-[12.5px] tabular-nums">
              {t("xp.progress", { current: xpIntoLevel, next: xpForNextLevel })}
            </div>
            <XpBar
              fraction={fraction}
              label={t("xp.progressLabel", {
                level,
                current: xpIntoLevel,
                next: xpForNextLevel,
              })}
              testId="level-up-xp-bar"
            />
          </div>

          <Button onClick={onClose} size="cta" className="w-full" data-testid="level-up-continue">
            {t("xp.levelUpDialog.continue")}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
