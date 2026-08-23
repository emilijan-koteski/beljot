import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, string>) => {
      const translations: Record<string, string> = {
        "match.matchResult.title": "Match Complete",
        "match.matchResult.duration": "Match Duration",
        "match.matchResult.returnToLobby": "Return to Lobby",
        "match.matchResult.returnToRoom": "Return to Room",
        "team.us": "Us",
        "team.them": "Them",
        "match.surrender.unknownProposer": "your opponent",
      };
      if (key === "match.matchResult.winnerUs") return "We Won!";
      if (key === "match.matchResult.winnerThem") return "They Won!";
      if (key === "match.matchResult.surrenderNote" && opts) {
        return `${opts.username} surrendered the match`;
      }
      if (key === "match.settlement.won" && opts) return `You won ${opts.amount} coins`;
      if (key === "match.settlement.lost" && opts) return `You lost ${opts.amount} coins`;
      return translations[key] ?? key;
    },
  }),
}));

// The room's last match feeds the collapsible hand breakdown. Mocked per test:
// undefined data (the default) must leave the whole section unrendered.
const mockGetRoomLastMatch = vi.fn();
vi.mock("@/shared/api/matches", () => ({
  getRoomLastMatch: (...args: unknown[]) => mockGetRoomLastMatch(...args),
}));

