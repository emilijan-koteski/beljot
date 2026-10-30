import "@/shared/i18n/i18n";

import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it } from "vitest";

import { SeasonStandingCard } from "@/shared/components/season/SeasonStandingCard";
import { i18n } from "@/shared/i18n/i18n";
import type { CurrentSeasonResponse, LeaderboardViewer } from "@/shared/types/apiTypes";

const standing: LeaderboardViewer = {
  position: 12,
  userId: 7,
  sp: 340,
  tier: "silver",
  division: 1,
  gamesPlayed: 10,
  gamesWon: 7,
};

const progress: CurrentSeasonResponse = {
  seasonName: "2026 Q3",
  endsAt: new Date(Date.now() + 10 * 86_400_000).toISOString(),
  sp: 340,
  rankTier: "silver",
  rankDivision: 1,
  spIntoDivision: 40,
  spForNextDivision: 100,
  gamesPlayed: 10,
  gamesCompleted: 10,
  gamesWon: 7,
};

function renderCard(props: Partial<Parameters<typeof SeasonStandingCard>[0]> = {}) {
  return render(
    <MemoryRouter>
      <SeasonStandingCard
        standing={standing}
        total={120}
        progress={progress}
        past={false}
        layout="stack"
        {...props}
      />
    </MemoryRouter>,
  );
}

afterEach(cleanup);

describe("SeasonStandingCard", () => {
  it("shows the viewer's place, record and win rate", () => {
    renderCard();
    expect(screen.getByTestId("leaderboard-standing-place")).toHaveTextContent("#12");
    expect(screen.getByTestId("leaderboard-standing-place")).toHaveTextContent("of 120");
    expect(screen.getByTestId("leaderboard-standing-record")).toHaveTextContent("7–3");
    expect(screen.getByTestId("leaderboard-standing-record")).toHaveTextContent("10 played");
    expect(screen.getByTestId("leaderboard-standing-winRate")).toHaveTextContent("70%");
  });

  it("names the SP left to the next rank in the running season", () => {
    renderCard();
    const card = screen.getByTestId("leaderboard-standing");
    expect(card).toHaveTextContent("Silver 1");
    expect(card).toHaveTextContent(
      i18n.t("season.leaderboard.toNext", { sp: 60, rank: "Silver 2" }),
    );
  });

  it("climbs into the next tier from its top division", () => {
    renderCard({ standing: { ...standing, sp: 590, division: 3 } });
    expect(screen.getByTestId("leaderboard-standing")).toHaveTextContent(
      i18n.t("season.leaderboard.toNext", { sp: 60, rank: "Gold 1" }),
    );
  });

  it("marks the top of the ladder instead of a next rank", () => {
    renderCard({
      standing: { ...standing, sp: 1500, tier: "grandmaster", division: null },
      progress: { ...progress, spIntoDivision: 100, spForNextDivision: 0 },
    });
    expect(screen.getByTestId("leaderboard-standing")).toHaveTextContent(
      i18n.t("season.banner.atTop"),
    );
  });

  it("shows the final rank for an ended season", () => {
    renderCard({ past: true, progress: undefined });
    expect(screen.getByTestId("leaderboard-standing")).toHaveTextContent(
      i18n.t("season.leaderboard.finalRank"),
    );
  });

  it("invites a first match when the viewer is not on the running season's board", () => {
    renderCard({ standing: null });
    expect(screen.getByTestId("leaderboard-standing-empty")).toHaveTextContent(
      i18n.t("season.leaderboard.notOnBoard"),
    );
    expect(screen.getByRole("link", { name: i18n.t("nav.play") })).toHaveAttribute(
      "href",
      "/lobby",
    );
  });

  it("renders nothing for an ended season the viewer never played", () => {
    renderCard({ standing: null, past: true, progress: undefined });
    expect(screen.queryByTestId("leaderboard-standing")).not.toBeInTheDocument();
    expect(screen.queryByTestId("leaderboard-standing-empty")).not.toBeInTheDocument();
  });
});
