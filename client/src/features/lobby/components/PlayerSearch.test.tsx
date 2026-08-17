import "@/shared/i18n/i18n";

import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BrowserRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";

import { PlayerSearch } from "@/features/lobby/components/PlayerSearch";
import { QueryWrapper } from "@/test-utils";

const mockSearchUsers = vi.fn();
const mockNavigate = vi.fn();

vi.mock("react-router", async () => {
  const actual = await vi.importActual("react-router");
  return { ...actual, useNavigate: () => mockNavigate };
});

vi.mock("@/shared/api/users", () => ({
  searchUsers: (...args: unknown[]) => mockSearchUsers(...args),
}));

function renderSearch(props: { onClose?: () => void } = {}) {
  render(
    <QueryWrapper>
      <BrowserRouter>
        <PlayerSearch {...props} />
      </BrowserRouter>
    </QueryWrapper>,
  );
}

afterEach(() => vi.clearAllMocks());

describe("PlayerSearch", () => {
  it("renders results after typing at least 2 characters", async () => {
    const user = userEvent.setup();
    mockSearchUsers.mockResolvedValue([
      { id: 7, username: "alice" },
      { id: 9, username: "alina" },
    ]);
    renderSearch();

    await user.type(screen.getByTestId("player-search"), "al");

    await waitFor(() => expect(mockSearchUsers).toHaveBeenCalledWith("al"));
    const rows = await screen.findAllByTestId("player-search-result");
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent("alice");
    expect(rows[1]).toHaveTextContent("alina");
  });

  it("does not fire a request for a single-character query", async () => {
    const user = userEvent.setup();
    mockSearchUsers.mockResolvedValue([]);
    renderSearch();

    await user.type(screen.getByTestId("player-search"), "a");
    // Wait past the debounce window: the request is gated by `enabled`, so even
    // after the debounce settles no call must be made for a 1-char query. The
    // wait is wrapped in act() because the debounce commits a state update.
    await act(async () => {
      await new Promise((r) => setTimeout(r, 300));
    });
    expect(mockSearchUsers).not.toHaveBeenCalled();
  });

  it("renders the interpolated empty state when there are no matches", async () => {
    const user = userEvent.setup();
    mockSearchUsers.mockResolvedValue([]);
    renderSearch();

    await user.type(screen.getByTestId("player-search"), "zznobody");

    const empty = await screen.findByTestId("player-search-empty");
    expect(empty).toHaveTextContent("zznobody");
    expect(screen.queryByTestId("player-search-result")).toBeNull();
  });

  it("navigates to the public profile when a result is activated", async () => {
    const user = userEvent.setup();
    mockSearchUsers.mockResolvedValue([{ id: 7, username: "alice" }]);
    renderSearch();

    await user.type(screen.getByTestId("player-search"), "al");
    const row = await screen.findByTestId("player-search-result");
    await user.click(row);

    expect(mockNavigate).toHaveBeenCalledWith("/players/7");
  });

  // The panel is opened from the Friends card header and dismissed from there or
  // with Escape — there is no in-field clear button to duplicate that control.
  it("drops the term and asks the host to close on Escape", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    mockSearchUsers.mockResolvedValue([{ id: 7, username: "alice" }]);
    renderSearch({ onClose });

    const input = screen.getByTestId<HTMLInputElement>("player-search");
    await user.type(input, "al");
    expect(input.value).toBe("al");

    await user.type(input, "{Escape}");

    expect(input.value).toBe("");
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("takes the focus on mount, so opening the search is one click", () => {
    mockSearchUsers.mockResolvedValue([]);
    renderSearch();

    expect(screen.getByTestId("player-search")).toHaveFocus();
  });

  it("removes the results list after the input is cleared", async () => {
    const user = userEvent.setup();
    mockSearchUsers.mockResolvedValue([{ id: 7, username: "alice" }]);
    renderSearch();

    const input = screen.getByTestId<HTMLInputElement>("player-search");
    await user.type(input, "al");
    expect(await screen.findByTestId("player-search-result")).toHaveTextContent("alice");

    // keepPreviousData keeps the resolved rows in the query cache, but the list
    // is gated on the active (>=2 char) term, so it must disappear on clear.
    await user.clear(input);
    await waitFor(() => expect(screen.queryByTestId("player-search-result")).toBeNull());
  });

  it("removes the results list when the query is narrowed below 2 characters", async () => {
    const user = userEvent.setup();
    mockSearchUsers.mockResolvedValue([{ id: 7, username: "alice" }]);
    renderSearch();

    const input = screen.getByTestId<HTMLInputElement>("player-search");
    await user.type(input, "al");
    expect(await screen.findByTestId("player-search-result")).toHaveTextContent("alice");

    // Backspacing to a single character drops below the 2-char gate; the stale
    // rows retained by keepPreviousData must not stay on screen.
    await user.type(input, "{backspace}");
    await waitFor(() => expect(screen.queryByTestId("player-search-result")).toBeNull());
  });
});
