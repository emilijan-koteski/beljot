import "@/shared/i18n/i18n";

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import React from "react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { FetchError } from "@/shared/api/axiosClient";
import { formatCoins } from "@/shared/lib/formatCoins";
import { MOTION } from "@/shared/lib/motion";
import { Z } from "@/shared/lib/zLayers";
import { useAuthStore } from "@/shared/stores/authStore";
import { useMatchStore } from "@/shared/stores/matchStore";
import type { MatchState, TrickCard } from "@/shared/types/matchTypes";
import type { HandScoredPayload } from "@/shared/types/wsEvents";

import { MatchPage } from "./MatchPage";

// Mock WebSocket context hooks. The send-message hook returns a stable spy so
// surrender flow tests can assert outgoing actions.
const mockSendMessage = vi.fn();
vi.mock("@/shared/providers/WebSocketContext", () => ({
  useWsSendMessage: () => mockSendMessage,
  useWsConnectionState: () => "connected" as const,
}));

vi.mock("@/shared/providers/WebSocketProvider", () => ({
  WebSocketProvider: ({ children }: { children: React.ReactNode }) => children,
}));

// Rooms API — controllable so the return-to-room flow (D146) can assert distinct
// error copy. getRoom resolves to a healthy membership so the on-mount splash
// check stays quiet when a room param is present.
vi.mock("@/shared/api/rooms", () => ({
  getRoom: vi.fn(),
  leaveRoom: vi.fn(),
  returnToRoom: vi.fn(),
}));

import { getRoom, returnToRoom } from "@/shared/api/rooms";
import { makeRoom, makeRoomDetail, makeUser } from "@/test-utils";

const mockGetRoom = vi.mocked(getRoom);
const mockReturnToRoom = vi.mocked(returnToRoom);

const mockMatchState: MatchState = {
  id: 1,
  roomId: 100,
  variant: "bitola",
  matchMode: "1001",
  phase: "playing",
  handNumber: 1,
  dealerSeat: 0,
  trumpSuit: "S",
  trumpCallerSeat: 0,
  trumpCandidate: null,
  biddingRound: 1,
  biddingPassCount: 0,
  mustPickTrump: false,
  deck: [],
  activePlayerSeat: 0,
  trickNumber: 1,
  currentTrick: [],
  leadSuit: null,
  trickWinnerSeat: null,
  awaitingDeclaration: false,
  declarationsResolved: false,
  players: [
    {
      hand: [{ rank: "K", suit: "S" }],
      seat: 0,
      userId: 10,
      username: "Alice",
      team: "teamA",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
      faceDownCount: 0,
    },
    {
      hand: [{ rank: "7", suit: "H" }],
      seat: 1,
      userId: 20,
      username: "Bob",
      team: "teamB",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
      faceDownCount: 0,
    },
    {
      hand: [{ rank: "A", suit: "D" }],
      seat: 2,
      userId: 30,
      username: "Carol",
      team: "teamA",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
      faceDownCount: 0,
    },
    {
      hand: [{ rank: "9", suit: "C" }],
      seat: 3,
      userId: 40,
      username: "Dave",
      team: "teamB",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
      faceDownCount: 0,
    },
  ],
  teamScores: [0, 0],
  handPoints: [0, 0],
  declarationPoints: [0, 0],
  belotPoints: [0, 0],
  tricksWon: [0, 0],
  pendingBelotSeat: null,
  belotAnnounced: false,
  winnerTeam: null,
  lastHandResult: null,
  turnExpiresAt: null,
  timerDurationSec: 0,
  previousPhase: "" as const,
  pausedPlayers: [false, false, false, false] as [boolean, boolean, boolean, boolean],
  pauseUsed: [false, false, false, false] as [boolean, boolean, boolean, boolean],
  surrenderProposerSeat: null,
  surrenderUsed: [false, false, false, false] as [boolean, boolean, boolean, boolean],
  turnTimeRemaining: 0,
  ownerSeat: 0,
  disconnectedSeat: -1,
  reconnectExpiresAt: null,
  playerReconnectExpiresAt: [null, null, null, null] as [
    string | null,
    string | null,
    string | null,
    string | null,
  ],
};

// `fromRoom: true` simulates the navigate state RoomPage / LobbyPage attach
// when a fresh game starts. Without it, MatchPage skips the splash hold —
// matching the reload / WS-reconnect remount path. `skipSplash` defaults to
// advancing past the hold so existing table-level assertions are unaffected;
// pass `false` when the test itself drives timer progression.
//
// MatchPage uses useBlocker, which only works inside a data router — hence
// createMemoryRouter + RouterProvider. The `path: "*"` route deliberately has
// no `:roomId` param (mirroring the old plain-BrowserRouter helper), so
// roomIdNum stays null and the mount-time getRoom splash check is skipped;
// tests that need the param use an explicit `/match/:roomId` route instead.
function createMatchPageRouter({ fromRoom = false } = {}) {
  return createMemoryRouter([{ path: "*", element: <MatchPage /> }], {
    initialEntries: [fromRoom ? { pathname: "/match/1", state: { fromRoom: true } } : "/match/1"],
  });
}

function renderMatchPage({ fromRoom = false, skipSplash = true } = {}) {
  const router = createMatchPageRouter({ fromRoom });
  const result = render(<RouterProvider router={router} />);
  if (fromRoom && skipSplash) {
    act(() => {
      vi.advanceTimersByTime(MOTION.GAME_STARTING_SPLASH);
    });
  }
  return { ...result, router };
}

