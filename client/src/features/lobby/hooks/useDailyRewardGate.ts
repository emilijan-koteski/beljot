import { useEffect, useRef, useState } from "react";

import { claimDailyLogin } from "@/shared/api/wallet";
import { useAuthStore } from "@/shared/stores/authStore";

interface DailyReward {
  amount: number;
  streakDay: number;
  newBalance: number;
}

/**
 * Claims the daily-login reward once per authenticated session, syncs the stored
 * balance and streak, and exposes any granted reward; a failed claim is ignored.
 */
export function useDailyRewardGate() {
  const token = useAuthStore((s) => s.token);
  const isLoading = useAuthStore((s) => s.isLoading);
  const claimedRef = useRef(false);
  const [reward, setReward] = useState<DailyReward | null>(null);

  useEffect(() => {
    if (isLoading || !token || claimedRef.current) {
      return;
    }
    claimedRef.current = true;

    claimDailyLogin()
      .then((res) => {
        const user = useAuthStore.getState().user;
        if (user) {
          useAuthStore.getState().setUser({
            ...user,
            walletBalance: res.newBalance,
            loginStreakDays: res.loginStreakDays,
          });
        }
        if (res.granted) {
          setReward({
            amount: res.amount,
            streakDay: res.streakDay,
            newBalance: res.newBalance,
          });
        }
      })
      .catch(() => undefined);
  }, [token, isLoading]);

  return {
    reward,
    open: reward !== null,
    close: () => setReward(null),
  };
}
