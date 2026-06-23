import { Outlet } from "react-router";

import { DailyRewardGate } from "@/features/lobby/components/DailyRewardGate";
import { LevelUpGate } from "@/shared/components/LevelUpGate";
import { TopBar } from "@/shared/components/TopBar";

export function AppLayout() {
  return (
    <div className="min-h-screen">
      <TopBar showNav showUserMenu persistLanguage />
      {/* Fires the once-per-session daily-login claim and shows the reward
          dialog when granted. Lives here so it covers every authed entry path. */}
      <DailyRewardGate />
      {/* Celebrates a post-match level-up once the player is back in the
          lobby/room — AppLayout doesn't wrap the match route, so it never
          shows on the match-end screen. */}
      <LevelUpGate />
      <main>
        <Outlet />
      </main>
    </div>
  );
}
