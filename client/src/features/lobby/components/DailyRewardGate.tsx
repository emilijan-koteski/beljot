import { useDailyRewardGate } from "@/features/lobby/hooks/useDailyRewardGate";

import { DailyRewardDialog } from "./DailyRewardDialog";

/**
 * DailyRewardGate runs the once-per-session daily-login claim (via the gate
 * hook) and renders the reward dialog when a bonus was granted. Mount it once
 * in the authenticated shell (AppLayout). Renders nothing until a reward opens.
 */
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
