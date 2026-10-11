import { axiosClient } from "@/shared/api/axiosClient";

export type StreakKind = "win" | "loss" | "none";

export interface CareerStreak {
  kind: StreakKind;
  length: number;
}

export interface BestHand {
  points: number;
  handNumber: number;
  completedAt: string;
}

export interface PartnerStat {
  userId: number;
  username: string;
  avatarUrl: string | null;
  played: number;
  wins: number;
}

export interface RivalStat {
  userId: number;
  username: string;
  avatarUrl: string | null;
  wins: number;
  losses: number;
}

/**
 * Derived career stats for the profile, returned by GET /users/:id/career.
 * `bestHand` and `lastPlayedAt` are absent for users with no scored hands or matches.
 */
export interface CareerResponse {
  capots: number;
  avgMatchSeconds: number;
  careerPoints: number;
  streak: CareerStreak;
  bestHand?: BestHand;
  lastPlayedAt?: string;
  topPartners: PartnerStat[];
  topRivals: RivalStat[];
}

export function getCareer(userId: number): Promise<CareerResponse> {
  return axiosClient.get(`/users/${userId}/career`);
}
