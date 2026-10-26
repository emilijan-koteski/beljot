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
  level: number;
  newTotalXp: number;
  xpEarned: number;
  onClose: () => void;
}

/** Gold coin medallion with the level number struck into its face. */
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
      <span
        style={{
          position: "absolute",
          inset: -size * 0.28,
          borderRadius: "50%",
          background: "radial-gradient(circle, rgba(232,194,90,0.45) 0%, transparent 68%)",
        }}
      />
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
          className=""
          style={{
            fontSize: size * 0.42,
            fontWeight: 700,
            color: "#6e520f",
            letterSpacing: "-0.025em",
            textShadow: "0 1px 0 rgba(255,255,255,0.55), 0 -1px 1px rgba(0,0,0,0.25)",
          }}
        >
          {level}
        </span>
      </span>
    </span>
  );
}

/** Post-match level-up celebration: a forced modal that closes only through the Continue button. */
export function LevelUpDialog({ open, level, newTotalXp, xpEarned, onClose }: LevelUpDialogProps) {
  const { t } = useTranslation();
  const { xpIntoLevel, xpForNextLevel, fraction } = xpBarFill(newTotalXp, level);

  return (
    <Dialog open={open} disablePointerDismissal onOpenChange={() => {}}>
      <DialogContent
        showCloseButton={false}
        data-testid="level-up-dialog"
        className={cn(
          "block overflow-hidden rounded-lg border border-border bg-surface p-0 ring-0 sm:max-w-[380px]",
          "shadow-xl inset-ring inset-ring-brass/18",
        )}
      >
        <div className="h-[3px] bg-[linear-gradient(90deg,transparent,var(--brass)_28%,var(--team-a-fill)_50%,var(--brass)_72%,transparent)]" />
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-x-0 top-0 h-[180px] bg-[radial-gradient(ellipse_70%_100%_at_50%_0%,rgba(232,194,90,0.18),transparent_70%)]"
        />

        <div className="relative flex flex-col items-center gap-4 px-[26px] pt-[26px] pb-6 text-center">
          <DialogTitle className="text-brass-deep text-2xs font-bold tracking-caps uppercase">
            {t("xp.levelUpDialog.title")}
          </DialogTitle>

          <LevelMedallion level={level} />

          <div>
            <div
              data-testid="level-up-level"
              className="text-team-a text-4xl leading-none font-bold tracking-tight"
            >
              {t("xp.levelLabel", { level })}
            </div>
            <DialogDescription
              data-testid="level-up-earned"
              className="text-ink-dim mt-1.5 text-sm"
            >
              {t("xp.levelUpDialog.earned", { amount: xpEarned })}
            </DialogDescription>
          </div>

          <div className="w-full">
            <div className="text-ink-dim mb-1 flex justify-end text-xs tabular-nums">
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
