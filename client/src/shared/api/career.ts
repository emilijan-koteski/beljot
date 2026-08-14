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
  played: number;
  wins: number;
}

export interface RivalStat {
  userId: number;
  username: string;
  wins: number;
  losses: number;
}

/**
 * Derived career stats powering the profile hero (capots), streak callout,
 * milestones, partner spotlight, and rivalries. Returned by
 * GET /users/:id/career. `bestHand` is absent for users with no scored hands.
 */
export interface CareerResponse {
  capots: number;
  avgMatchSeconds: number;
  /**
   * Lifetime total game points the player scored across their completed matches
   * (Story 11.3 — server sum of the subject's own team score). Surfaced on both
   * the public and self career views.
   */
  careerPoints: number;
  streak: CareerStreak;
  bestHand?: BestHand;
  /** Completed-at of the most recent match, absent if none played. */
  lastPlayedAt?: string;
  topPartners: PartnerStat[];
  topRivals: RivalStat[];
}

export function getCareer(userId: number): Promise<CareerResponse> {
  return axiosClient.get(`/users/${userId}/career`);
}
