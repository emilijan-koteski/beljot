import "@/shared/i18n/i18n";

import { QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useLevelUpStore } from "@/shared/stores/levelUpStore";
import { type RankChangeInfo, useRankChangeStore } from "@/shared/stores/rankChangeStore";
import type { CurrentSeasonResponse } from "@/shared/types/apiTypes";
import { createTestQueryClient } from "@/test-utils";

import { RankChangeGate } from "./RankChangeGate";

const mockGetCurrentSeason = vi.fn();
vi.mock("@/shared/api/season", () => ({
  getCurrentSeason: (...args: unknown[]) => mockGetCurrentSeason(...args),
}));

const promotedToGold2: RankChangeInfo = {
  direction: "promoted",
  rankTier: "gold",
  rankDivision: 2,
  spChange: 40,
  newSeasonSp: 690,
};

const seasonAfterMatch: CurrentSeasonResponse = {
  seasonName: "2026 Q4",
  endsAt: "2099-01-01T00:00:00Z",
  sp: 690,
  rankTier: "gold",
  rankDivision: 2,
  spIntoDivision: 23,
  spForNextDivision: 67,
  gamesPlayed: 12,
  gamesCompleted: 12,
};

function renderGate() {
  return render(
    <QueryClientProvider client={createTestQueryClient()}>
      <RankChangeGate />
    </QueryClientProvider>,
  );
}

describe("RankChangeGate", () => {
  beforeEach(() => {
    useRankChangeStore.getState().clear();
    useLevelUpStore.getState().clear();
    mockGetCurrentSeason.mockReset();
    mockGetCurrentSeason.mockResolvedValue(seasonAfterMatch);
  });

  it("renders nothing when no rank change is pending", () => {
    renderGate();
    expect(screen.queryByTestId("rank-change-dialog")).not.toBeInTheDocument();
  });

  it("shows the dialog when a change is pending and clears it on Continue", async () => {
    const user = userEvent.setup();
    useRankChangeStore.getState().setPending(promotedToGold2);

    renderGate();
    expect(screen.getByTestId("rank-change-dialog")).toHaveAttribute("data-direction", "promoted");
    expect(screen.getByTestId("rank-change-rank")).toHaveTextContent("Gold 2");

    await user.click(screen.getByTestId("rank-change-continue"));

    await waitFor(() => {
      expect(screen.queryByTestId("rank-change-dialog")).not.toBeInTheDocument();
    });
    expect(useRankChangeStore.getState().pending).toBeNull();
  });

  it("renders a demotion in the subdued variant", () => {
    useRankChangeStore.getState().setPending({
      direction: "demoted",
      rankTier: "silver",
      rankDivision: 3,
      spChange: -20,
      newSeasonSp: 590,
    });

    renderGate();
    expect(screen.getByTestId("rank-change-dialog")).toHaveAttribute("data-direction", "demoted");
    expect(screen.getByTestId("rank-change-rank")).toHaveTextContent("Silver 3");
    expect(screen.queryByTestId("rank-change-halo")).not.toBeInTheDocument();
  });

  it("waits for a pending level-up to be dismissed first", async () => {
    useLevelUpStore.getState().setPending({ newLevel: 5, newTotalXp: 1300, xpEarned: 90 });
    useRankChangeStore.getState().setPending(promotedToGold2);

    renderGate();
    expect(screen.queryByTestId("rank-change-dialog")).not.toBeInTheDocument();

    act(() => useLevelUpStore.getState().clear());

    expect(await screen.findByTestId("rank-change-dialog")).toBeInTheDocument();
  });

  it("shows the step bar once the season read has caught up with this match", async () => {
    useRankChangeStore.getState().setPending(promotedToGold2);

    renderGate();

    const bar = await screen.findByTestId("rank-change-progress");
    expect(bar).toHaveAttribute("aria-valuenow", "34");
  });

  it("leaves the bar out while the season read still describes the previous match", async () => {
    mockGetCurrentSeason.mockResolvedValue({
      ...seasonAfterMatch,
      sp: 650,
      rankDivision: 1,
      spIntoDivision: 50,
      spForNextDivision: 67,
    });
    useRankChangeStore.getState().setPending(promotedToGold2);

    renderGate();

    await waitFor(() => expect(mockGetCurrentSeason).toHaveBeenCalled());
    // Let the resolved (stale) read land before asserting it was ignored.
    await act(async () => {});
    expect(screen.getByTestId("rank-change-rank")).toHaveTextContent("Gold 2");
    expect(screen.queryByTestId("rank-change-progress")).not.toBeInTheDocument();
  });

  // Version skew: a tier token this bundle does not know falls back to the SP's
  // own bucket (1100 SP is Diamond 2) for the label AND for the caught-up check.
  it("falls back to the SP bucket for an unrecognised tier token", async () => {
    mockGetCurrentSeason.mockResolvedValue({
      ...seasonAfterMatch,
      sp: 1100,
      rankTier: "mythic",
      rankDivision: 2,
      spIntoDivision: 33,
      spForNextDivision: 67,
    });
    useRankChangeStore.getState().setPending({
      direction: "promoted",
      rankTier: "mythic",
      rankDivision: 2,
      spChange: 30,
      newSeasonSp: 1100,
    });

    renderGate();

    expect(screen.getByTestId("rank-change-rank")).toHaveTextContent("Diamond 2");
    expect(await screen.findByTestId("rank-change-progress")).toHaveAttribute(
      "aria-valuenow",
      "49",
    );
  });

  it("does not read the season while nothing is pending", () => {
    renderGate();
    expect(mockGetCurrentSeason).not.toHaveBeenCalled();
  });
});
