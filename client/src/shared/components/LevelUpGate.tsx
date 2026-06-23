import { useLevelUpStore } from "@/shared/stores/levelUpStore";

import { LevelUpDialog } from "./LevelUpDialog";

/**
 * LevelUpGate renders the post-match level-up dialog whenever a level-up is
 * pending on the levelUpStore. Mount it once in the authenticated shell
 * (AppLayout) alongside DailyRewardGate — because AppLayout does NOT wrap the
 * match route, the celebration only ever appears after the player lands back in
 * the lobby/room. Renders nothing until a level-up is pending.
 */
export function LevelUpGate() {
  const pending = useLevelUpStore((s) => s.pending);
  const clear = useLevelUpStore((s) => s.clear);

  return (
    <LevelUpDialog
      open={pending !== null}
      level={pending?.newLevel ?? 0}
      newTotalXp={pending?.newTotalXp ?? 0}
      xpEarned={pending?.xpEarned ?? 0}
      onClose={clear}
    />
  );
}
