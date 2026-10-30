import "@/shared/i18n/i18n";

import { cleanup, render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it } from "vitest";

import { LeaderboardPodium } from "@/features/leaderboard/components/LeaderboardPodium";
import { SEASON_TIER_COLOR } from "@/shared/lib/seasonTier";
import type { LeaderboardRow } from "@/shared/types/apiTypes";

function row(position: number): LeaderboardRow {
  return {
    position,
    userId: position * 10,
    username: `p${position}`,
    avatarUrl: null,
    sp: 1300 - position * 50,
    tier: "master",
    division: null,
    gamesPlayed: 20,
    gamesWon: 12,
  };
}

function renderPodium(top: LeaderboardRow[], viewerId: number | null = null) {
  return render(
    <MemoryRouter>
      <LeaderboardPodium top={top} viewerId={viewerId} size="sm" />
    </MemoryRouter>,
  );
}

afterEach(cleanup);

describe("LeaderboardPodium", () => {
  it("stands first place in the middle, second on the left and third on the right", () => {
    renderPodium([row(1), row(2), row(3)]);
    const places = screen.getAllByTestId("leaderboard-podium-place");
    expect(places.map((p) => p.getAttribute("data-place"))).toEqual(["2", "1", "3"]);
  });

  it("links each name to the player's profile, and the viewer's to their own", () => {
    renderPodium([row(1), row(2), row(3)], 20);
    const [second, first] = screen.getAllByTestId("leaderboard-podium-place");
    expect(within(first!).getByRole("link", { name: "p1" })).toHaveAttribute("href", "/players/10");
    expect(within(second!).getByRole("link", { name: "p2" })).toHaveAttribute("href", "/profile");
  });

  it("rings each avatar in its medal colour alone", () => {
    renderPodium([row(1), row(2), row(3)]);
    const [second, first, third] = screen.getAllByTestId("leaderboard-podium-ring");
    const edge = (ring: HTMLElement | undefined) => (ring!.firstChild as HTMLElement).style.border;
    expect(edge(first)).toBe(`2px solid ${SEASON_TIER_COLOR.gold}`);
    expect(edge(second)).toBe(`2px solid ${SEASON_TIER_COLOR.silver}`);
    expect(edge(third)).toBe(`2px solid ${SEASON_TIER_COLOR.bronze}`);
  });

  it("keeps the steps in place when the season has fewer than three players", () => {
    renderPodium([row(1), row(2)]);
    const places = screen.getAllByTestId("leaderboard-podium-place");
    expect(places.map((p) => p.getAttribute("data-place"))).toEqual(["2", "1"]);
  });
});