// Friendship reads back the per-player Add-friend row inside the breakdown.
const mockGetFriendshipStatus = vi.fn(
  (): Promise<FriendshipStatus> => Promise.resolve({ status: "none", requestId: null }),
);
vi.mock("@/shared/api/friends", () => ({
  getFriendshipStatus: () => mockGetFriendshipStatus(),
  sendFriendRequest: vi.fn(),
  acceptFriendRequest: vi.fn(),
  declineFriendRequest: vi.fn(),
  removeFriend: vi.fn(),
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import { FetchError } from "@/shared/api/axiosClient";
import type { MatchListItem } from "@/shared/api/matches";
import { queryKeys } from "@/shared/api/queryKeys";
import type { FriendshipStatus } from "@/shared/types/apiTypes";
import type { TeamString } from "@/shared/types/matchTypes";
import type { MatchEndPayload } from "@/shared/types/wsEvents";

import { MatchResult } from "./MatchResult";

function makeClient() {
  return new QueryClient({ defaultOptions: { queries: { gcTime: 0 } } });
}

function Providers({ children, client }: { children: ReactNode; client: QueryClient }) {
  return (
    <QueryClientProvider client={client}>
      <MemoryRouter>{children}</MemoryRouter>
    </QueryClientProvider>
  );
}

/** Asserts `first` precedes `second` in the DOM (sibling order, not nesting). */
function expectPrecedes(firstTestId: string, secondTestId: string) {
  const first = screen.getByTestId(firstTestId);
  const second = screen.getByTestId(secondTestId);
  expect(
    first.compareDocumentPosition(second),
    `expected ${firstTestId} to be followed by a sibling ${secondTestId}`,
  ).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
}

function makeLastMatch(overrides: Partial<MatchListItem> = {}): MatchListItem {
  return {
    id: 42,
    variant: "bitola",
    matchMode: "1001",
    startedAt: "2026-08-01T12:00:00Z",
    completedAt: "2026-08-01T12:25:00Z",
    status: "completed",
    winnerTeam: 0,
    teamAScore: 1020,
    teamBScore: 850,
    hasBots: false,
    viewerSeat: 0,
    outcome: "win",
    endReason: "natural",
    players: [
      { seat: 0, userId: 10, username: "viewer", isBot: false },
      { seat: 1, userId: 11, username: "opp1", isBot: false },
      { seat: 2, userId: 12, username: "mate", isBot: false },
      { seat: 3, userId: 13, username: "opp2", isBot: false },
    ],
    hands: [
      {
        handNumber: 1,
        teamACardPoints: 90,
        teamBCardPoints: 72,
        teamADeclPoints: 20,
        teamBDeclPoints: 0,
        lastTrickTeam: 0,
        lastTrickBonus: 10,
        capot: false,
        capotTeam: undefined,
        capotBonus: 0,
        failedContract: false,
        contractingTeam: 0,
        teamAHandTotal: 120,
        teamBHandTotal: 72,
      },
    ],
    ...overrides,
  };
}

const matchData: MatchEndPayload = {
  winnerTeam: 0,
  teamAFinalScore: 1020,
  teamBFinalScore: 850,
  matchDurationSec: 725,
};

interface RenderOverrides {
  data?: MatchEndPayload;
  viewerTeam?: TeamString;
  onReturnToLobby?: () => void;
  onReturnToRoom?: () => void;
  surrenderedByUsername?: string;
  coinDelta?: number;
  roomId?: number;
  client?: QueryClient;
}

function renderResult(overrides: RenderOverrides = {}) {
  const props = {
    data: overrides.data ?? matchData,
    viewerTeam: overrides.viewerTeam ?? ("teamA" as TeamString),
    onReturnToLobby: overrides.onReturnToLobby ?? vi.fn(),
    onReturnToRoom: overrides.onReturnToRoom ?? vi.fn(),
    surrenderedByUsername: overrides.surrenderedByUsername,
    coinDelta: overrides.coinDelta,
    roomId: overrides.roomId,
  };
  const client = overrides.client ?? makeClient();
  return {
    client,
    ...render(
      <Providers client={client}>
        <MatchResult {...props} />
      </Providers>,
    ),
  };
}

describe("MatchResult", () => {
  beforeEach(() => {
    mockGetRoomLastMatch.mockReset();
    mockGetRoomLastMatch.mockReturnValue(new Promise(() => {}));
    mockGetFriendshipStatus.mockReset();
    mockGetFriendshipStatus.mockResolvedValue({ status: "none", requestId: null });
  });

  it("renders winner banner with 'We Won!' when viewer is on the winning team", () => {
    renderResult({ viewerTeam: "teamA" });

    expect(screen.getByTestId("match-result")).toBeInTheDocument();
    expect(screen.getByTestId("match-result-title")).toHaveTextContent("Match Complete");
    const winner = screen.getByTestId("match-result-winner");
    expect(winner).toHaveTextContent("We Won!");
    expect(winner).toHaveAttribute("data-team", "teamA");
  });

  it("renders winner banner with 'They Won!' when viewer is NOT on the winning team", () => {
    renderResult({ viewerTeam: "teamB" });

    const winner = screen.getByTestId("match-result-winner");
    expect(winner).toHaveTextContent("They Won!");
    expect(winner).toHaveAttribute("data-team", "teamA");
  });

  it("renders final scores and column data-team attributes", () => {
    renderResult({ viewerTeam: "teamA" });

    expect(screen.getByTestId("match-result-team-a-score")).toHaveTextContent("1020");
    expect(screen.getByTestId("match-result-team-b-score")).toHaveTextContent("850");
    expect(screen.getByTestId("match-result-team-a-column")).toHaveAttribute("data-team", "teamA");
    expect(screen.getByTestId("match-result-team-b-column")).toHaveAttribute("data-team", "teamB");
  });

  it("renders score column labels viewer-relative — viewer on teamA sees Us / Them", () => {
    renderResult({ viewerTeam: "teamA" });

    expect(screen.getByTestId("match-result-team-a-column")).toHaveTextContent("Us");
    expect(screen.getByTestId("match-result-team-b-column")).toHaveTextContent("Them");
  });

  it("renders score column labels viewer-relative — viewer on teamB sees Them / Us", () => {
    renderResult({ viewerTeam: "teamB" });

    expect(screen.getByTestId("match-result-team-a-column")).toHaveTextContent("Them");
    expect(screen.getByTestId("match-result-team-b-column")).toHaveTextContent("Us");
  });

  it("renders viewer's team column first — viewer on teamA", () => {
    const { container } = renderResult({ viewerTeam: "teamA" });
    const cols = container.querySelectorAll<HTMLElement>('[data-testid$="-column"]');
    expect(cols).toHaveLength(2);
    expect(cols[0]).toHaveAttribute("data-team", "teamA");
    expect(cols[1]).toHaveAttribute("data-team", "teamB");
  });

  it("renders viewer's team column first — viewer on teamB", () => {
    const { container } = renderResult({ viewerTeam: "teamB" });
    const cols = container.querySelectorAll<HTMLElement>('[data-testid$="-column"]');
    expect(cols).toHaveLength(2);
    expect(cols[0]).toHaveAttribute("data-team", "teamB");
    expect(cols[1]).toHaveAttribute("data-team", "teamA");
  });

  it("formats match duration correctly", () => {
    renderResult({ viewerTeam: "teamA" });

    // 725 seconds = 12m 5s
    expect(screen.getByTestId("match-result-duration")).toHaveTextContent("12m 5s");
  });

  it("renders teamB winner correctly", () => {
    renderResult({ data: { ...matchData, winnerTeam: 1 }, viewerTeam: "teamB" });

    const winner = screen.getByTestId("match-result-winner");
    expect(winner).toHaveTextContent("We Won!");
    expect(winner).toHaveAttribute("data-team", "teamB");
  });

  it("renders both the Return-to-room and Return-to-lobby actions", () => {
    renderResult();

    expect(screen.getByTestId("match-result-room-btn")).toHaveTextContent("Return to Room");
    expect(screen.getByTestId("match-result-lobby-btn")).toHaveTextContent("Return to Lobby");
  });

  it("calls onReturnToLobby when the lobby button is clicked", async () => {
    const onReturnToLobby = vi.fn();
    renderResult({ onReturnToLobby });

    await userEvent.click(screen.getByTestId("match-result-lobby-btn"));
    expect(onReturnToLobby).toHaveBeenCalledOnce();
  });

  it("calls onReturnToRoom when the room button is clicked", async () => {
    const onReturnToRoom = vi.fn();
    renderResult({ onReturnToRoom });

    await userEvent.click(screen.getByTestId("match-result-room-btn"));
    expect(onReturnToRoom).toHaveBeenCalledOnce();
  });

  it("does NOT render surrender note for natural match-end", () => {
    renderResult({ viewerTeam: "teamA" });
    expect(screen.queryByTestId("match-result-surrender-note")).toBeNull();
  });

  it("renders surrender note when outcomeReason is 'surrender'", () => {
    renderResult({
      data: { ...matchData, outcomeReason: "surrender", surrenderedBySeat: 1 },
      viewerTeam: "teamA",
      surrenderedByUsername: "alice",
    });
    const note = screen.getByTestId("match-result-surrender-note");
    expect(note).toBeInTheDocument();
    expect(note).toHaveTextContent("alice surrendered the match");
  });

  it("falls back to unknownProposer when surrender username is missing", () => {
    renderResult({
      data: { ...matchData, outcomeReason: "surrender", surrenderedBySeat: 1 },
      viewerTeam: "teamA",
    });
    const note = screen.getByTestId("match-result-surrender-note");
    expect(note).toHaveTextContent(/your opponent/);
  });

  // Story 9.2 — coin outcome moved from a transient toast into this dialog.
  it("renders the won amount with a positive coin delta", () => {
    renderResult({ coinDelta: 500 });
    const coins = screen.getByTestId("match-result-coins");
    expect(coins).toHaveTextContent("You won 500 coins");
    expect(coins).toHaveAttribute("data-coin-delta", "500");
  });

  it("renders the lost amount (as a positive number) with a negative coin delta", () => {
    renderResult({ coinDelta: -500 });
    const coins = screen.getByTestId("match-result-coins");
    expect(coins).toHaveTextContent("You lost 500 coins");
    expect(coins).toHaveAttribute("data-coin-delta", "-500");
  });

  it("renders no coin line on a zero delta (lone winner who only recovers their stake)", () => {
    renderResult({ coinDelta: 0 });
    expect(screen.queryByTestId("match-result-coins")).toBeNull();
  });

  it("renders no coin line for a free match (coinDelta undefined)", () => {
    renderResult({});
    expect(screen.queryByTestId("match-result-coins")).toBeNull();
  });
  // --- Collapsible hand breakdown (room last-match stats) ---

  it("renders no stats section without a roomId — there is nothing to read", () => {
    renderResult({});
    expect(mockGetRoomLastMatch).not.toHaveBeenCalled();
    expect(screen.queryByTestId("match-result-stats")).toBeNull();
  });

  it("renders no stats section while the query has no data", () => {
    renderResult({ roomId: 7 });
    expect(screen.queryByTestId("match-result-stats")).toBeNull();
  });

  it("renders no stats section when the room's last match 404s", async () => {
    mockGetRoomLastMatch.mockRejectedValue(new FetchError(404, "NOT_FOUND", "not found"));
    renderResult({ roomId: 7 });
    await waitFor(() => expect(mockGetRoomLastMatch).toHaveBeenCalledWith(7));
    expect(screen.queryByTestId("match-result-stats")).toBeNull();
  });

  it("shows a collapsed toggle once the match arrives, above the footer actions", async () => {
    mockGetRoomLastMatch.mockResolvedValue(makeLastMatch());
    renderResult({ roomId: 7 });

    const toggle = await screen.findByTestId("match-result-stats-toggle");
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByTestId("match-result-stats-panel")).toBeNull();

    // Collapsed section sits ABOVE the two footer actions.
    expectPrecedes("match-result-stats", "match-result-actions");
  });

  it("expands to the stacked hand breakdown plus per-player actions, no reinvite", async () => {
    mockGetRoomLastMatch.mockResolvedValue(makeLastMatch());
    renderResult({ roomId: 7 });

    await userEvent.click(await screen.findByTestId("match-result-stats-toggle"));

    expect(screen.getByTestId("match-result-stats-panel")).toBeInTheDocument();
    expect(screen.getByTestId("match-history-hands-grid")).toBeInTheDocument();
    // handsLayout="stacked" forces the per-hand cards even though jsdom's
    // matchMedia reports the desktop viewport — the desktop grid would
    // overflow the 520px panel.
    expect(screen.getByTestId("match-history-hand-row")).toBeInTheDocument();
    expect(screen.queryByTestId("match-history-hands-grid")?.className).not.toContain(
      "overflow-x-auto",
    );

    // One action row per human co-player, and never a reinvite here: everyone
    // is still present and "Return to room" already covers regrouping.
    const rows = await screen.findAllByTestId("match-player-action-row");
    expect(rows).toHaveLength(3);
    expect(screen.queryByTestId("match-player-reinvite-11")).toBeNull();
  });
  // P1: the cache key is per-ROOM, and the room lobby populates it BEFORE the
  // match starts. With the client-wide 30s staleTime a short match (a
  // surrender) ends inside that window, so the overlay would be handed match
  // N-1 out of cache and paint it as the result the player just lived through.
  it("never paints a cached row from a PREVIOUS match in the same room", async () => {
    const client = makeClient();
    // Same room, different match: the scores do not match this match_end.
    client.setQueryData(
      queryKeys.matches.lastByRoom(7),
      makeLastMatch({ id: 41, teamAScore: 640, teamBScore: 1010, winnerTeam: 1 }),
    );
    mockGetRoomLastMatch.mockReturnValue(new Promise(() => {}));

    renderResult({ roomId: 7, client });

    // Nothing at all until the fresh read lands — not the stale row, not a
    // toggle that would open onto it.
    expect(screen.queryByTestId("match-result-stats")).toBeNull();
    await waitFor(() => expect(mockGetRoomLastMatch).toHaveBeenCalledWith(7));
    expect(screen.queryByTestId("match-result-stats")).toBeNull();
  });

  it("paints the row once the fresh read matches this match", async () => {
    const client = makeClient();
    client.setQueryData(
      queryKeys.matches.lastByRoom(7),
      makeLastMatch({ id: 41, teamAScore: 640, teamBScore: 1010, winnerTeam: 1 }),
    );
    // matchData is 1020 / 850, winnerTeam 0.
    mockGetRoomLastMatch.mockResolvedValue(
      makeLastMatch({ id: 42, teamAScore: 1020, teamBScore: 850, winnerTeam: 0 }),
    );

    renderResult({ roomId: 7, client });

    expect(await screen.findByTestId("match-result-stats-toggle")).toBeInTheDocument();
  });

  // P2: a seat-chip <Link> here would PUSH-navigate out of the match, unmounting
  // MatchPage before the player has returned to the room or left it.
  it("renders no profile links inside the overlay", async () => {
    mockGetRoomLastMatch.mockResolvedValue(
      makeLastMatch({ teamAScore: 1020, teamBScore: 850, winnerTeam: 0 }),
    );
    renderResult({ roomId: 7 });

    await userEvent.click(await screen.findByTestId("match-result-stats-toggle"));
    expect(screen.getAllByTestId("match-seat-chip").length).toBeGreaterThan(0);
    expect(screen.queryAllByRole("link")).toHaveLength(0);
  });

  // P3: RemoveFriendDialog is a z-50 shadcn Dialog; this panel is Z.PROMPT (74).
  it("offers no Remove-friend control inside the overlay", async () => {
    mockGetFriendshipStatus.mockResolvedValue({ status: "friends", requestId: 1 });
    mockGetRoomLastMatch.mockResolvedValue(
      makeLastMatch({ teamAScore: 1020, teamBScore: 850, winnerTeam: 0 }),
    );
    renderResult({ roomId: 7 });

    await userEvent.click(await screen.findByTestId("match-result-stats-toggle"));
    await waitFor(() => expect(screen.getAllByTestId("friend-button-friends")).toHaveLength(3));
    expect(screen.queryByTestId("friend-button-remove")).toBeNull();
  });
});
