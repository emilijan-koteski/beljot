import "@/shared/i18n/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { LeaderboardPage } from "@/features/leaderboard/LeaderboardPage";
import { getSeasonLeaderboard, getSeasons } from "@/shared/api/season";
import { i18n } from "@/shared/i18n/i18n";
import { useAuthStore } from "@/shared/stores/authStore";
import type {
  LeaderboardResponse,
  LeaderboardRow,
  SeasonsListResponse,
} from "@/shared/types/apiTypes";
import { makeUser } from "@/test-utils";

vi.mock("@/shared/api/season", () => ({
  getSeasonLeaderboard: vi.fn(),
  getSeasons: vi.fn(),
}));

const mockGet = vi.mocked(getSeasonLeaderboard);
const mockGetSeasons = vi.mocked(getSeasons);

const DAY = 86_400_000;
function seasonsFixture(): SeasonsListResponse {
  const now = Date.now();
  return {
    items: [
      {
        id: 7,
        name: "2026 Q3",
        startedAt: new Date(now - 30 * DAY).toISOString(),
        endsAt: new Date(now + 60 * DAY).toISOString(),
      },
      {
        id: 5,
        name: "2026 Q2",
        startedAt: new Date(now - 120 * DAY).toISOString(),
        endsAt: new Date(now - 30 * DAY).toISOString(),
      },
    ],
  };
}

/** n rows starting at `from`, descending SP so position order is obvious. */
function rows(from: number, n: number): LeaderboardRow[] {
  return Array.from({ length: n }, (_, i) => ({
    position: from + i,
    userId: from + i,
    username: `p${from + i}`,
    avatarUrl: null,
    sp: 800 - (from + i) * 4,
    tier: "gold",
    division: 2,
    gamesPlayed: 5,
  }));
}

function response(over: Partial<LeaderboardResponse> = {}): LeaderboardResponse {
  return { items: rows(1, 25), total: 25, limit: 25, offset: 0, viewer: null, ...over };
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>
      <MemoryRouter>{children}</MemoryRouter>
    </QueryClientProvider>
  );
  return render(<LeaderboardPage />, { wrapper });
}

