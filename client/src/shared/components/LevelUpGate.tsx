import { useLevelUpStore } from "@/shared/stores/levelUpStore";

import { LevelUpDialog } from "./LevelUpDialog";

/**
 * Renders the level-up dialog whenever a level-up is pending on the levelUpStore.
 * Mounted in AppLayout, which does not wrap the match route, so it only appears back in the lobby/room.
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