describe("MatchPage", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    mockSendMessage.mockClear();
    useMatchStore.getState().reset();
    useAuthStore.setState({
      token: "test-token",
      user: makeUser({
        id: 10,
        email: "a@b.com",
        username: "Alice",
        loginStreakDays: 1,
        createdAt: "",
      }),
      isLoading: false,
    });

    Object.defineProperty(window, "matchMedia", {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  describe("match stake (pot) HUD", () => {
    // Minimal room detail; userId 10 (the beforeEach user) is seated so the
    // mount-time membership check passes and the table renders.
    const roomDetail = (coinBuyIn: number) => ({
      room: {
        id: 1,
        name: "R",
        code: "ABC123",
        ownerId: 10,
        ownerUsername: "Alice",
        variant: "bitola",
        matchMode: "1001",
        timerStyle: "relaxed",
        timerDurationSeconds: null,
        status: "in_progress",
        playerCount: 4,
        isQuickPlay: false,
        coinBuyIn,
        isPrivate: false,
        createdAt: "",
        updatedAt: "",
      },
      players: [
        {
          id: 1,
          roomId: 1,
          userId: 10,
          username: "Alice",
          seat: 0,
          team: "teamA",
          isBot: false,
          createdAt: "",
        },
      ],
      returnedUserIds: [],
    });

    const renderAtRoom1 = () =>
      render(
        <RouterProvider
          router={createMemoryRouter([{ path: "/match/:roomId", element: <MatchPage /> }], {
            initialEntries: ["/match/1"],
          })}
        />,
      );

    it("shows the pot (humans × buy-in) once the room buy-in loads", async () => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      mockGetRoom.mockResolvedValue(roomDetail(500) as any);
      useMatchStore.getState().setMatchState(mockMatchState); // 4 humans
      useMatchStore.getState().setMyPlayerSeat(0);

      renderAtRoom1();
      await act(async () => {});

      // Both breakpoints render a pill in jsdom (CSS toggles visibility); each
      // shows 4 × 500 = 2,000.
      const amounts = screen.getAllByTestId("match-stake-amount");
      expect(amounts.length).toBeGreaterThan(0);
      for (const node of amounts) {
        expect(node).toHaveTextContent(formatCoins(2000));
      }
    });

    it("excludes bots from the pot", async () => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      mockGetRoom.mockResolvedValue(roomDetail(500) as any);
      const withBots: MatchState = {
        ...mockMatchState,
        players: [
          mockMatchState.players[0],
          { ...mockMatchState.players[1], isBot: true, userId: 0, username: "" },
          mockMatchState.players[2],
          { ...mockMatchState.players[3], isBot: true, userId: 0, username: "" },
        ],
      };
      useMatchStore.getState().setMatchState(withBots);
      useMatchStore.getState().setMyPlayerSeat(0);

      renderAtRoom1();
      await act(async () => {});

      const amounts = screen.getAllByTestId("match-stake-amount");
      expect(amounts.length).toBeGreaterThan(0);
      for (const node of amounts) {
        expect(node).toHaveTextContent(formatCoins(1000)); // 2 humans × 500
      }
    });

    it("renders no stake pill for a free room (buy-in 0)", async () => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      mockGetRoom.mockResolvedValue(roomDetail(0) as any);
      useMatchStore.getState().setMatchState(mockMatchState);
      useMatchStore.getState().setMyPlayerSeat(0);

      renderAtRoom1();
      await act(async () => {});

      expect(screen.queryAllByTestId("match-stake")).toHaveLength(0);
    });
  });

  it("renders loading splash when matchState is null", () => {
    renderMatchPage();

    expect(screen.getByTestId("match-starting-splash")).toBeInTheDocument();
  });

  it("uses 'Reconnecting…' copy on refresh / reconnect mounts (no fromRoom flag)", () => {
    // matchState null + no fromRoom => reload-while-game-in-progress path.
    renderMatchPage();

    expect(screen.getByText("Reconnecting to the game…")).toBeInTheDocument();
    expect(screen.queryByText("Match is starting…")).not.toBeInTheDocument();
  });

  it("uses 'Match is starting…' copy when arriving from room lobby", () => {
    // No matchState yet, but fromRoom flag is set — this is the room→game beat,
    // not a recovery. Copy reflects that.
    renderMatchPage({ fromRoom: true, skipSplash: false });

    expect(screen.getByText("Match is starting…")).toBeInTheDocument();
    expect(screen.queryByText("Reconnecting to the game…")).not.toBeInTheDocument();
  });

  it("holds the splash for GAME_STARTING_SPLASH when arriving from room lobby", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage({ fromRoom: true, skipSplash: false });

    // Splash visible, table not yet mounted — even though state is ready.
    expect(screen.getByTestId("match-starting-splash")).toBeInTheDocument();
    expect(screen.queryByTestId("trick-area")).not.toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(MOTION.GAME_STARTING_SPLASH);
    });

    // After the gate elapses, the table mounts.
    expect(screen.queryByTestId("match-starting-splash")).not.toBeInTheDocument();
    expect(screen.getByTestId("trick-area")).toBeInTheDocument();
  });

  it("uses GAME_STARTING_SPLASH_REDUCED hold when reduced-motion is set", () => {
    // Override matchMedia just for this test so useReducedMotion reports true.
    Object.defineProperty(window, "matchMedia", {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: query === "(prefers-reduced-motion: reduce)",
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });

    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage({ fromRoom: true, skipSplash: false });

    expect(screen.getByTestId("match-starting-splash")).toBeInTheDocument();

    // Just before the reduced duration: still visible.
    act(() => {
      vi.advanceTimersByTime(MOTION.GAME_STARTING_SPLASH_REDUCED - 50);
    });
    expect(screen.getByTestId("match-starting-splash")).toBeInTheDocument();

    // Past the reduced duration but well under the normal one: gone.
    act(() => {
      vi.advanceTimersByTime(100);
    });
    expect(screen.queryByTestId("match-starting-splash")).not.toBeInTheDocument();
    expect(screen.getByTestId("trick-area")).toBeInTheDocument();
  });

  it("skips the splash hold on reload / reconnect mounts (no fromRoom flag)", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage(); // default: fromRoom = false

    // Table is mounted immediately — no artificial hold.
    expect(screen.queryByTestId("match-starting-splash")).not.toBeInTheDocument();
    expect(screen.getByTestId("trick-area")).toBeInTheDocument();
  });

  it("clears the splash timer when unmounted before it elapses", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    const { unmount } = renderMatchPage({ fromRoom: true, skipSplash: false });
    expect(screen.getByTestId("match-starting-splash")).toBeInTheDocument();

    unmount();

    // Advancing past the splash duration after unmount must not fire stale
    // setState (would trigger a React warning / test failure under strict
    // detection). The mere absence of console errors here is the assertion.
    act(() => {
      vi.advanceTimersByTime(MOTION.GAME_STARTING_SPLASH);
    });
  });

  it("renders 4 PlayerSeat components when matchState is set", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    expect(screen.getByTestId("player-seat-0")).toBeInTheDocument();
    expect(screen.getByTestId("player-seat-1")).toBeInTheDocument();
    expect(screen.getByTestId("player-seat-2")).toBeInTheDocument();
    expect(screen.getByTestId("player-seat-3")).toBeInTheDocument();
  });

  it("derives myPlayerSeat from matchState.players matching authStore.user.id", () => {
    useMatchStore.getState().setMatchState(mockMatchState);

    renderMatchPage();

    expect(useMatchStore.getState().myPlayerSeat).toBe(0);
  });

  it("renders trick area when matchState is set", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    expect(screen.getByTestId("trick-area")).toBeInTheDocument();
  });

  it("renders hand cards when matchState is set", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    expect(screen.getByTestId("hand-cards")).toBeInTheDocument();
  });

  it("propagates a 501 matchMode snapshot to the score panel target", () => {
    useMatchStore.getState().setMatchState({ ...mockMatchState, matchMode: "501" });
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    expect(screen.getByTestId("score-a")).toHaveTextContent("/ 501");
    expect(screen.getByTestId("score-b")).toHaveTextContent("/ 501");
  });

  it("renders the 1001 target for a snapshot with the default matchMode", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    expect(screen.getByTestId("score-a")).toHaveTextContent("/ 1001");
    expect(screen.getByTestId("score-b")).toHaveTextContent("/ 1001");
  });

  it("shows match result overlay on match_end phase with matchEndData", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    // Set match end data and transition to match_end phase
    act(() => {
      useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "match_end" });
      useMatchStore.getState().setMatchEndData({
        winnerTeam: 0,
        teamAFinalScore: 1020,
        teamBFinalScore: 850,
        matchDurationSec: 300,
      });
    });

    // Match result overlay should appear
    expect(screen.getByTestId("match-result")).toBeInTheDocument();
    expect(screen.getByTestId("match-result-team-a-score")).toHaveTextContent("1020");
  });

  // D145a: a new match's match_state (any live phase) must clear a stale result
  // overlay left from the previous match.
  it("clears a stale result overlay when a fresh match_state arrives", () => {
    useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "match_end" });
    useMatchStore.getState().setMyPlayerSeat(0);
    useMatchStore.getState().setMatchEndData({
      winnerTeam: 0,
      teamAFinalScore: 1020,
      teamBFinalScore: 850,
      matchDurationSec: 300,
    });

    renderMatchPage();
    expect(screen.getByTestId("match-result")).toBeInTheDocument();

    // A new match begins → match_state arrives with a live phase.
    act(() => {
      useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "dealing" });
    });

    expect(screen.queryByTestId("match-result")).not.toBeInTheDocument();
    expect(useMatchStore.getState().matchEndData).toBeNull();
  });

  // D146: distinct return-to-room error copy per failure code.
  it.each([
    ["MATCH_ALREADY_STARTED", 409, "The next match already started without you."],
    ["NOT_IN_ROOM", 404, "You're no longer in this room."],
  ])("surfaces distinct copy when return-to-room fails with %s", async (code, status, message) => {
    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: makeRoom({
          id: 1,
          name: "R",
          code: "ABC123",
          ownerId: 10,
          ownerUsername: "alice",
          variant: "bitola",
          matchMode: "1001",
          timerStyle: "relaxed",
          timerDurationSeconds: null,
          status: "in_progress",
          playerCount: 4,
          isQuickPlay: false,
          coinBuyIn: 0,
          isPrivate: false,
          createdAt: "",
          updatedAt: "",
        }),
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
        ],
        returnedUserIds: [],
      }),
    );
    mockReturnToRoom.mockRejectedValueOnce(new FetchError(status, code, "x"));

    useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "match_end" });
    useMatchStore.getState().setMyPlayerSeat(0);
    useMatchStore.getState().setMatchEndData({
      winnerTeam: 0,
      teamAFinalScore: 1020,
      teamBFinalScore: 850,
      matchDurationSec: 300,
    });

    render(
      <RouterProvider
        router={createMemoryRouter([{ path: "/match/:roomId", element: <MatchPage /> }], {
          initialEntries: ["/match/1"],
        })}
      />,
    );

    expect(screen.getByTestId("match-result")).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByTestId("match-result-room-btn"));
    });

    expect(screen.getByTestId("error-toast")).toHaveTextContent(message);
  });

  // Story 9.3 AC1: an insolvent return is rejected with 409 INSUFFICIENT_COINS.
  // Unlike the other return errors, the seat is gone server-side, so we clear
  // match state and route to the lobby instead of keeping the result overlay.
  it("routes to the lobby (no overlay toast) when return-to-room is rejected for insolvency", async () => {
    mockGetRoom.mockResolvedValue(
      makeRoomDetail({
        room: makeRoom({
          id: 1,
          name: "R",
          code: "ABC123",
          ownerId: 10,
          ownerUsername: "alice",
          variant: "bitola",
          matchMode: "1001",
          timerStyle: "relaxed",
          timerDurationSeconds: null,
          status: "completed",
          playerCount: 4,
          isQuickPlay: false,
          coinBuyIn: 500,
          isPrivate: false,
          createdAt: "",
          updatedAt: "",
        }),
        players: [
          {
            id: 1,
            roomId: 1,
            userId: 10,
            username: "alice",
            seat: 0,
            team: "teamA",
            isBot: false,
            createdAt: "",
          },
        ],
        returnedUserIds: [],
      }),
    );
    mockReturnToRoom.mockRejectedValueOnce(new FetchError(409, "INSUFFICIENT_COINS", "x"));

    useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "match_end" });
    useMatchStore.getState().setMyPlayerSeat(0);
    useMatchStore.getState().setMatchEndData({
      winnerTeam: 0,
      teamAFinalScore: 1020,
      teamBFinalScore: 850,
      matchDurationSec: 300,
    });

    render(
      <RouterProvider
        router={createMemoryRouter(
          [
            { path: "/match/:roomId", element: <MatchPage /> },
            { path: "/lobby", element: <div data-testid="lobby-marker" /> },
          ],
          { initialEntries: ["/match/1"] },
        )}
      />,
    );

    expect(screen.getByTestId("match-result")).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByTestId("match-result-room-btn"));
    });

    expect(screen.getByTestId("lobby-marker")).toBeInTheDocument();
    expect(screen.queryByTestId("error-toast")).not.toBeInTheDocument();
    expect(useMatchStore.getState().matchEndData).toBeNull();
  });

  it("shows error toast when lastError is set and dismisses it on close button click", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    act(() => {
      useMatchStore.getState().setLastError("error:illegal_play");
    });

    const toast = screen.getByTestId("error-toast");
    expect(toast).toHaveTextContent("That card cannot be played");

    fireEvent.click(screen.getByTestId("error-toast-close"));

    expect(screen.queryByTestId("error-toast")).not.toBeInTheDocument();
  });

  it("auto-dismisses the error toast after 3 seconds", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    act(() => {
      useMatchStore.getState().setLastError("error:illegal_play");
    });

    expect(screen.getByTestId("error-toast")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(3000);
    });

    expect(screen.queryByTestId("error-toast")).not.toBeInTheDocument();
  });

  it("re-shows the error toast on a new error after manual dismiss", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    act(() => {
      useMatchStore.getState().setLastError("error:illegal_play");
    });
    fireEvent.click(screen.getByTestId("error-toast-close"));
    expect(screen.queryByTestId("error-toast")).not.toBeInTheDocument();

    act(() => {
      useMatchStore.getState().setLastError("error:not_your_turn");
    });

    expect(screen.getByTestId("error-toast")).toHaveTextContent("It's not your turn");
  });

  // --- Surrender integration tests (Task 9.6) ---

  it("shows SurrenderButton in playing phase, hides in match_end", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    const { rerender, router } = renderMatchPage();
    expect(screen.getByTestId("surrender-button")).toBeInTheDocument();

    act(() => {
      useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "match_end" });
      useMatchStore.getState().setMatchEndData({
        winnerTeam: 0,
        teamAFinalScore: 1020,
        teamBFinalScore: 850,
        matchDurationSec: 300,
      });
    });
    rerender(<RouterProvider router={router} />);

    expect(screen.queryByTestId("surrender-button")).not.toBeInTheDocument();
  });

  it("shows SurrenderPrompt for the partner when surrenderProposerSeat is set", () => {
    // Local player is seat 2 (Carol); proposer is seat 0 (Alice). Partner of
    // proposer is (0 + 2) % 4 == 2, i.e. the local player.
    useAuthStore.setState({
      token: "test-token",
      user: makeUser({
        id: 30,
        email: "c@b.com",
        username: "Carol",
        loginStreakDays: 1,
        createdAt: "",
      }),
      isLoading: false,
    });
    useMatchStore.getState().setMatchState({ ...mockMatchState, surrenderProposerSeat: 0 });
    useMatchStore.getState().setMyPlayerSeat(2);

    renderMatchPage();

    expect(screen.getByTestId("surrender-prompt")).toBeInTheDocument();
    expect(screen.queryByTestId("surrender-opponent-banner")).not.toBeInTheDocument();
  });

  it("shows SurrenderOpponentBanner for opponents when surrenderProposerSeat is set", () => {
    // Local player is seat 1 (Bob, team B); proposer is seat 0 (Alice, team A).
    useAuthStore.setState({
      token: "test-token",
      user: makeUser({
        id: 20,
        email: "b@b.com",
        username: "Bob",
        loginStreakDays: 1,
        createdAt: "",
      }),
      isLoading: false,
    });
    useMatchStore.getState().setMatchState({ ...mockMatchState, surrenderProposerSeat: 0 });
    useMatchStore.getState().setMyPlayerSeat(1);

    renderMatchPage();

    expect(screen.getByTestId("surrender-opponent-banner")).toBeInTheDocument();
    expect(screen.queryByTestId("surrender-prompt")).not.toBeInTheDocument();
  });

  it("hides SurrenderPrompt when surrenderProposerSeat clears", () => {
    useAuthStore.setState({
      token: "test-token",
      user: makeUser({
        id: 30,
        email: "c@b.com",
        username: "Carol",
        loginStreakDays: 1,
        createdAt: "",
      }),
      isLoading: false,
    });
    useMatchStore.getState().setMatchState({ ...mockMatchState, surrenderProposerSeat: 0 });
    useMatchStore.getState().setMyPlayerSeat(2);

    renderMatchPage();
    expect(screen.getByTestId("surrender-prompt")).toBeInTheDocument();

    act(() => {
      useMatchStore.getState().setMatchState({ ...mockMatchState, surrenderProposerSeat: null });
    });

    expect(screen.queryByTestId("surrender-prompt")).not.toBeInTheDocument();
  });

  it("sends action:surrender_request after confirm dialog", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    fireEvent.click(screen.getByTestId("surrender-button"));
    fireEvent.click(screen.getByTestId("surrender-confirm"));

    expect(mockSendMessage).toHaveBeenCalledWith("action:surrender_request", {});
  });

  // --- Emote integration tests (Story 8.3) ---

  it("shows the emote toggle during the playing phase", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    expect(screen.getByTestId("emote-toggle")).toBeInTheDocument();
  });

  it("hides the emote toggle when match has ended", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    const { rerender, router } = renderMatchPage();

    act(() => {
      useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "match_end" });
      useMatchStore.getState().setMatchEndData({
        winnerTeam: 0,
        teamAFinalScore: 1020,
        teamBFinalScore: 850,
        matchDurationSec: 300,
      });
    });
    rerender(<RouterProvider router={router} />);

    expect(screen.queryByTestId("emote-toggle")).not.toBeInTheDocument();
  });

  it("sends action:emote when a tile is clicked", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    fireEvent.click(screen.getByTestId("emote-toggle"));
    fireEvent.click(screen.getByTestId("emote-tile-thumbs_up"));

    expect(mockSendMessage).toHaveBeenCalledWith("action:emote", { emote: "thumbs_up" });
  });

  it("renders an emote bubble at the correct compass position for an opponent", () => {
    // Local player at seat 0 (South). Opponent at seat 2 emotes — bubble
    // should appear at compass 2 (North) from the receiver's perspective.
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    act(() => {
      useMatchStore.getState().setActiveEmote(2, "clap");
    });

    expect(screen.getByTestId("emote-bubble-2")).toBeInTheDocument();
  });

  it("renders the sender's own bubble at South (compass 0)", () => {
    // Local player is seat 1 (Bob). Their own emote should anchor to South
    // (compass 0) regardless of the absolute seat index.
    useAuthStore.setState({
      token: "test-token",
      user: makeUser({
        id: 20,
        email: "b@b.com",
        username: "Bob",
        loginStreakDays: 1,
        createdAt: "",
      }),
      isLoading: false,
    });
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(1);

    renderMatchPage();

    act(() => {
      useMatchStore.getState().setActiveEmote(1, "heart");
    });

    expect(screen.getByTestId("emote-bubble-0")).toBeInTheDocument();
  });

  it("suppresses emote bubbles while the match-end overlay is up", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    const { rerender, router } = renderMatchPage();

    act(() => {
      useMatchStore.getState().setActiveEmote(2, "laugh");
      useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "match_end" });
      useMatchStore.getState().setMatchEndData({
        winnerTeam: 0,
        teamAFinalScore: 1020,
        teamBFinalScore: 850,
        matchDurationSec: 300,
      });
    });
    rerender(<RouterProvider router={router} />);

    expect(screen.queryByTestId("emote-bubble-2")).not.toBeInTheDocument();
  });

  it("elevates seat wrappers above the trick area so emotes/surrender banners aren't hidden by thrown cards", () => {
    // Regression: on phones the EmoteBubble / SurrenderOpponentBanner render
    // INSIDE the seat wrapper, whose `-translate-*` transform forms a stacking
    // context — so their own z is trapped there and can't beat the center
    // TrickArea (rendered later in the DOM at z-auto). The wrapper itself must
    // carry the seats tier (Z.SEATS) to outrank the thrown cards; otherwise the
    // cards paint over the emote/banner (most visible on mobile, where seats
    // overlap the table).
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    for (const compass of [0, 1, 2, 3] as const) {
      expect(screen.getByTestId(`player-seat-${compass}-wrapper`).style.zIndex).toBe(
        String(Z.SEATS),
      );
    }
  });

  it("hides declarationReveal while the table is paused (AC3)", () => {
    useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "paused" });
    useMatchStore.getState().setMyPlayerSeat(0);
    useMatchStore.getState().setDeclarationReveal({
      winnerTeam: 0,
      contested: false,
      declarations: [
        {
          playerSeat: 0,
          type: "sequence",
          cards: ["9S", "TS", "JS", "QS"],
          value: 50,
        },
      ],
    });

    renderMatchPage();

    expect(screen.queryByTestId("declaration-reveal")).not.toBeInTheDocument();
  });

  it("hides dealer indicator when an overlay is up (AC6)", () => {
    useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "match_end" });
    useMatchStore.getState().setMyPlayerSeat(0);
    useMatchStore.getState().setMatchEndData({
      winnerTeam: 0,
      teamAFinalScore: 1020,
      teamBFinalScore: 850,
      matchDurationSec: 300,
    });

    renderMatchPage();

    expect(screen.queryByTestId("dealer-indicator")).not.toBeInTheDocument();
  });

  it("hides belotReveal while a match-end overlay is up (AC3)", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);
    useMatchStore.getState().setBelotReveal({
      playerSeat: 0,
      team: 0,
      cardId: "QS",
    });

    const { rerender, router } = renderMatchPage();

    act(() => {
      useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "match_end" });
      useMatchStore.getState().setMatchEndData({
        winnerTeam: 0,
        teamAFinalScore: 1020,
        teamBFinalScore: 850,
        matchDurationSec: 300,
      });
    });
    rerender(<RouterProvider router={router} />);

    expect(screen.queryByTestId("belot-reveal")).not.toBeInTheDocument();
  });

  // Trick-collect gating: end-of-hand and reveal overlays must wait for the
  // four-card collect sweep (pendingResolvedTrick) to finish before they mount,
  // so players see the final card, the winner, and where the cards went.
  describe("trick-collect gating", () => {
    const collectSnapshot: { trick: TrickCard[]; winnerSeat: number } = {
      trick: [
        { card: { rank: "K", suit: "S" }, playerSeat: 0 },
        { card: { rank: "7", suit: "H" }, playerSeat: 1 },
        { card: { rank: "A", suit: "D" }, playerSeat: 2 },
        { card: { rank: "9", suit: "C" }, playerSeat: 3 },
      ],
      winnerSeat: 2,
    };

    const scorePayload: HandScoredPayload = {
      teamACardPoints: 90,
      teamBCardPoints: 72,
      teamADeclPoints: 0,
      teamBDeclPoints: 0,
      lastTrickTeam: 0,
      lastTrickBonus: 10,
      capot: false,
      capotTeam: null,
      capotBonus: 0,
      failedContract: false,
      contractingTeam: 0,
      teamAHandTotal: 100,
      teamBHandTotal: 62,
      teamAMatchScore: 100,
      teamBMatchScore: 62,
    };

    it("defers the score reveal until the collect snapshot clears (bug 1)", () => {
      useMatchStore.getState().setMatchState(mockMatchState);
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      // Hand-end burst: trick_resolved (snapshot) is immediately followed by
      // hand_scored. The scoreboard must stay hidden while the four cards are
      // still sweeping to the winner.
      act(() => {
        useMatchStore.getState().setPendingResolvedTrick(collectSnapshot);
        useMatchStore.getState().setScoreRevealData(scorePayload);
      });
      expect(screen.queryByTestId("score-reveal")).not.toBeInTheDocument();

      // Collect completes → snapshot cleared (handleFlightComplete) → mounts.
      act(() => {
        useMatchStore.getState().setPendingResolvedTrick(null);
      });
      expect(screen.getByTestId("score-reveal")).toBeInTheDocument();
    });

    it("defers the declaration reveal until the collect snapshot clears (bug 3)", () => {
      useMatchStore.getState().setMatchState(mockMatchState);
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      act(() => {
        useMatchStore.getState().setPendingResolvedTrick(collectSnapshot);
        useMatchStore.getState().setDeclarationReveal({
          winnerTeam: 0,
          contested: false,
          declarations: [
            { playerSeat: 0, type: "sequence", cards: ["9S", "TS", "JS", "QS"], value: 50 },
          ],
        });
      });
      expect(screen.queryByTestId("declaration-reveal")).not.toBeInTheDocument();

      act(() => {
        useMatchStore.getState().setPendingResolvedTrick(null);
      });
      expect(screen.getByTestId("declaration-reveal")).toBeInTheDocument();
    });

    it("shows the belot reveal immediately, even while the collect snapshot is in flight", () => {
      useMatchStore.getState().setMatchState(mockMatchState);
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      // Unlike declarations, the belot reveal is NOT deferred behind the
      // collect sweep — the announcer expects "belote!" the instant they
      // confirm, concurrent with the throw (issue 3).
      act(() => {
        useMatchStore.getState().setPendingResolvedTrick(collectSnapshot);
        useMatchStore.getState().setBelotReveal({ playerSeat: 0, team: 0, cardId: "QS" });
      });
      expect(screen.getByTestId("belot-reveal")).toBeInTheDocument();

      // Still up after the collect completes.
      act(() => {
        useMatchStore.getState().setPendingResolvedTrick(null);
      });
      expect(screen.getByTestId("belot-reveal")).toBeInTheDocument();
    });

    it("shows the score reveal immediately when no collect is in flight", () => {
      useMatchStore.getState().setMatchState(mockMatchState);
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      act(() => {
        useMatchStore.getState().setScoreRevealData(scorePayload);
      });
      expect(screen.getByTestId("score-reveal")).toBeInTheDocument();
    });

    it("shows the final-hand score reveal before the match result (does not skip it)", () => {
      useMatchStore.getState().setMatchState(mockMatchState);
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      // Final-hand burst: trick_resolved (snapshot) → hand_scored → match_end,
      // with match_end landing while the collect is still sweeping. The match
      // result must NOT preempt the (still-gated) score reveal.
      act(() => {
        useMatchStore.getState().setPendingResolvedTrick(collectSnapshot);
        useMatchStore.getState().setScoreRevealData(scorePayload);
        useMatchStore.getState().setMatchEndData({
          winnerTeam: 0,
          teamAFinalScore: 1020,
          teamBFinalScore: 850,
          matchDurationSec: 300,
        });
      });
      expect(screen.queryByTestId("match-result")).not.toBeInTheDocument();
      expect(screen.queryByTestId("score-reveal")).not.toBeInTheDocument();

      // Collect completes → the score reveal surfaces first; the match result
      // still waits until the reveal is dismissed.
      act(() => {
        useMatchStore.getState().setPendingResolvedTrick(null);
      });
      expect(screen.getByTestId("score-reveal")).toBeInTheDocument();
      expect(screen.queryByTestId("match-result")).not.toBeInTheDocument();
    });

    it("defers the capot animation until the collect snapshot clears", () => {
      useMatchStore.getState().setMatchState(mockMatchState);
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      act(() => {
        useMatchStore.getState().setPendingResolvedTrick(collectSnapshot);
        useMatchStore
          .getState()
          .setScoreRevealData({ ...scorePayload, capot: true, capotTeam: 0, capotBonus: 100 });
      });
      expect(screen.queryByTestId("capot-animation")).not.toBeInTheDocument();

      act(() => {
        useMatchStore.getState().setPendingResolvedTrick(null);
      });
      expect(screen.getByTestId("capot-animation")).toBeInTheDocument();
    });

    it("still gates the score reveal for the glow beat under reduced motion", () => {
      // Reduced-motion path runs no flights and clears the snapshot after
      // TRICK_RESOLVE_PAUSE — the reveal must wait that beat, not appear instantly.
      window.matchMedia = vi.fn().mockImplementation((query: string) => ({
        matches: query.includes("prefers-reduced-motion"),
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })) as unknown as typeof window.matchMedia;

      useMatchStore.getState().setMatchState(mockMatchState);
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      act(() => {
        useMatchStore.getState().setPendingResolvedTrick(collectSnapshot);
        useMatchStore.getState().setScoreRevealData(scorePayload);
      });
      expect(screen.queryByTestId("score-reveal")).not.toBeInTheDocument();

      act(() => {
        vi.advanceTimersByTime(MOTION.TRICK_RESOLVE_PAUSE);
      });
      expect(screen.getByTestId("score-reveal")).toBeInTheDocument();
    });
  });

  describe("mid-match back-press blocker", () => {
    // Stack [lobby, match] with the match on top — router.navigate(-1) is the
    // data-router equivalent of the browser back button (a POP), which the
    // useBlocker predicate intercepts while the match is active.
    function renderWithHistoryBeneath() {
      const router = createMemoryRouter([{ path: "*", element: <MatchPage /> }], {
        initialEntries: ["/lobby", "/match/1"],
        initialIndex: 1,
      });
      render(<RouterProvider router={router} />);
      return router;
    }

    it("shows confirm dialog on browser back button and stays if declined", async () => {
      useMatchStore.getState().setMatchState(mockMatchState);
      useMatchStore.getState().setMyPlayerSeat(0);

      const router = renderWithHistoryBeneath();

      // Mock window.confirm to decline leaving
      const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(false);

      await act(async () => {
        await router.navigate(-1);
      });

      expect(confirmSpy).toHaveBeenCalledWith("Leave the game? You may lose your progress.");
      // Game state should not be cleared and the pop is cancelled
      expect(useMatchStore.getState().matchState).not.toBeNull();
      expect(router.state.location.pathname).toBe("/match/1");

      confirmSpy.mockRestore();
    });

    it("clears the game and routes to the lobby when the back confirm is accepted", async () => {
      useMatchStore.getState().setMatchState(mockMatchState);
      useMatchStore.getState().setMyPlayerSeat(0);

      const router = renderWithHistoryBeneath();

      const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(true);

      await act(async () => {
        await router.navigate(-1);
      });

      expect(confirmSpy).toHaveBeenCalledWith("Leave the game? You may lose your progress.");
      // Game store wiped; the blocked pop targeted /lobby, so the blocker
      // takes the proceed() path — the pop itself completes onto the lobby
      // entry beneath.
      expect(useMatchStore.getState().matchState).toBeNull();
      expect(router.state.location.pathname).toBe("/lobby");

      confirmSpy.mockRestore();
    });

    it("does not block the back button once the match has ended", async () => {
      // Distinct routes (not "*") so the pop actually unmounts MatchPage —
      // that unmount is what triggers the end-state store cleanup.
      mockGetRoom.mockResolvedValue(
        makeRoomDetail({
          room: makeRoom({
            id: 1,
            name: "R",
            code: "ABC123",
            ownerId: 10,
            ownerUsername: "alice",
            variant: "bitola",
            matchMode: "1001",
            timerStyle: "relaxed",
            timerDurationSeconds: null,
            status: "in_progress",
            playerCount: 4,
            isQuickPlay: false,
            coinBuyIn: 0,
            isPrivate: false,
            createdAt: "",
            updatedAt: "",
          }),
          players: [
            {
              id: 1,
              roomId: 1,
              userId: 10,
              username: "alice",
              seat: 0,
              team: "teamA",
              isBot: false,
              createdAt: "",
            },
          ],
          returnedUserIds: [],
        }),
      );

      useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "match_end" });
      useMatchStore.getState().setMyPlayerSeat(0);
      useMatchStore.getState().setMatchEndData({
        winnerTeam: 0,
        teamAFinalScore: 1020,
        teamBFinalScore: 850,
        matchDurationSec: 300,
      });

      const router = createMemoryRouter(
        [
          { path: "/match/:roomId", element: <MatchPage /> },
          { path: "/lobby", element: <div data-testid="lobby-after-back" /> },
        ],
        { initialEntries: ["/lobby", "/match/1"], initialIndex: 1 },
      );
      render(<RouterProvider router={router} />);
      expect(screen.getByTestId("match-result")).toBeInTheDocument();

      const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(false);

      await act(async () => {
        await router.navigate(-1);
      });

      // No confirm — the pop goes straight through to the entry beneath, and
      // the unmount cleanup wipes the ended match from the store.
      expect(confirmSpy).not.toHaveBeenCalled();
      expect(router.state.location.pathname).toBe("/lobby");
      expect(screen.getByTestId("lobby-after-back")).toBeInTheDocument();
      expect(useMatchStore.getState().matchEndData).toBeNull();
      expect(useMatchStore.getState().matchState).toBeNull();

      confirmSpy.mockRestore();
    });
  });

  describe("opponent throw flight timing", () => {
    it("never paints the opponent's card at center before the deck→slot flight starts", () => {
      // jsdom reports zeroed rects, which makes rectFrom() return null and the
      // flight push bail. Give every element a plausible rect so the deck and
      // slot measurements succeed like they would in a real browser.
      const gbcrSpy = vi.spyOn(Element.prototype, "getBoundingClientRect").mockReturnValue({
        left: 100,
        top: 100,
        width: 72,
        height: 104,
        right: 172,
        bottom: 204,
        x: 100,
        y: 100,
        toJSON: () => ({}),
      } as DOMRect);

      useMatchStore.getState().setMatchState(mockMatchState);
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      // Opponent at seat 1 (east, compass 1) throws 7H — one store commit,
      // exactly how the WS dispatcher applies event:card_played.
      act(() => {
        useMatchStore.getState().setMatchState({
          ...mockMatchState,
          activePlayerSeat: 2,
          currentTrick: [{ playerSeat: 1, card: { rank: "7", suit: "H" } }],
        });
      });

      // The commit that grows the trick must come up with the flight already
      // active and the static slot card suppressed. If the slot card renders
      // here, the card flashes at center for a frame before the flight is
      // pushed — the "appears, disappears, then slides in" bug.
      expect(
        document.querySelector('[data-testid^="card-flight-throw-opp-1-7H"]'),
      ).toBeInTheDocument();
      expect(screen.queryByTestId("trick-slot-card-1")).not.toBeInTheDocument();

      gbcrSpy.mockRestore();
    });
  });

  describe("belote/rebelote pre-play prompt", () => {
    // Seat 0 holds both trump (spades) K and Q and it is their turn to lead.
    function belotEligibleState(): MatchState {
      return {
        ...mockMatchState,
        phase: "playing",
        trumpSuit: "S",
        activePlayerSeat: 0,
        currentTrick: [],
        awaitingDeclaration: false,
        pendingBelotSeat: null,
        players: mockMatchState.players.map((p) =>
          p.seat === 0
            ? {
                ...p,
                hand: [
                  { rank: "K", suit: "S" },
                  { rank: "Q", suit: "S" },
                ],
              }
            : p,
        ) as MatchState["players"],
      };
    }

    it("prompts to announce before throwing the belot card (no play_card sent yet)", () => {
      useMatchStore.getState().setMatchState(belotEligibleState());
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      act(() => {
        fireEvent.click(screen.getByTestId("playing-card-KS"));
      });

      // The announce/pass dialog appears and the card has NOT been sent.
      expect(screen.getByTestId("belot-prompt")).toBeInTheDocument();
      expect(mockSendMessage).not.toHaveBeenCalledWith("action:play_card", expect.anything());
    });

    it("throws the card on confirm, then announces only once the server confirms the deferred play", () => {
      useMatchStore.getState().setMatchState(belotEligibleState());
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      act(() => {
        fireEvent.click(screen.getByTestId("playing-card-KS"));
      });
      act(() => {
        fireEvent.click(screen.getByTestId("belot-prompt-announce"));
      });

      // Card is thrown immediately; the announce is held back so it can't race
      // play_card on the server (the race that produced "Невалидна акција" and
      // stalled the turn).
      expect(mockSendMessage).toHaveBeenCalledWith("action:play_card", { cardId: "KS" });
      expect(mockSendMessage).not.toHaveBeenCalledWith("action:announce_belot", {});

      // Server registers the deferred play (pendingBelotSeat === my seat) — now
      // the announce is safe to fire.
      act(() => {
        useMatchStore.getState().setMatchState({ ...belotEligibleState(), pendingBelotSeat: 0 });
      });
      expect(mockSendMessage).toHaveBeenCalledWith("action:announce_belot", {});
    });

    it("declines via the local prompt, then sends skip once the server confirms the deferred play", () => {
      useMatchStore.getState().setMatchState(belotEligibleState());
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      act(() => {
        fireEvent.click(screen.getByTestId("playing-card-KS"));
      });
      act(() => {
        fireEvent.click(screen.getByTestId("belot-prompt-decline"));
      });

      expect(mockSendMessage).toHaveBeenCalledWith("action:play_card", { cardId: "KS" });
      expect(mockSendMessage).not.toHaveBeenCalledWith("action:decline_belot", {});

      act(() => {
        useMatchStore.getState().setMatchState({ ...belotEligibleState(), pendingBelotSeat: 0 });
      });
      expect(mockSendMessage).toHaveBeenCalledWith("action:decline_belot", {});
    });
  });

  describe("declaration / belot reveal lifetime (issue 5)", () => {
    const decls = {
      winnerTeam: 0 as const,
      contested: false,
      declarations: [
        { playerSeat: 0, type: "sequence" as const, cards: ["9S", "TS", "JS", "QS"], value: 50 },
      ],
    };

    const aSnapshot: { trick: TrickCard[]; winnerSeat: number } = {
      trick: [
        { card: { rank: "K", suit: "S" }, playerSeat: 0 },
        { card: { rank: "7", suit: "H" }, playerSeat: 1 },
        { card: { rank: "A", suit: "D" }, playerSeat: 2 },
        { card: { rank: "9", suit: "C" }, playerSeat: 3 },
      ],
      winnerSeat: 2,
    };

    it("stays up through a LATER trick's collect — no re-loop every trick (issue 2)", () => {
      useMatchStore.getState().setMatchState(mockMatchState);
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      // Trick 1 resolves: the reveal is deferred behind the trick-1 sweep.
      act(() => {
        useMatchStore.getState().setPendingResolvedTrick(aSnapshot);
        useMatchStore.getState().setDeclarationReveal(decls);
      });
      expect(screen.queryByTestId("declaration-reveal")).not.toBeInTheDocument();

      // Sweep clears → the reveal latches visible.
      act(() => {
        useMatchStore.getState().setPendingResolvedTrick(null);
      });
      expect(screen.getByTestId("declaration-reveal")).toBeInTheDocument();

      // A LATER trick resolves while the reveal is still up (players played fast).
      // It must NOT unmount/re-defer — that's what restarted its countdown every
      // trick and made it re-appear forever. It stays up for its own 8 s.
      act(() => {
        useMatchStore.getState().setPendingResolvedTrick(aSnapshot);
      });
      expect(screen.getByTestId("declaration-reveal")).toBeInTheDocument();
    });

    it("keeps the declaration reveal up across a normal match_state (another player's move)", () => {
      useMatchStore.getState().setMatchState(mockMatchState);
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      act(() => {
        useMatchStore.getState().setDeclarationReveal(decls);
      });
      expect(screen.getByTestId("declaration-reveal")).toBeInTheDocument();

      // Another player plays → a fresh playing match_state lands. The reveal
      // owns its own 8 s countdown + X, so an unrelated state push must NOT
      // close it (that was the bug: the dialog vanished the instant anyone
      // played).
      act(() => {
        useMatchStore.getState().setMatchState({ ...mockMatchState, activePlayerSeat: 1 });
      });
      expect(screen.getByTestId("declaration-reveal")).toBeInTheDocument();
    });

    it("does not resurface a stale declaration reveal after an overlay clears (D69/D71)", () => {
      useMatchStore.getState().setMatchState(mockMatchState);
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      act(() => {
        useMatchStore.getState().setDeclarationReveal(decls);
      });
      expect(screen.getByTestId("declaration-reveal")).toBeInTheDocument();

      // Overlay takes over the table (pause) — the reveal is orphaned (its
      // component unmounts, so its own countdown can't fire it).
      act(() => {
        useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "paused" });
      });
      expect(screen.queryByTestId("declaration-reveal")).not.toBeInTheDocument();

      // Resume → back to playing. The stale reveal must NOT reappear.
      act(() => {
        useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "playing" });
      });
      expect(screen.queryByTestId("declaration-reveal")).not.toBeInTheDocument();
    });
  });

  describe("server-gated hand-complete pause", () => {
    const handScore = {
      teamACardPoints: 70,
      teamBCardPoints: 82,
      teamADeclPoints: 0,
      teamBDeclPoints: 0,
      lastTrickTeam: 1,
      lastTrickBonus: 10,
      capot: false,
      capotTeam: null,
      capotBonus: 0,
      failedContract: false,
      contractingTeam: 1,
      teamAHandTotal: 70,
      teamBHandTotal: 92,
    };
    const scorePayload = { ...handScore, teamAMatchScore: 70, teamBMatchScore: 92 };

    it("acknowledges via action:continue and waits (does not dismiss locally)", () => {
      useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "hand_complete" });
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      act(() => {
        useMatchStore.getState().setScoreRevealData(scorePayload);
      });
      act(() => {
        vi.advanceTimersByTime(2000); // Continue-enable delay
      });
      expect(screen.getByTestId("score-reveal")).toBeInTheDocument();

      act(() => {
        fireEvent.click(screen.getByTestId("score-reveal-continue"));
      });
      // Sends the continue action; the dialog stays up in its waiting state —
      // the server (not the client) decides when the next hand is dealt.
      expect(mockSendMessage).toHaveBeenCalledWith("action:continue", {});
      expect(screen.getByTestId("score-reveal")).toBeInTheDocument();
    });

    it("dismisses the score dialog when the server deals the next hand", () => {
      useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "hand_complete" });
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      act(() => {
        useMatchStore.getState().setScoreRevealData(scorePayload);
      });
      act(() => {
        vi.advanceTimersByTime(2000);
      });
      expect(screen.getByTestId("score-reveal")).toBeInTheDocument();

      // Next hand dealt — phase leaves "hand_complete".
      act(() => {
        useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "bidding" });
      });
      expect(screen.queryByTestId("score-reveal")).not.toBeInTheDocument();
    });

    it("reconstructs the score dialog from lastHandResult on reconnect", () => {
      // Reconnect snapshot: phase hand_complete, lastHandResult present, but no
      // event:hand_scored was received (scoreRevealData stays null until derived).
      useMatchStore.getState().setMatchState({
        ...mockMatchState,
        phase: "hand_complete",
        lastHandResult: handScore,
        teamScores: [70, 92],
      });
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      act(() => {
        vi.advanceTimersByTime(2000);
      });
      expect(screen.getByTestId("score-reveal")).toBeInTheDocument();
    });
  });
  // Story 12.5: the prompt's fallback detector is the ONE production site that
  // reads the variant's declaration-overlap rule. The lib tests pass the flag as
  // a literal and the resolver test is isolated, so without a page-level test
  // hardcoding the argument to `false` here would keep the whole suite green.
  describe("declaration prompt overlap wiring", () => {
    // Seat 0 holds quarte 9S-TS-JS-QS (50) and a carré of Jacks (200) sharing
    // JS. Croatian keeps both melds; Bitola dedups down to the carré.
    function overlapPromptState(variant: MatchState["variant"]): MatchState {
      return {
        ...mockMatchState,
        variant,
        phase: "playing",
        trumpSuit: "H",
        activePlayerSeat: 0,
        awaitingDeclaration: true,
        players: mockMatchState.players.map((p) =>
          p.seat === 0
            ? {
                ...p,
                declarations: [],
                hand: [
                  { rank: "9", suit: "S" },
                  { rank: "T", suit: "S" },
                  { rank: "J", suit: "S" },
                  { rank: "Q", suit: "S" },
                  { rank: "J", suit: "H" },
                  { rank: "J", suit: "D" },
                  { rank: "J", suit: "C" },
                  { rank: "7", suit: "C" },
                ] as MatchState["players"][number]["hand"],
              }
            : p,
        ) as MatchState["players"],
      };
    }

    it("lists both overlapping melds and totals 250 in a Croatian match", () => {
      useMatchStore.getState().setMatchState(overlapPromptState("croatia"));
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      const prompt = screen.getByTestId("declaration-prompt");
      // The shared JS is rendered once in each of the two meld rows.
      expect(within(prompt).getAllByTestId("playing-card-JS")).toHaveLength(2);
      expect(within(prompt).getAllByTestId("playing-card-QS")).toHaveLength(1);
      expect(within(prompt).getAllByTestId("playing-card-JC")).toHaveLength(1);
      expect(within(prompt).getByTestId("declaration-prompt-total")).toHaveTextContent("250");
    });

    it("lists one deduped meld and totals 200 in a Bitola match", () => {
      useMatchStore.getState().setMatchState(overlapPromptState("bitola"));
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      const prompt = screen.getByTestId("declaration-prompt");
      // Only the carré survives, so JS appears once and the quarte's QS not at all.
      expect(within(prompt).getAllByTestId("playing-card-JS")).toHaveLength(1);
      expect(within(prompt).queryByTestId("playing-card-QS")).not.toBeInTheDocument();
      expect(within(prompt).getByTestId("declaration-prompt-total")).toHaveTextContent("200");
    });
  });

  // The dedicated declaration phase (Croatian) is a phase the table has never
  // rendered before. The prompt gate is phase-agnostic and the table chrome is
  // phase-independent, so both work here for free — which is exactly why they
  // would regress silently. Everything below is driven by `phase: "declaring"`
  // alone; nothing derives behaviour from the variant.
  describe("dedicated declaration phase", () => {
    // Seat 0 holds a tierce 9S-TS-JS, and the server has it on the clock in the
    // declaring phase with a live turn deadline.
    function declaringState(overrides: Partial<MatchState> = {}): MatchState {
      return {
        // Deliberately NOT a Croatian state: the phase is the only input the
        // client reacts to, and the tierce below is worth 20 under both rule
        // sets. Carrying the variant here would quietly suggest the page
        // branches on it — it does not, and variantRules.ts stays the single
        // variant→behaviour mapping.
        ...mockMatchState,
        phase: "declaring",
        trumpSuit: "H",
        trickNumber: 0,
        activePlayerSeat: 0,
        awaitingDeclaration: true,
        turnExpiresAt: new Date(Date.now() + 30_000).toISOString(),
        timerDurationSec: 30,
        players: mockMatchState.players.map((p) =>
          p.seat === 0
            ? {
                ...p,
                declarations: [],
                hand: [
                  { rank: "9", suit: "S" },
                  { rank: "T", suit: "S" },
                  { rank: "J", suit: "S" },
                ] as MatchState["players"][number]["hand"],
              }
            : p,
        ) as MatchState["players"],
        ...overrides,
      };
    }

    function renderDeclaring(overrides: Partial<MatchState> = {}) {
      useMatchStore.getState().setMatchState(declaringState(overrides));
      useMatchStore.getState().setMyPlayerSeat(0);
      return renderMatchPage();
    }

    it("shows the declaration prompt to the prompted seat", () => {
      renderDeclaring();

      const prompt = screen.getByTestId("declaration-prompt");
      expect(within(prompt).getByTestId("declaration-prompt-total")).toHaveTextContent("20");
      expect(within(prompt).getByTestId("declaration-prompt-declare")).toBeInTheDocument();
      expect(within(prompt).getByTestId("declaration-prompt-skip")).toBeInTheDocument();
    });

    it("shows no prompt to a seat that is not on the clock", () => {
      useMatchStore.getState().setMatchState(declaringState({ activePlayerSeat: 1 }));
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      expect(screen.queryByTestId("declaration-prompt")).not.toBeInTheDocument();
    });

    it("keeps the active seat's turn countdown visible", () => {
      renderDeclaring();

      // Without the phase in the seat-ring predicate the countdown goes dark and
      // the table looks frozen while the server is very much running a clock.
      expect(screen.getByTestId("player-seat-timer-seconds")).toBeInTheDocument();
    });

    it("keeps the pause and surrender controls available", () => {
      renderDeclaring();

      const pause = screen.getByTestId("pause-button");
      expect(pause).toBeInTheDocument();
      expect(pause).toBeEnabled();
      expect(screen.getByTestId("surrender-button")).toBeInTheDocument();
    });

    it("does not make cards playable — no trick is open yet", () => {
      renderDeclaring();

      // Two independent guards hold here: isMyTurn requires the playing phase,
      // AND it requires no declaration to be outstanding. Scoped to the hand —
      // the same card is also drawn inside the prompt's meld preview.
      // Click the CARD, not its positioning wrapper — the click handler lives on
      // playing-card-*, and hand-card-* is the parent it bubbles up to.
      const hand = screen.getByTestId("hand-cards");
      fireEvent.click(within(hand).getByTestId("playing-card-9S"));
      expect(mockSendMessage).not.toHaveBeenCalledWith("action:play_card", expect.anything());
    });

    it("keeps cards unplayable on the phase alone, with no prompt outstanding", () => {
      // The server never emits this shape — the phase resolves the instant
      // nothing is awaited — so this isolates the PHASE clause of isMyTurn from
      // the awaitingDeclaration clause that masks it above. Without it, dropping
      // "playing" from that check would go unnoticed and a stray tap could throw
      // a card before trick 1 exists.
      renderDeclaring({ awaitingDeclaration: false });

      // Click the CARD, not its positioning wrapper — the click handler lives on
      // playing-card-*, and hand-card-* is the parent it bubbles up to.
      const hand = screen.getByTestId("hand-cards");
      fireEvent.click(within(hand).getByTestId("playing-card-9S"));
      expect(mockSendMessage).not.toHaveBeenCalledWith("action:play_card", expect.anything());
    });

    it("sends action:declare when the player declares in the phase", () => {
      renderDeclaring();

      fireEvent.click(
        within(screen.getByTestId("declaration-prompt")).getByTestId("declaration-prompt-declare"),
      );

      expect(mockSendMessage).toHaveBeenCalledWith("action:declare", {});
    });

    it("sends action:skip_declare when the player skips in the phase", () => {
      renderDeclaring();

      fireEvent.click(
        within(screen.getByTestId("declaration-prompt")).getByTestId("declaration-prompt-skip"),
      );

      expect(mockSendMessage).toHaveBeenCalledWith("action:skip_declare", {});
    });
  });

  // Opponent card counts during a deal that holds two cards back per seat. The
  // count is server-authoritative (players[n].faceDownCount) — the page adds it
  // to the open hand and branches on no variant at all, which is why these cases
  // are driven by the field and not by `variant: "croatia"`.
  describe("opponent card counts with face-down cards outstanding", () => {
    function biddingState(faceDownCount: number): MatchState {
      return {
        ...mockMatchState,
        phase: "bidding",
        trumpSuit: null,
        trumpCallerSeat: null,
        trumpCandidate: null,
        biddingRound: 2,
        trickNumber: 0,
        activePlayerSeat: 1,
        players: mockMatchState.players.map((p) => ({
          ...p,
          // Six OPEN cards, as a seat holds mid-bidding under this deal.
          hand: [
            { rank: "7", suit: "S" },
            { rank: "8", suit: "S" },
            { rank: "9", suit: "S" },
            { rank: "T", suit: "S" },
            { rank: "7", suit: "H" },
            { rank: "8", suit: "H" },
          ] as MatchState["players"][number]["hand"],
          faceDownCount,
        })) as MatchState["players"],
      };
    }

    it("reads 8 while each opponent holds six open cards plus two face-down", () => {
      useMatchStore.getState().setMatchState(biddingState(2));
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      // Three opponents (the viewer's own seat renders its real hand instead).
      const counts = screen.getAllByTestId("player-seat-card-count");
      expect(counts).toHaveLength(3);
      for (const c of counts) {
        expect(c).toHaveTextContent("×8");
      }
    });

    // The page-level wiring of mustPickTrump -> TrumpPrompt's canPass. The
    // component's own suite covers the prop; nothing covered the PROP BEING
    // PASSED, and because canPass defaults to true, deleting the attribute here
    // silently restores the rejected-Pass button for the forced dealer.
    it("hides the Pass control from the viewer when the server says they must pick", () => {
      const st = biddingState(2);
      useMatchStore.getState().setMatchState({
        ...st,
        activePlayerSeat: 0,
        mustPickTrump: true,
      });
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      expect(screen.getByTestId("trump-prompt")).toBeInTheDocument();
      expect(screen.queryByTestId("trump-prompt-pass")).not.toBeInTheDocument();
      expect(screen.getByTestId("trump-prompt-must-pick")).toBeInTheDocument();
      // The legal moves are still all there.
      for (const suit of ["S", "H", "D", "C"] as const) {
        expect(screen.getByTestId(`trump-prompt-suit-${suit}`)).toBeEnabled();
      }
    });

    it("offers the Pass control when the server does not force a pick", () => {
      const st = biddingState(2);
      useMatchStore.getState().setMatchState({
        ...st,
        activePlayerSeat: 0,
        mustPickTrump: false,
      });
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      expect(screen.getByTestId("trump-prompt-pass")).toBeInTheDocument();
      expect(screen.queryByTestId("trump-prompt-must-pick")).not.toBeInTheDocument();
    });

    it("does not promise the other seats a pass the dealer cannot make", () => {
      // Viewer is NOT the bidder: the waiting banner must drop "or pass" too,
      // which is the same prop reaching the sibling-seat branch.
      const st = biddingState(2);
      useMatchStore.getState().setMatchState({
        ...st,
        activePlayerSeat: 1,
        mustPickTrump: true,
      });
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      const prompt = screen.getByTestId("trump-prompt");
      expect(prompt).toHaveTextContent(/name a suit/i);
      expect(prompt).not.toHaveTextContent(/or pass/i);
    });

    it("reads the open hand alone when nothing is held face-down", () => {
      // The Bitola path and every post-bidding phase: faceDownCount is 0, so the
      // sum must be indistinguishable from hand.length on its own.
      useMatchStore.getState().setMatchState(biddingState(0));
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      for (const c of screen.getAllByTestId("player-seat-card-count")) {
        expect(c).toHaveTextContent("×6");
      }
    });
  });
});
