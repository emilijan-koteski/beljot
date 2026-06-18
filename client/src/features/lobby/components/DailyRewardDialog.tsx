import { Coins } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/shared/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/shared/components/ui/dialog";

interface DailyRewardDialogProps {
  open: boolean;
  amount: number;
  streakDay: number;
  newBalance: number;
  onClose: () => void;
}

/**
 * DailyRewardDialog is the persistent daily-login reward modal (AC #2). It is
 * fully controlled and opened programmatically by the gate — never via a
 * trigger. It stays open until the player clicks Collect: `disablePointerDismissal`
 * blocks outside-click dismissal and the no-op `onOpenChange` swallows every
 * library-initiated close (outside press / Escape / focus-out), so the only
 * close path is the explicit Collect button. There is no auto-dismiss timer.
 */
export function DailyRewardDialog({
  open,
  amount,
  streakDay,
  newBalance,
  onClose,
}: DailyRewardDialogProps) {
  const { t } = useTranslation();

  return (
    <Dialog open={open} disablePointerDismissal onOpenChange={() => {}}>
      <DialogContent
        showCloseButton={false}
        className="sm:max-w-sm"
        data-testid="daily-reward-dialog"
      >
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Coins className="text-brass-deep size-5" aria-hidden="true" />
            {t("rewards.dailyReward.title")}
          </DialogTitle>
          <DialogDescription>
            {t("rewards.dailyReward.streak", { streak: streakDay })}
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col items-center gap-1 py-2">
          <div
            className="text-brass-deep text-2xl font-bold tabular-nums"
            data-testid="daily-reward-amount"
          >
            {t("rewards.dailyReward.amount", { amount: amount.toLocaleString() })}
          </div>
          <div className="text-ink-dim text-sm tabular-nums">
            {t("rewards.dailyReward.balance", { balance: newBalance.toLocaleString() })}
          </div>
        </div>

        <Button onClick={onClose} className="w-full" data-testid="daily-reward-collect">
          {t("rewards.dailyReward.collect")}
        </Button>
      </DialogContent>
    </Dialog>
  );
}
