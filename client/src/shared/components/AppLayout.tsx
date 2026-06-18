import { Outlet } from "react-router";

import { DailyRewardGate } from "@/features/lobby/components/DailyRewardGate";
import { TopBar } from "@/shared/components/TopBar";

export function AppLayout() {
  return (
    <div className="min-h-screen">
      <TopBar showNav showUserMenu persistLanguage />
      {/* Fires the once-per-session daily-login claim and shows the reward
          dialog when granted. Lives here so it covers every authed entry path. */}
      <DailyRewardGate />
      <main>
        <Outlet />
      </main>
    </div>
  );
}
