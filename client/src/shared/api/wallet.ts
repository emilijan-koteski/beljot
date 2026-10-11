import { axiosClient } from "@/shared/api/axiosClient";

// Mirrors the server wallet.DailyLoginResult contract (camelCase wire format).
interface DailyLoginResult {
  granted: boolean;
  amount: number;
  streakDay: number;
  newBalance: number;
  loginStreakDays: number;
}

// claimDailyLogin grants the once-per-UTC-day login bonus.
// It is idempotent: a second call the same day returns granted:false.
export function claimDailyLogin(): Promise<DailyLoginResult> {
  return axiosClient.post("/wallet/daily-login");
}
