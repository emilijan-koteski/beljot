import { useTranslation } from "react-i18next";

import { CoinMedallion } from "@/shared/components/CoinMedallion";
import { AppDialog } from "@/shared/components/ui/app-dialog";
import { XpBar } from "@/shared/components/XpBar";
import { xpBarFill } from "@/shared/lib/xpLevel";

interface LevelUpDialogProps {
  open: boolean;
  level: number;
  newTotalXp: number;
  xpEarned: number;
  onClose: () => void;
}

/** Post-match level-up celebration on the celebration shell: the new level, the XP earned and the bar. */
export function LevelUpDialog({ open, level, newTotalXp, xpEarned, onClose }: LevelUpDialogProps) {
  const { t } = useTranslation();
  const { xpIntoLevel, xpForNextLevel, fraction } = xpBarFill(newTotalXp, level);

  return (
    <AppDialog
      open={open}
      onClose={onClose}
      tone="celebration"
      art={<CoinMedallion>{level}</CoinMedallion>}
      title={t("xp.levelUpDialog.title")}
      primary={{
        label: t("xp.levelUpDialog.continue"),
        onClick: onClose,
        testId: "level-up-continue",
      }}
      testId="level-up-dialog"
    >
      <p
        data-testid="level-up-level"
        className="text-team-a m-0 text-5xl leading-none font-bold tracking-tight"
      >
        {t("xp.levelLabel", { level })}
      </p>
      <p data-testid="level-up-earned" className="text-ink-dim mt-2 mb-0 text-sm tabular-nums">
        {t("xp.levelUpDialog.earned", { amount: xpEarned })}
      </p>
      <div className="mt-5 w-full">
        <div className="text-ink-dim mb-1 flex justify-end text-xs tabular-nums">
          {t("xp.progress", { current: xpIntoLevel, next: xpForNextLevel })}
        </div>
        <XpBar
          fraction={fraction}
          label={t("xp.progressLabel", { level, current: xpIntoLevel, next: xpForNextLevel })}
          testId="level-up-xp-bar"
        />
      </div>
    </AppDialog>
  );
}
