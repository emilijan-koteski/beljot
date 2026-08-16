import { Outlet } from "react-router";

import { DailyRewardGate } from "@/features/lobby/components/DailyRewardGate";
import { RoomInviteModal } from "@/features/lobby/components/RoomInviteModal";
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
      {/* Story 11.5: the incoming friend room-invite popup. Mounted HERE, not in
          LobbyPage, because the server marks a friend invitable whenever they are
          online, not in a match and not in a room — which includes anyone reading
          their profile, another player's profile, or the rules. Mounting it on the
          lobby alone made every such invite a silent black hole: pushed, stored,
          never rendered, expired. It renders nothing unless an invite is live. */}
      <RoomInviteModal />
      <main>
        <Outlet />
      </main>
    </div>
  );
}
