import { useEffect, useRef, useState } from "react";

import { claimDailyLogin } from "@/shared/api/wallet";
import { useAuthStore } from "@/shared/stores/authStore";

export interface DailyReward {
  amount: number;
  streakDay: number;
  newBalance: number;
}

/**
 * useDailyRewardGate fires the daily-login claim exactly once per authenticated
 * app session and exposes the reward to display. Mounted in the authenticated
 * shell (AppLayout) so it covers explicit login, registration, AND refresh-token
 * auto-login uniformly — the single bootstrap entry point.
 *
 * It always refreshes the stored balance/streak from the response, and only
 * surfaces a reward (opening the dialog) when the server actually granted one.
 *
 * StrictMode safety: a synchronous ref guard ensures only ONE request fires
 * across React 18's mount→unmount→remount cycle. We deliberately do NOT cancel
 * the in-flight request on cleanup — doing so would suppress the reward on the
 * StrictMode remount. The endpoint is idempotent server-side regardless, so a
 * stray second call could never double-grant.
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
          // Immutable replace — keep balance/streak on authStore.user in sync.
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
      .catch(() => {
        // Non-fatal: a failed daily-login claim must never block the app. Next
        // session retries; the idempotent endpoint makes retries safe.
      });
  }, [token, isLoading]);

  return {
    reward,
    open: reward !== null,
    close: () => setReward(null),
  };
}
