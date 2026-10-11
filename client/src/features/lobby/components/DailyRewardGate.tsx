import { useDailyRewardGate } from "@/features/lobby/hooks/useDailyRewardGate";

import { DailyRewardDialog } from "./DailyRewardDialog";

/** Runs the once-per-session daily-login claim and shows the reward dialog when a bonus was granted. */
export function DailyRewardGate() {
  const { reward, open, close } = useDailyRewardGate();

  return (
    <DailyRewardDialog
      open={open}
      amount={reward?.amount ?? 0}
      streakDay={reward?.streakDay ?? 0}
      newBalance={reward?.newBalance ?? 0}
      onClose={close}
    />
  );
}
