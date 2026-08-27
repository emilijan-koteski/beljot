import "@/shared/i18n/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SeasonSection } from "@/features/profile/components/SeasonSection";
import { getSeasonArchive } from "@/shared/api/season";
import type { SeasonArchiveResponse, SeasonRank } from "@/shared/types/apiTypes";

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
        sp: 1800,
        tier: "silver",
        gamesPlayed: 14,
        startedAt: "2026-04-01T00:00:00Z",
        endsAt: "2026-07-01T00:00:00Z",
      },
      {
        seasonId: 4,
        seasonName: "2026 Q1",
        sp: 0,
        tier: "iron",
        gamesPlayed: 2,
        startedAt: "2026-01-01T00:00:00Z",
        endsAt: "2026-04-01T00:00:00Z",
      },
    ],
    ...over,
  };
}

const rank: SeasonRank = { seasonName: "2026 Q3", tier: "gold", sp: 4000 };

function renderSection(props: {
  userId: number | undefined;
  seasonRank: SeasonRank | null | undefined;
}) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
  return render(<SeasonSection {...props} />, { wrapper });
}

describe("SeasonSection", () => {
  beforeEach(() => {
    mockGetArchive.mockReset();
  });

  it("renders the current-rank chip and the archive newest-first", async () => {
    mockGetArchive.mockResolvedValue(archive());

    renderSection({ userId: 2, seasonRank: rank });

    const chip = screen.getByTestId("profile-season");
    expect(chip).toHaveAttribute("data-tier", "gold");
    expect(screen.getByTestId("profile-season-tier").textContent).toBe("Gold");
    expect(chip.textContent).toContain((4000).toLocaleString());
    // The machine token, verbatim.
    expect(chip.textContent).toContain("2026 Q3");

    const list = await screen.findByTestId("prior-season-archive");
    const rows = list.querySelectorAll('[data-testid="season-archive-row"]');
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveAttribute("data-season", "2026 Q2");
    expect(rows[1]).toHaveAttribute("data-season", "2026 Q1");
    expect(mockGetArchive).toHaveBeenCalledWith(2);
  });

  // The epic AC: the whole archive is OMITTED from the DOM for players with no
  // played seasons — never an empty state.
  it("renders no archive section at all when the archive is empty", async () => {
    mockGetArchive.mockResolvedValue({ items: [] });

    renderSection({ userId: 2, seasonRank: rank });

    // Chip still renders (they have a current rank), archive never appears.
    expect(screen.getByTestId("profile-season")).toBeInTheDocument();
    await waitFor(() => expect(mockGetArchive).toHaveBeenCalled());
    expect(screen.queryByTestId("prior-season-archive")).not.toBeInTheDocument();
  });

  it("hides the rank chip when seasonRank is null, keeping the archive", async () => {
    mockGetArchive.mockResolvedValue(archive());

    renderSection({ userId: 2, seasonRank: null });

    await screen.findByTestId("prior-season-archive");
    expect(screen.queryByTestId("profile-season")).not.toBeInTheDocument();
  });

  it("contributes nothing to the DOM when there is no rank and no archive", async () => {
    mockGetArchive.mockResolvedValue({ items: [] });

    renderSection({ userId: 2, seasonRank: null });

    await waitFor(() => expect(mockGetArchive).toHaveBeenCalled());
    expect(screen.queryByTestId("profile-season-section")).not.toBeInTheDocument();
    expect(screen.queryByTestId("profile-season")).not.toBeInTheDocument();
    expect(screen.queryByTestId("prior-season-archive")).not.toBeInTheDocument();
  });

  // The archive is supplementary history: a failed read hides the list rather
  // than erroring the profile.
  it("renders no archive on a failed read, without an error surface", async () => {
    mockGetArchive.mockRejectedValue(new Error("boom"));

    renderSection({ userId: 2, seasonRank: rank });

    expect(screen.getByTestId("profile-season")).toBeInTheDocument();
    await waitFor(() => expect(mockGetArchive).toHaveBeenCalled());
    expect(screen.queryByTestId("prior-season-archive")).not.toBeInTheDocument();
  });

  it("does not query the archive while the subject id is unresolved", () => {
    renderSection({ userId: undefined, seasonRank: rank });

    expect(mockGetArchive).not.toHaveBeenCalled();
    // The chip needs no id — it comes off the profile response.
    expect(screen.getByTestId("profile-season")).toBeInTheDocument();
  });

  // A played season at 0 SP is a REAL rank: the chip must render Iron rather
  // than treating 0 as absent (Go zero values are real values).
  it("renders a 0-SP current rank as Iron", () => {
    mockGetArchive.mockResolvedValue({ items: [] });

    renderSection({ userId: 2, seasonRank: { seasonName: "2026 Q3", tier: "iron", sp: 0 } });

    const chip = screen.getByTestId("profile-season");
    expect(chip).toHaveAttribute("data-tier", "iron");
    expect(screen.getByTestId("profile-season-tier").textContent).toBe("Iron");
  });

  it("falls back to the SP bucket for an unrecognised rank tier token", () => {
    mockGetArchive.mockResolvedValue({ items: [] });

    renderSection({ userId: 2, seasonRank: { seasonName: "2026 Q3", tier: "mythic", sp: 4000 } });

    expect(screen.getByTestId("profile-season")).toHaveAttribute("data-tier", "gold");
  });

  // A NETWORK BLIP MUST NOT DELETE HISTORY THE READER IS LOOKING AT. React
  // Query keeps the resolved pages when a background refetch fails, so gating
  // the section on `isError` made a failed refetch drop an already-rendered
  // archive and jump the profile's layout.
  it("keeps the rendered archive when a refetch fails", async () => {
    mockGetArchive.mockResolvedValueOnce(archive());
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<SeasonSection userId={2} seasonRank={rank} />, {
      wrapper: ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={qc}>{children}</QueryClientProvider>
      ),
    });

    const list = await screen.findByTestId("prior-season-archive");
    expect(list.querySelectorAll('[data-testid="season-archive-row"]')).toHaveLength(2);

    mockGetArchive.mockRejectedValueOnce(new Error("offline"));
    await qc.refetchQueries({ queryKey: ["season", "archive", 2] });

    await waitFor(() => expect(qc.getQueryState(["season", "archive", 2])?.status).toBe("error"));
    expect(screen.getByTestId("prior-season-archive")).toBeInTheDocument();
    expect(
      screen
        .getByTestId("prior-season-archive")
        .querySelectorAll('[data-testid="season-archive-row"]'),
    ).toHaveLength(2);
  });
});
