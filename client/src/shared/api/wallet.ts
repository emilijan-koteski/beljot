import { axiosClient } from "@/shared/api/axiosClient";

// Mirrors the server wallet.DailyLoginResult contract (camelCase wire format).
export interface DailyLoginResult {
  granted: boolean;
  amount: number;
  streakDay: number;
  newBalance: number;
  loginStreakDays: number;
}

// claimDailyLogin grants the once-per-UTC-day login bonus. The endpoint is
// idempotent — a second call the same day returns granted:false. axiosClient's
// response interceptor already unwraps the { data } envelope and maps errors to
// FetchError (incl. the 401 refresh/retry cycle), so callers get the result
// object directly.
export function claimDailyLogin(): Promise<DailyLoginResult> {
  return axiosClient.post("/wallet/daily-login");
}