describe("LeaderboardPage", () => {
  beforeEach(() => {
    mockGet.mockReset();
    mockGetSeasons.mockReset();
    mockGetSeasons.mockResolvedValue({ items: [] });
    useAuthStore.setState({
      token: "t",
      user: makeUser({ id: 7, username: "kiro" }),
      isLoading: false,
    });
  });

  afterEach(() => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  it("renders the section header", () => {
    mockGet.mockReturnValue(new Promise(() => {}));
    renderPage();
    expect(screen.getByTestId("leaderboard-page")).toBeInTheDocument();
  });

  it("shows a skeleton while the first page is in flight", () => {
    mockGet.mockReturnValue(new Promise(() => {}));
    renderPage();
    expect(screen.getByTestId("leaderboard-loading")).toBeInTheDocument();
  });

  it("shows an error branch when the request fails", async () => {
    mockGet.mockRejectedValue(new Error("boom"));
    renderPage();
    await waitFor(() => expect(screen.getByTestId("leaderboard-error")).toBeInTheDocument());
  });

  it("shows an empty branch for a season nobody has scored in", async () => {
    mockGet.mockResolvedValue(response({ items: [], total: 0 }));
    renderPage();
    await waitFor(() => expect(screen.getByTestId("leaderboard-empty")).toBeInTheDocument());
    expect(screen.queryByTestId("leaderboard-list")).not.toBeInTheDocument();
  });

  it("renders the rows with absolute positions and a showing caption", async () => {
    mockGet.mockResolvedValue(response({ total: 60 }));
    renderPage();

    const list = await screen.findAllByTestId("leaderboard-row");
    expect(list).toHaveLength(25);
    expect(list[0]!.querySelector('[data-testid="leaderboard-position"]')).toHaveTextContent("1");
    expect(screen.getByTestId("leaderboard-count")).toHaveTextContent("25");
    expect(screen.getByTestId("leaderboard-count")).toHaveTextContent("60");
  });

  it("requests the first page with the page size and offset zero", async () => {
    mockGet.mockResolvedValue(response());
    renderPage();

    await screen.findAllByTestId("leaderboard-row");
    expect(mockGet).toHaveBeenCalledWith(25, 0, "current");
  });

  it("appends the next page from the loaded-row offset when load more is clicked", async () => {
    const user = userEvent.setup();
    mockGet
      .mockResolvedValueOnce(response({ items: rows(1, 25), total: 40 }))
      .mockResolvedValueOnce(response({ items: rows(26, 15), total: 40, offset: 25 }));
    renderPage();

    await screen.findByTestId("leaderboard-load-more");
    await user.click(screen.getByTestId("leaderboard-load-more"));

    await waitFor(() => {
      expect(screen.getAllByTestId("leaderboard-row")).toHaveLength(40);
    });
    expect(mockGet).toHaveBeenLastCalledWith(25, 25, "current");
    expect(screen.queryByTestId("leaderboard-load-more")).not.toBeInTheDocument();
  });

  it("hides load more when the first page is already the whole season", async () => {
    mockGet.mockResolvedValue(response({ items: rows(1, 3), total: 3 }));
    renderPage();

    await screen.findAllByTestId("leaderboard-row");
    expect(screen.queryByTestId("leaderboard-load-more")).not.toBeInTheDocument();
  });

  it("marks the viewer's row in place without pinning it", async () => {
    mockGet.mockResolvedValue(
      response({
        items: rows(1, 25),
        total: 25,
        viewer: { position: 7, userId: 7, sp: 772, tier: "gold", division: 3, gamesPlayed: 5 },
      }),
    );
    renderPage();

    const list = await screen.findAllByTestId("leaderboard-row");
    const marked = list.filter((row) => row.getAttribute("data-self") === "true");
    expect(marked).toHaveLength(1);
    expect(marked[0]).toHaveAttribute("data-user-id", "7");
    expect(screen.queryByTestId("leaderboard-pinned")).not.toBeInTheDocument();
  });

  it("pins the viewer's row when it falls outside the loaded pages", async () => {
    mockGet.mockResolvedValue(
      response({
        items: rows(1, 25),
        total: 400,
        viewer: { position: 340, userId: 999, sp: 120, tier: "iron", division: 3, gamesPlayed: 3 },
      }),
    );
    renderPage();

    const pinned = await screen.findByTestId("leaderboard-pinned");
    expect(pinned).toHaveTextContent("340");
    expect(pinned).toHaveTextContent("kiro");
    expect(pinned.querySelector('[data-testid="leaderboard-row"]')).toHaveAttribute(
      "data-self",
      "true",
    );
  });

  it("draws the viewer's own avatar on the pinned row", async () => {
    const url = "https://assets.test/avatars/kiro/128.webp";
    useAuthStore.setState({ user: makeUser({ id: 7, username: "kiro", avatarUrl: url }) });
    mockGet.mockResolvedValue(
      response({
        items: rows(1, 25),
        total: 400,
        viewer: { position: 340, userId: 999, sp: 120, tier: "iron", division: 3, gamesPlayed: 3 },
      }),
    );
    renderPage();

    const pinned = await screen.findByTestId("leaderboard-pinned");
    expect(pinned.querySelector("img")).toHaveAttribute("src", url);
  });

  it("renders no own-row marker and no pinned row when the viewer has no standing", async () => {
    mockGet.mockResolvedValue(response({ viewer: null }));
    renderPage();

    const list = await screen.findAllByTestId("leaderboard-row");
    for (const row of list) {
      expect(row).not.toHaveAttribute("data-self");
    }
    expect(screen.queryByTestId("leaderboard-pinned")).not.toBeInTheDocument();
  });

  it("renders a row's games played alongside its SP", async () => {
    mockGet.mockResolvedValue(
      response({
        items: [
          {
            position: 1,
            userId: 1,
            username: "ada",
            avatarUrl: null,
            sp: 700,
            tier: "gold",
            division: 2,
            gamesPlayed: 31,
          },
        ],
        total: 1,
      }),
    );
    renderPage();

    const row = await screen.findByTestId("leaderboard-row");
    expect(row.querySelector('[data-testid="leaderboard-games"]')).toHaveTextContent("31");
    expect(row.querySelector('[data-testid="leaderboard-sp"]')).toHaveTextContent(
      (700).toLocaleString(),
    );
  });

  it("keeps the loaded rows when fetchNextPage fails, and offers a retry", async () => {
    const user = userEvent.setup();
    mockGet
      .mockResolvedValueOnce(response({ items: rows(1, 25), total: 60 }))
      .mockRejectedValueOnce(new Error("transient"));
    renderPage();

    await screen.findByTestId("leaderboard-load-more");
    await user.click(screen.getByTestId("leaderboard-load-more"));

    await waitFor(() => {
      expect(screen.getByTestId("leaderboard-page-error")).toBeInTheDocument();
    });
    expect(screen.getAllByTestId("leaderboard-row")).toHaveLength(25);
    expect(screen.queryByTestId("leaderboard-error")).not.toBeInTheDocument();
    expect(screen.getByTestId("leaderboard-retry")).toBeInTheDocument();
  });

  it("recovers the next page when the retry succeeds", async () => {
    const user = userEvent.setup();
    mockGet
      .mockResolvedValueOnce(response({ items: rows(1, 25), total: 40 }))
      .mockRejectedValueOnce(new Error("transient"))
      .mockResolvedValueOnce(response({ items: rows(26, 15), total: 40, offset: 25 }));
    renderPage();

    await screen.findByTestId("leaderboard-load-more");
    await user.click(screen.getByTestId("leaderboard-load-more"));
    await screen.findByTestId("leaderboard-retry");

    await user.click(screen.getByTestId("leaderboard-retry"));

    await waitFor(() => {
      expect(screen.getAllByTestId("leaderboard-row")).toHaveLength(40);
    });
    expect(screen.queryByTestId("leaderboard-page-error")).not.toBeInTheDocument();
  });

  it("shows the whole-body error when there is nothing loaded to keep", async () => {
    mockGet.mockRejectedValue(new Error("boom"));
    renderPage();

    await waitFor(() => expect(screen.getByTestId("leaderboard-error")).toBeInTheDocument());
    expect(screen.queryByTestId("leaderboard-list")).not.toBeInTheDocument();
  });

  it("retires load more when the total shrinks below the rows already loaded", async () => {
    const user = userEvent.setup();
    mockGet
      .mockResolvedValueOnce(response({ items: rows(1, 25), total: 60 }))
      .mockResolvedValueOnce(response({ items: rows(26, 5), total: 28, offset: 25 }));
    renderPage();

    await screen.findByTestId("leaderboard-load-more");
    await user.click(screen.getByTestId("leaderboard-load-more"));

    await waitFor(() => {
      expect(screen.getAllByTestId("leaderboard-row")).toHaveLength(30);
    });
    expect(screen.queryByTestId("leaderboard-load-more")).not.toBeInTheDocument();
  });

  it("retires load more when the last page came back empty but total says more", async () => {
    const user = userEvent.setup();
    mockGet
      .mockResolvedValueOnce(response({ items: rows(1, 25), total: 60 }))
      .mockResolvedValueOnce(response({ items: [], total: 60, offset: 25 }));
    renderPage();

    await screen.findByTestId("leaderboard-load-more");
    await user.click(screen.getByTestId("leaderboard-load-more"));

    await waitFor(() => {
      expect(screen.queryByTestId("leaderboard-load-more")).not.toBeInTheDocument();
    });
    expect(screen.getAllByTestId("leaderboard-row")).toHaveLength(25);
    expect(screen.getByTestId("leaderboard-count")).toHaveTextContent("60");
  });

  it("reads the viewer standing and the total from the newest page", async () => {
    const user = userEvent.setup();
    mockGet
      .mockResolvedValueOnce(
        response({
          items: rows(1, 25),
          total: 60,
          viewer: {
            position: 340,
            userId: 999,
            sp: 120,
            tier: "iron",
            division: 3,
            gamesPlayed: 3,
          },
        }),
      )
      .mockResolvedValueOnce(
        response({
          items: rows(26, 25),
          total: 55,
          offset: 25,
          viewer: {
            position: 310,
            userId: 999,
            sp: 140,
            tier: "iron",
            division: 3,
            gamesPlayed: 4,
          },
        }),
      );
    renderPage();

    await screen.findByTestId("leaderboard-load-more");
    expect(screen.getByTestId("leaderboard-pinned")).toHaveTextContent("340");

    await user.click(screen.getByTestId("leaderboard-load-more"));

    await waitFor(() => {
      expect(screen.getAllByTestId("leaderboard-row")).toHaveLength(51);
    });
    expect(screen.getByTestId("leaderboard-pinned")).toHaveTextContent("310");
    expect(screen.getByTestId("leaderboard-count")).toHaveTextContent("55");
  });

  it("dedupes a player who appears on two pages", async () => {
    const user = userEvent.setup();
    const first = rows(1, 25);
    const second = [first[24]!, ...rows(26, 4)];
    mockGet
      .mockResolvedValueOnce(response({ items: first, total: 40 }))
      .mockResolvedValueOnce(response({ items: second, total: 40, offset: 25 }));
    renderPage();

    await screen.findByTestId("leaderboard-load-more");
    await user.click(screen.getByTestId("leaderboard-load-more"));

    await waitFor(() => {
      expect(screen.getAllByTestId("leaderboard-row")).toHaveLength(29);
    });
    const ids = screen.getAllByTestId("leaderboard-row").map((r) => r.getAttribute("data-user-id"));
    expect(new Set(ids).size).toBe(ids.length);
    expect(ids.filter((id) => id === "25")).toHaveLength(1);
  });

  it("falls back to the you label when the auth store has no username yet", async () => {
    useAuthStore.setState({ token: "t", user: null, isLoading: false });
    mockGet.mockResolvedValue(
      response({
        items: rows(1, 25),
        total: 400,
        viewer: { position: 340, userId: 999, sp: 120, tier: "iron", division: 3, gamesPlayed: 3 },
      }),
    );
    renderPage();

    const pinned = await screen.findByTestId("leaderboard-pinned");
    const name = pinned.querySelector('[data-testid="leaderboard-username"]');
    expect(name).toHaveTextContent(i18n.t("season.leaderboard.you"));
    expect(name?.textContent?.trim()).not.toBe("");
    expect(pinned.querySelector('[data-testid="leaderboard-row-summary"]')).toHaveTextContent(
      i18n.t("season.leaderboard.you"),
    );
  });

  it("announces the loaded count through a live region", async () => {
    mockGet.mockResolvedValue(response({ items: rows(1, 25), total: 60 }));
    renderPage();

    const caption = await screen.findByTestId("leaderboard-count");
    expect(caption).toHaveAttribute("aria-live", "polite");
    expect(caption).toHaveAttribute("aria-atomic", "true");
  });

  it("marks the loading skeleton as a busy status region", () => {
    mockGet.mockReturnValue(new Promise(() => {}));
    renderPage();

    const skeleton = screen.getByTestId("leaderboard-loading");
    expect(skeleton).toHaveAttribute("role", "status");
    expect(skeleton).toHaveAttribute("aria-busy", "true");
  });

  it("renders the picker newest-first with season tokens verbatim and the current chip marked", async () => {
    mockGetSeasons.mockResolvedValue(seasonsFixture());
    mockGet.mockResolvedValue(response());
    renderPage();

    const picker = await screen.findByTestId("leaderboard-season-picker");
    const chips = picker.querySelectorAll('[role="radio"]');
    expect(chips).toHaveLength(2);
    expect(chips[0]).toHaveTextContent("2026 Q3");
    expect(chips[1]).toHaveTextContent("2026 Q2");
    expect(chips[0]).toHaveAttribute("aria-checked", "true");
    expect(chips[0]).toHaveTextContent(i18n.t("season.picker.current"));
    expect(chips[1]).toHaveAttribute("aria-checked", "false");
  });

  it("renders no picker while the seasons list is empty or failed", async () => {
    mockGetSeasons.mockRejectedValue(new Error("boom"));
    mockGet.mockResolvedValue(response());
    renderPage();

    await screen.findAllByTestId("leaderboard-row");
    expect(screen.queryByTestId("leaderboard-season-picker")).not.toBeInTheDocument();
    expect(mockGet).toHaveBeenCalledWith(25, 0, "current");
  });

  it("renders the current season's divisions and an ended season's stored bare tier", async () => {
    const user = userEvent.setup();
    mockGetSeasons.mockResolvedValue(seasonsFixture());
    mockGet.mockImplementation((_limit, _offset, season) =>
      Promise.resolve(
        season === 5
          ? response({
              items: [
                {
                  position: 1,
                  userId: 3,
                  username: "veteran",
                  avatarUrl: null,
                  sp: 3500,
                  tier: "gold",
                  division: null,
                  gamesPlayed: 90,
                },
              ],
              total: 1,
            })
          : response({ items: rows(1, 1), total: 1 }),
      ),
    );
    renderPage();

    const current = await screen.findByTestId("leaderboard-row");
    expect(current.querySelector('[data-testid="leaderboard-tier"]')).toHaveTextContent("Gold 2");

    await user.click(await screen.findByTestId("leaderboard-season-picker-5"));

    await waitFor(() => {
      const row = screen.getByTestId("leaderboard-row");
      expect(row).toHaveAttribute("data-user-id", "3");
    });
    const past = screen.getByTestId("leaderboard-row");
    expect(past.querySelector('[data-testid="leaderboard-tier"]')?.textContent).toBe("Gold");
    expect(past.querySelector('[data-testid="leaderboard-tier-badge"]')).toHaveAttribute(
      "data-tier",
      "gold",
    );
  });

  it("requests the picked ended season by id and resets to a fresh first page", async () => {
    const user = userEvent.setup();
    mockGetSeasons.mockResolvedValue(seasonsFixture());
    mockGet.mockResolvedValue(response());
    renderPage();

    await screen.findByTestId("leaderboard-season-picker");
    await user.click(screen.getByTestId("leaderboard-season-picker-5"));

    await waitFor(() => {
      expect(mockGet).toHaveBeenLastCalledWith(25, 0, 5);
    });
    expect(screen.getByTestId("leaderboard-season-picker-5")).toHaveAttribute(
      "aria-checked",
      "true",
    );
  });

  it("maps picking the current window back to the `current` selector", async () => {
    const user = userEvent.setup();
    mockGetSeasons.mockResolvedValue(seasonsFixture());
    mockGet.mockResolvedValue(response());
    renderPage();

    await screen.findByTestId("leaderboard-season-picker");
    await user.click(screen.getByTestId("leaderboard-season-picker-5"));
    await waitFor(() => expect(mockGet).toHaveBeenLastCalledWith(25, 0, 5));

    await user.click(screen.getByTestId("leaderboard-season-picker-7"));
    await waitFor(() => expect(mockGet).toHaveBeenLastCalledWith(25, 0, "current"));
  });

  it("hides the picker when only one window exists", async () => {
    const now = Date.now();
    mockGetSeasons.mockResolvedValue({
      items: [
        {
          id: 7,
          name: "2026 Q3",
          startedAt: new Date(now - 30 * DAY).toISOString(),
          endsAt: new Date(now + 60 * DAY).toISOString(),
        },
      ],
    });
    mockGet.mockResolvedValue(response());
    renderPage();

    await screen.findByTestId("leaderboard-list");
    expect(screen.queryByTestId("leaderboard-season-picker")).not.toBeInTheDocument();
  });

  it("switches the header copy to the past-season variant for an ended season", async () => {
    const user = userEvent.setup();
    mockGetSeasons.mockResolvedValue(seasonsFixture());
    mockGet.mockResolvedValue(response());
    renderPage();

    await screen.findByTestId("leaderboard-season-picker");
    expect(screen.getByText(i18n.t("season.leaderboard.sub"))).toBeInTheDocument();

    await user.click(screen.getByTestId("leaderboard-season-picker-5"));

    await waitFor(() =>
      expect(
        screen.getByText(i18n.t("season.leaderboard.subPast", { season: "2026 Q2" })),
      ).toBeInTheDocument(),
    );
    expect(screen.queryByText(i18n.t("season.leaderboard.sub"))).not.toBeInTheDocument();
  });

  it("uses the past-season empty copy for an ended season with no scorers", async () => {
    const user = userEvent.setup();
    mockGetSeasons.mockResolvedValue(seasonsFixture());
    mockGet.mockResolvedValue(response({ items: [], total: 0 }));
    renderPage();

    await screen.findByTestId("leaderboard-season-picker");
    await user.click(screen.getByTestId("leaderboard-season-picker-5"));

    await waitFor(() =>
      expect(screen.getByTestId("leaderboard-empty").textContent).toBe(
        i18n.t("season.leaderboard.emptyPast"),
      ),
    );
  });

  it("invalidates the ladder and the seasons list when the newest window ends", async () => {
    const now = Date.now();
    mockGetSeasons.mockResolvedValue({
      items: [
        {
          id: 7,
          name: "2026 Q3",
          startedAt: new Date(now - 120 * DAY).toISOString(),
          endsAt: new Date(now - DAY).toISOString(),
        },
        {
          id: 5,
          name: "2026 Q2",
          startedAt: new Date(now - 210 * DAY).toISOString(),
          endsAt: new Date(now - 120 * DAY).toISOString(),
        },
      ],
    });
    mockGet.mockResolvedValue(response());

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const spy = vi.spyOn(qc, "invalidateQueries");
    render(<LeaderboardPage />, {
      wrapper: ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={qc}>
          <MemoryRouter>{children}</MemoryRouter>
        </QueryClientProvider>
      ),
    });

    await waitFor(() => expect(spy).toHaveBeenCalledWith({ queryKey: ["season", "leaderboard"] }));
    expect(spy).toHaveBeenCalledWith({ queryKey: ["season", "list"] });
    expect(spy).toHaveBeenCalledWith({ queryKey: ["season", "current"] });
  });
});
