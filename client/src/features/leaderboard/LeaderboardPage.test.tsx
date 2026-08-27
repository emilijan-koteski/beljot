import "@/shared/i18n/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { LeaderboardPage } from "@/features/leaderboard/LeaderboardPage";
import { getSeasonLeaderboard } from "@/shared/api/season";
import { i18n } from "@/shared/i18n/i18n";
import { useAuthStore } from "@/shared/stores/authStore";
import type { LeaderboardResponse, LeaderboardRow } from "@/shared/types/apiTypes";
import { makeUser } from "@/test-utils";

vi.mock("@/shared/api/season", () => ({
  getSeasonLeaderboard: vi.fn(),
}));

const mockGet = vi.mocked(getSeasonLeaderboard);

/** n rows starting at `from`, descending SP so position order is obvious. */
function rows(from: number, n: number): LeaderboardRow[] {
  return Array.from({ length: n }, (_, i) => ({
    position: from + i,
    userId: from + i,
    username: `p${from + i}`,
    sp: 10_000 - (from + i) * 10,
    tier: "gold",
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
    expect(mockGet).toHaveBeenCalledWith(25, 0);
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
    // Offset comes from the rows already held, not pageSize * pages.
    expect(mockGet).toHaveBeenLastCalledWith(25, 25);
    // Fully loaded — the button retires.
    expect(screen.queryByTestId("leaderboard-load-more")).not.toBeInTheDocument();
  });

  it("hides load more when the first page is already the whole season", async () => {
    mockGet.mockResolvedValue(response({ items: rows(1, 3), total: 3 }));
    renderPage();

    await screen.findAllByTestId("leaderboard-row");
    expect(screen.queryByTestId("leaderboard-load-more")).not.toBeInTheDocument();
  });

  // AC3: the viewer's own row is marked in place when it is on screen, and
  // NOT duplicated into a pinned row.
  it("marks the viewer's row in place without pinning it", async () => {
    mockGet.mockResolvedValue(
      response({
        items: rows(1, 25),
        total: 25,
        viewer: { position: 7, userId: 7, sp: 9930, tier: "gold", gamesPlayed: 5 },
      }),
    );
    renderPage();

    const list = await screen.findAllByTestId("leaderboard-row");
    const marked = list.filter((row) => row.getAttribute("data-self") === "true");
    expect(marked).toHaveLength(1);
    expect(marked[0]).toHaveAttribute("data-user-id", "7");
    expect(screen.queryByTestId("leaderboard-pinned")).not.toBeInTheDocument();
  });

  // AC3: off-page, the standing is PINNED below the list — otherwise a player
  // ranked 340th would have to page thirteen times to find themselves.
  it("pins the viewer's row when it falls outside the loaded pages", async () => {
    mockGet.mockResolvedValue(
      response({
        items: rows(1, 25),
        total: 400,
        viewer: { position: 340, userId: 999, sp: 120, tier: "iron", gamesPlayed: 3 },
      }),
    );
    renderPage();

    const pinned = await screen.findByTestId("leaderboard-pinned");
    expect(pinned).toHaveTextContent("340");
    // The username comes from the auth store: the server's viewer block
    // deliberately carries no name.
    expect(pinned).toHaveTextContent("kiro");
    expect(pinned.querySelector('[data-testid="leaderboard-row"]')).toHaveAttribute(
      "data-self",
      "true",
    );
  });

  // AC4: no standing at all -> no marker anywhere and nothing pinned.
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
          { position: 1, userId: 1, username: "ada", sp: 4000, tier: "gold", gamesPlayed: 31 },
        ],
        total: 1,
      }),
    );
    renderPage();

    const row = await screen.findByTestId("leaderboard-row");
    expect(row.querySelector('[data-testid="leaderboard-games"]')).toHaveTextContent("31");
    expect(row.querySelector('[data-testid="leaderboard-sp"]')).toHaveTextContent(
      (4000).toLocaleString(),
    );
  });

  // --- Review follow-ups (P3, P4, P5, P6, P14) ---

  // P3: the destructive error branch. `isError` was checked BEFORE `data`, so a
  // failed load-more — the click at the bottom of 75 rows the reader had already
  // scrolled through — replaced all of them with one line of error text.
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
    // The 25 rows survive the failure...
    expect(screen.getAllByTestId("leaderboard-row")).toHaveLength(25);
    // ...the whole-body error branch never fires...
    expect(screen.queryByTestId("leaderboard-error")).not.toBeInTheDocument();
    // ...and the failure is recoverable in place.
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

  // The FIRST load failing is still the full-body error — there is nothing to
  // preserve, so the reader must see why the page is blank.
  it("shows the whole-body error when there is nothing loaded to keep", async () => {
    mockGet.mockRejectedValue(new Error("boom"));
    renderPage();

    await waitFor(() => expect(screen.getByTestId("leaderboard-error")).toBeInTheDocument());
    expect(screen.queryByTestId("leaderboard-list")).not.toBeInTheDocument();
  });

  // P4: the button used to be driven by `items.length < pages[0].total` while the
  // FETCH decided from the last page's total. When the total shrank between pages
  // (an account deleted, a row leaving the ladder) the two disagreed and the
  // button survived with nothing to fetch — every click a silent no-op.
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
    // 30 loaded, latest total 28: there is no next page, so no button.
    expect(screen.queryByTestId("leaderboard-load-more")).not.toBeInTheDocument();
  });

  // P4, THE CASE WHERE THE TWO CONDITIONS ACTUALLY DIVERGE. An empty last page
  // while `total` still exceeds the loaded rows: `getNextPageParam` stops (its
  // `items.length === 0` guard), so there is no next page param and
  // `fetchNextPage()` is a silent no-op — but `items.length (25) < total (60)`
  // is still true, so the old condition left a button that did nothing at all,
  // forever. Only `hasNextPage` agrees with the fetch.
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
    // The rows are untouched and the caption still reports the real total, so the
    // reader is not told the list is complete — only that there is nothing more
    // to fetch.
    expect(screen.getAllByTestId("leaderboard-row")).toHaveLength(25);
    expect(screen.getByTestId("leaderboard-count")).toHaveTextContent("60");
  });

  // P4, second half: `standing` and `total` are read from the LAST page, so a
  // reader who has paged deeper sees their current position, not the one from the
  // first response they happened to receive.
  it("reads the viewer standing and the total from the newest page", async () => {
    const user = userEvent.setup();
    mockGet
      .mockResolvedValueOnce(
        response({
          items: rows(1, 25),
          total: 60,
          viewer: { position: 340, userId: 999, sp: 120, tier: "iron", gamesPlayed: 3 },
        }),
      )
      .mockResolvedValueOnce(
        response({
          items: rows(26, 25),
          total: 55,
          offset: 25,
          // Their position improved while they were reading.
          viewer: { position: 310, userId: 999, sp: 180, tier: "iron", gamesPlayed: 4 },
        }),
      );
    renderPage();

    await screen.findByTestId("leaderboard-load-more");
    expect(screen.getByTestId("leaderboard-pinned")).toHaveTextContent("340");

    await user.click(screen.getByTestId("leaderboard-load-more"));

    await waitFor(() => {
      expect(screen.getAllByTestId("leaderboard-row")).toHaveLength(51);
    });
    // The fresher standing, not the stale one from page 0.
    expect(screen.getByTestId("leaderboard-pinned")).toHaveTextContent("310");
    // And the fresher total in the caption.
    expect(screen.getByTestId("leaderboard-count")).toHaveTextContent("55");
  });

  // P5: offset paging over live data can return the same player twice — if
  // someone above the fold gains SP between requests, every row below shifts down
  // and the last row of page 1 reappears first on page 2. Duplicate React keys
  // make React reconcile the wrong nodes.
  it("dedupes a player who appears on two pages", async () => {
    const user = userEvent.setup();
    const first = rows(1, 25);
    // Page 2 repeats the last row of page 1 (userId 25) before continuing.
    const second = [first[24]!, ...rows(26, 4)];
    mockGet
      .mockResolvedValueOnce(response({ items: first, total: 40 }))
      .mockResolvedValueOnce(response({ items: second, total: 40, offset: 25 }));
    renderPage();

    await screen.findByTestId("leaderboard-load-more");
    await user.click(screen.getByTestId("leaderboard-load-more"));

    await waitFor(() => {
      // 25 + 5 items came back, but one was a repeat -> 29 rendered rows.
      expect(screen.getAllByTestId("leaderboard-row")).toHaveLength(29);
    });
    const ids = screen.getAllByTestId("leaderboard-row").map((r) => r.getAttribute("data-user-id"));
    expect(new Set(ids).size).toBe(ids.length);
    expect(ids.filter((id) => id === "25")).toHaveLength(1);
  });

  // P6: the auth store can be unhydrated (a hard refresh straight onto this URL)
  // while the query — which needs only the token — has already resolved. The
  // pinned row was then rendered with an empty name, in the visible cell AND in
  // its spoken summary.
  it("falls back to the you label when the auth store has no username yet", async () => {
    useAuthStore.setState({ token: "t", user: null, isLoading: false });
    mockGet.mockResolvedValue(
      response({
        items: rows(1, 25),
        total: 400,
        viewer: { position: 340, userId: 999, sp: 120, tier: "iron", gamesPlayed: 3 },
      }),
    );
    renderPage();

    const pinned = await screen.findByTestId("leaderboard-pinned");
    const name = pinned.querySelector('[data-testid="leaderboard-username"]');
    expect(name).toHaveTextContent(i18n.t("season.leaderboard.you"));
    expect(name?.textContent?.trim()).not.toBe("");
    // The spoken summary is not left nameless either.
    expect(pinned.querySelector('[data-testid="leaderboard-row-summary"]')).toHaveTextContent(
      i18n.t("season.leaderboard.you"),
    );
  });

  // P14: clicking Load more inserts 25 rows with no event a screen reader can
  // observe, so the caption is a live region and its text is the announcement.
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
});
