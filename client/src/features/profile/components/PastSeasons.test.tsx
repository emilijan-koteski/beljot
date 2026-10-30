import "@/shared/i18n/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { PastSeasons } from "@/features/profile/components/PastSeasons";
import { getSeasonArchive } from "@/shared/api/season";
import { i18n } from "@/shared/i18n/i18n";
import type { SeasonArchiveResponse } from "@/shared/types/apiTypes";

vi.mock("@/shared/api/season", () => ({
  getSeasonArchive: vi.fn(),
}));

const mockGetArchive = vi.mocked(getSeasonArchive);

function archive(over: Partial<SeasonArchiveResponse> = {}): SeasonArchiveResponse {
  return {
    items: [
      {
        seasonId: 5,
        seasonName: "2026 Q2",
        sp: 3500,
        tier: "silver",
        division: null,
        gamesPlayed: 14,
        gamesWon: 7,
        startedAt: "2026-04-01T00:00:00Z",
        endsAt: "2026-07-01T00:00:00Z",
      },
      {
        seasonId: 4,
        seasonName: "2026 Q1",
        sp: 0,
        tier: "iron",
        division: null,
        gamesPlayed: 2,
        gamesWon: 1,
        startedAt: "2026-01-01T00:00:00Z",
        endsAt: "2026-04-01T00:00:00Z",
      },
    ],
    ...over,
  };
}

function renderSeasons(props: { userId: number | undefined; compact?: boolean }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
  return render(<PastSeasons {...props} />, { wrapper });
}

describe("PastSeasons", () => {
  beforeEach(async () => {
    mockGetArchive.mockReset();
    await i18n.changeLanguage("en");
  });

  it("lists the subject's past seasons newest first, each with its record", async () => {
    mockGetArchive.mockResolvedValue(archive());
    renderSeasons({ userId: 7 });

    const rows = await screen.findAllByTestId("season-archive-row");
    expect(rows.map((r) => r.getAttribute("data-season"))).toEqual(["2026 Q2", "2026 Q1"]);
    expect(rows[0]!.querySelector('[data-testid="season-archive-record"]')?.textContent).toBe(
      "7–7 · 50%",
    );
    expect(mockGetArchive).toHaveBeenCalledWith(7);
  });

  it("says there are no finished seasons yet in the full list", async () => {
    mockGetArchive.mockResolvedValue({ items: [] });
    renderSeasons({ userId: 7 });

    expect(await screen.findByTestId("profile-seasons-empty")).toHaveTextContent(
      "No finished seasons yet.",
    );
  });

  it("titles the compact panel and leaves it out without history", async () => {
    mockGetArchive.mockResolvedValue(archive());
    const { unmount } = renderSeasons({ userId: 7, compact: true });
    expect(await screen.findByTestId("profile-season-section")).toHaveTextContent("Past seasons");
    unmount();

    mockGetArchive.mockResolvedValue({ items: [] });
    renderSeasons({ userId: 8, compact: true });
    await waitFor(() => expect(mockGetArchive).toHaveBeenCalledWith(8));
    expect(screen.queryByTestId("profile-season-section")).not.toBeInTheDocument();
  });

  it("keeps the rows when a background refetch fails", async () => {
    mockGetArchive.mockResolvedValueOnce(archive()).mockRejectedValue(new Error("boom"));
    renderSeasons({ userId: 7 });
    expect(await screen.findAllByTestId("season-archive-row")).toHaveLength(2);
  });
});
