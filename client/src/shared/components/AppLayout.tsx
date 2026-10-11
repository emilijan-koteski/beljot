import { Outlet } from "react-router";

import { DailyRewardGate } from "@/features/lobby/components/DailyRewardGate";
import { RoomInviteModal } from "@/features/lobby/components/RoomInviteModal";
import { LevelUpGate } from "@/shared/components/LevelUpGate";
import { RankChangeGate } from "@/shared/components/RankChangeGate";
import { TopBar } from "@/shared/components/TopBar";

export function AppLayout() {
  return (
    <div className="min-h-screen">
      <TopBar showNav showUserMenu persistLanguage />
      <DailyRewardGate />
      <LevelUpGate />
      <RankChangeGate />
      <RoomInviteModal />
      <main>
        <Outlet />
      </main>
    </div>
  );
}
