import "@/shared/i18n/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import React from "react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { FetchError } from "@/shared/api/axiosClient";
import { resetAudioPreferenceRequestSequence } from "@/shared/lib/audioPreference";
import { formatCoins } from "@/shared/lib/formatCoins";
import { MOTION } from "@/shared/lib/motion";
import { Z } from "@/shared/lib/zLayers";
import { useAuthStore } from "@/shared/stores/authStore";
import { useMatchStore } from "@/shared/stores/matchStore";
import type { HandResult, MatchState, TrickCard } from "@/shared/types/matchTypes";
import type { HandScoredPayload } from "@/shared/types/wsEvents";

import { MatchPage } from "./MatchPage";

const mockSendMessage = vi.fn();
vi.mock("@/shared/providers/WebSocketContext", () => ({
  useWsSendMessage: () => mockSendMessage,
  useWsConnectionState: () => "connected" as const,
}));

vi.mock("@/shared/providers/WebSocketProvider", () => ({
  WebSocketProvider: ({ children }: { children: React.ReactNode }) => children,
}));

const audioEngine = vi.hoisted(() => ({
  playSfx: vi.fn(),
  preloadSfx: vi.fn(() => Promise.resolve()),
  startMusic: vi.fn(),
  stopMusic: vi.fn(),
  setMusicVolume: vi.fn(),
  openAudioSession: vi.fn(() => () => {}),
}));
vi.mock("@/shared/audio/audioEngine", () => audioEngine);

const mockUpdatePreferences = vi.fn((..._args: unknown[]) => Promise.resolve({}));
vi.mock("@/shared/api/profile", () => ({
  updatePreferences: (...args: unknown[]) => mockUpdatePreferences(...args),
}));

const mockGetRoomLastMatch = vi.fn((_roomId: number): Promise<unknown> => new Promise(() => {}));
vi.mock("@/shared/api/matches", () => ({
  getRoomLastMatch: (roomId: number) => mockGetRoomLastMatch(roomId),
}));

vi.mock("@/shared/api/friends", () => ({
  getFriendshipStatus: vi.fn(() => Promise.resolve({ status: "none", requestId: null })),
  sendFriendRequest: vi.fn(),
  acceptFriendRequest: vi.fn(),
  declineFriendRequest: vi.fn(),
  removeFriend: vi.fn(),
}));

vi.mock("@/shared/api/rooms", () => ({
  getRoom: vi.fn(),
  leaveRoom: vi.fn(),
  returnToRoom: vi.fn(),
}));

const toastMock = vi.hoisted(() => ({
  error: vi.fn(),
  info: vi.fn(),
  success: vi.fn(),
  dismiss: vi.fn(),
}));
vi.mock("sonner", () => ({ toast: toastMock }));

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
  declarationsEnabled: true,
  stopAtTarget: false,
  declarationsTransfer: true,
  clockwise: false,
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
      avatarUrl: null,
      team: "teamA",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
      faceDownCount: 0,
      handCount: 1,
      declarationAnswered: false,
    },
    {
      hand: [],
      seat: 1,
      userId: 20,
      username: "Bob",
      avatarUrl: null,
      team: "teamB",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
      faceDownCount: 0,
      handCount: 1,
      declarationAnswered: false,
    },
    {
      hand: [],
      seat: 2,
      userId: 30,
      username: "Carol",
      avatarUrl: null,
      team: "teamA",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
      faceDownCount: 0,
      handCount: 1,
      declarationAnswered: false,
    },
    {
      hand: [],
      seat: 3,
      userId: 40,
      username: "Dave",
      avatarUrl: null,
      team: "teamB",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
      faceDownCount: 0,
      handCount: 1,
      declarationAnswered: false,
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

const matchPageQueryClient = new QueryClient({
  defaultOptions: { queries: { retry: false, gcTime: 0 } },
});

/** MatchPage under the QueryClientProvider the app gives it at the root. */
function MatchPageRoute() {
  return (
    <QueryClientProvider client={matchPageQueryClient}>
      <MatchPage />
    </QueryClientProvider>
  );
}

function createMatchPageRouter({ fromRoom = false } = {}) {
  return createMemoryRouter([{ path: "*", element: <MatchPageRoute /> }], {
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
    Object.values(toastMock).forEach((fn) => fn.mockClear());
    mockGetRoomLastMatch.mockReset();
    mockGetRoomLastMatch.mockReturnValue(new Promise(() => {}));
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
          router={createMemoryRouter([{ path: "/match/:roomId", element: <MatchPageRoute /> }], {
            initialEntries: ["/match/1"],
          })}
        />,
      );

    it("shows the pot (humans × buy-in) once the room buy-in loads", async () => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      mockGetRoom.mockResolvedValue(roomDetail(500) as any);
      useMatchStore.getState().setMatchState(mockMatchState);
      useMatchStore.getState().setMyPlayerSeat(0);

      renderAtRoom1();
      await act(async () => {});

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
        expect(node).toHaveTextContent(formatCoins(1000));
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
    renderMatchPage();

    expect(screen.getByText("Reconnecting to the game…")).toBeInTheDocument();
    expect(screen.queryByText("Match is starting…")).not.toBeInTheDocument();
  });

  it("uses 'Match is starting…' copy when arriving from room lobby", () => {
    renderMatchPage({ fromRoom: true, skipSplash: false });

    expect(screen.getByText("Match is starting…")).toBeInTheDocument();
    expect(screen.queryByText("Reconnecting to the game…")).not.toBeInTheDocument();
  });

  it("holds the splash for GAME_STARTING_SPLASH when arriving from room lobby", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage({ fromRoom: true, skipSplash: false });

    expect(screen.getByTestId("match-starting-splash")).toBeInTheDocument();
    expect(screen.queryByTestId("trick-area")).not.toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(MOTION.GAME_STARTING_SPLASH);
    });

    expect(screen.queryByTestId("match-starting-splash")).not.toBeInTheDocument();
    expect(screen.getByTestId("trick-area")).toBeInTheDocument();
  });

  it("uses GAME_STARTING_SPLASH_REDUCED hold when reduced-motion is set", () => {
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

    act(() => {
      vi.advanceTimersByTime(MOTION.GAME_STARTING_SPLASH_REDUCED - 50);
    });
    expect(screen.getByTestId("match-starting-splash")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(100);
    });
    expect(screen.queryByTestId("match-starting-splash")).not.toBeInTheDocument();
    expect(screen.getByTestId("trick-area")).toBeInTheDocument();
  });

  it("skips the splash hold on reload / reconnect mounts (no fromRoom flag)", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    expect(screen.queryByTestId("match-starting-splash")).not.toBeInTheDocument();
    expect(screen.getByTestId("trick-area")).toBeInTheDocument();
  });

  it("clears the splash timer when unmounted before it elapses", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    const { unmount } = renderMatchPage({ fromRoom: true, skipSplash: false });
    expect(screen.getByTestId("match-starting-splash")).toBeInTheDocument();

    unmount();

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

  it("propagates a 701 matchMode snapshot to the score panel target", () => {
    useMatchStore.getState().setMatchState({ ...mockMatchState, matchMode: "701" });
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    expect(screen.getByTestId("score-a")).toHaveTextContent("/ 701");
    expect(screen.getByTestId("score-b")).toHaveTextContent("/ 701");
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

    act(() => {
      useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "match_end" });
      useMatchStore.getState().setMatchEndData({
        winnerTeam: 0,
        teamAFinalScore: 1020,
        teamBFinalScore: 850,
        matchDurationSec: 300,
      });
    });

    expect(screen.getByTestId("match-result")).toBeInTheDocument();
    expect(screen.getByTestId("match-result-team-a-score")).toHaveTextContent("1020");
  });

  it("shows the Season Points line on the result overlay from the match store", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    act(() => {
      useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "match_end" });
      useMatchStore.getState().setMatchEndData({
        winnerTeam: 0,
        teamAFinalScore: 1020,
        teamBFinalScore: 850,
        matchDurationSec: 300,
      });
    });
    expect(screen.getByTestId("match-result")).toBeInTheDocument();
    expect(screen.queryByTestId("season-sp-line")).not.toBeInTheDocument();

    act(() => {
      useMatchStore.getState().setSeasonSettlement({
        spChange: 24,
        newSeasonSp: 690,
        rankTier: "gold",
        rankDivision: 2,
        reason: "normal",
      });
    });

    expect(screen.getByTestId("season-sp-line")).toHaveTextContent("+24 SP · Gold 2");
  });

  it("shows the Season Points line on the abandonment overlay when it arrives late", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    act(() => {
      useMatchStore.getState().setMatchAbandonedData({
        abandonedByPlayer: 2,
        teamAFinalScore: 450,
        teamBFinalScore: 380,
        matchDurationSec: 600,
      });
    });
    expect(screen.getByTestId("reconnect-overlay")).toBeInTheDocument();
    expect(screen.queryByTestId("season-sp-line")).not.toBeInTheDocument();

    act(() => {
      useMatchStore.getState().setSeasonSettlement({
        spChange: -12,
        newSeasonSp: 488,
        rankTier: "silver",
        rankDivision: 2,
        reason: "partner_abandoned",
      });
    });

    expect(screen.getByTestId("season-sp-line")).toHaveTextContent(
      "Partner abandoned: \u221212 SP (half loss)",
    );
  });

  it("passes the route roomId to MatchResult, which reads that room's last match", async () => {
    mockGetRoom.mockResolvedValue({
      room: { id: 1, status: "playing", coinBuyIn: 0 },
      players: [{ id: 1, roomId: 1, userId: 10, username: "Alice", seat: 0, isBot: false }],
      returnedUserIds: [],
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    mockGetRoomLastMatch.mockResolvedValue({
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
        { seat: 0, userId: 10, username: "Alice", isBot: false },
        { seat: 1, userId: 11, username: "Bob", isBot: false },
        { seat: 2, userId: 12, username: "Carol", isBot: false },
        { seat: 3, userId: 13, username: "Dave", isBot: false },
      ],
      hands: [],
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);

    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    render(
      <RouterProvider
        router={createMemoryRouter([{ path: "/match/:roomId", element: <MatchPageRoute /> }], {
          initialEntries: ["/match/1"],
        })}
      />,
    );
    await act(async () => {});

    act(() => {
      useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "match_end" });
      useMatchStore.getState().setMatchEndData({
        winnerTeam: 0,
        teamAFinalScore: 1020,
        teamBFinalScore: 850,
        matchDurationSec: 300,
      });
    });
    await act(async () => {});

    expect(mockGetRoomLastMatch).toHaveBeenCalledWith(1);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(screen.getByTestId("match-result-stats-toggle")).toBeInTheDocument();
  });

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

    act(() => {
      useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "dealing" });
    });

    expect(screen.queryByTestId("match-result")).not.toBeInTheDocument();
    expect(useMatchStore.getState().matchEndData).toBeNull();
  });

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
            avatarUrl: null,
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
        router={createMemoryRouter([{ path: "/match/:roomId", element: <MatchPageRoute /> }], {
          initialEntries: ["/match/1"],
        })}
      />,
    );

    expect(screen.getByTestId("match-result")).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByTestId("match-result-room-btn"));
    });

    expect(toastMock.error).toHaveBeenCalledWith(
      message,
      expect.objectContaining({ id: "match-error" }),
    );
  });

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
            avatarUrl: null,
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
            { path: "/match/:roomId", element: <MatchPageRoute /> },
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
    expect(toastMock.error).not.toHaveBeenCalled();
    expect(useMatchStore.getState().matchEndData).toBeNull();
  });

  it("raises a closable error toast for the error's duration when lastError is set", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    act(() => {
      useMatchStore.getState().setLastError("error:illegal_play");
    });

    expect(toastMock.error).toHaveBeenCalledWith("That card cannot be played", {
      id: "match-error",
      duration: MOTION.TOAST_ERROR,
      closeButton: true,
    });
  });

  it("replaces the showing error with a newer one under the same toast id", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    act(() => {
      useMatchStore.getState().setLastError("error:illegal_play");
    });
    act(() => {
      useMatchStore.getState().setLastError("error:not_your_turn");
    });

    expect(toastMock.error).toHaveBeenCalledTimes(2);
    expect(toastMock.error).toHaveBeenLastCalledWith(
      "It's not your turn",
      expect.objectContaining({ id: "match-error" }),
    );
  });

  it("gives toasts the felt skin while mounted and clears the match error on leaving", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    const { unmount } = renderMatchPage();
    expect(document.body).toHaveClass("felt-toasts");

    unmount();

    expect(document.body).not.toHaveClass("felt-toasts");
    expect(toastMock.dismiss).toHaveBeenCalledWith("match-error");
  });

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
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    act(() => {
      useMatchStore.getState().setActiveEmote(2, "clap");
    });

    expect(screen.getByTestId("emote-bubble-2")).toBeInTheDocument();
  });

  it("renders the sender's own bubble at South (compass 0)", () => {
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
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    for (const compass of [0, 1, 2, 3] as const) {
      expect(screen.getByTestId(`player-seat-${compass}-wrapper`).style.zIndex).toBe(
        String(Z.SEATS),
      );
    }
  });

  it.each([
    ["settings-button", "settings-dialog"],
    ["rules-button", "rules-dialog"],
  ])("opens %s's dialog above the pause overlay while paused", (button, dialog) => {
    useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "paused" });
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();
    fireEvent.click(screen.getByTestId(button));

    const dialogZ = Number(screen.getByTestId(dialog).style.zIndex);
    const pauseZ = Number(screen.getByTestId("pause-overlay").style.zIndex);
    expect(dialogZ).toBeGreaterThan(pauseZ);
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

      act(() => {
        useMatchStore.getState().setPendingResolvedTrick(collectSnapshot);
        useMatchStore.getState().setScoreRevealData(scorePayload);
      });
      expect(screen.queryByTestId("score-reveal")).not.toBeInTheDocument();

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

      act(() => {
        useMatchStore.getState().setPendingResolvedTrick(collectSnapshot);
        useMatchStore.getState().setBelotReveal({ playerSeat: 0, team: 0, cardId: "QS" });
      });
      expect(screen.getByTestId("belot-reveal")).toBeInTheDocument();

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
    function renderWithHistoryBeneath() {
      const router = createMemoryRouter([{ path: "*", element: <MatchPageRoute /> }], {
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

      const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(false);

      await act(async () => {
        await router.navigate(-1);
      });

      expect(confirmSpy).toHaveBeenCalledWith("Leave the game? You may lose your progress.");
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
      expect(useMatchStore.getState().matchState).toBeNull();
      expect(router.state.location.pathname).toBe("/lobby");

      confirmSpy.mockRestore();
    });

    it("does not block the back button once the match has ended", async () => {
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
              avatarUrl: null,
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
          { path: "/match/:roomId", element: <MatchPageRoute /> },
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

      act(() => {
        useMatchStore.getState().setMatchState({
          ...mockMatchState,
          activePlayerSeat: 2,
          currentTrick: [{ playerSeat: 1, card: { rank: "7", suit: "H" } }],
        });
      });

      expect(
        document.querySelector('[data-testid^="card-flight-throw-opp-1-7H"]'),
      ).toBeInTheDocument();
      expect(screen.queryByTestId("trick-slot-card-1")).not.toBeInTheDocument();

      gbcrSpy.mockRestore();
    });
  });

  describe("belote/rebelote pre-play prompt", () => {
    beforeEach(() => {
      audioEngine.playSfx.mockClear();
    });

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

      expect(screen.getByTestId("belot-prompt")).toBeInTheDocument();
      expect(mockSendMessage).not.toHaveBeenCalledWith("action:play_card", expect.anything());
      expect(audioEngine.playSfx).not.toHaveBeenCalled();
    });

    it("throws the card on confirm, then announces only once the server confirms the deferred play", () => {
      useMatchStore.getState().setMatchState(belotEligibleState());
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      act(() => {
        fireEvent.click(screen.getByTestId("playing-card-KS"));
      });
      expect(audioEngine.playSfx).not.toHaveBeenCalled();
      act(() => {
        fireEvent.click(screen.getByTestId("belot-prompt-announce"));
      });
      expect(audioEngine.playSfx).toHaveBeenCalledTimes(1);
      expect(audioEngine.playSfx).toHaveBeenCalledWith("cardPlay");

      expect(mockSendMessage).toHaveBeenCalledWith("action:play_card", { cardId: "KS" });
      expect(mockSendMessage).not.toHaveBeenCalledWith("action:announce_belot", {});

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
      expect(audioEngine.playSfx).not.toHaveBeenCalled();
      act(() => {
        fireEvent.click(screen.getByTestId("belot-prompt-decline"));
      });
      expect(audioEngine.playSfx).toHaveBeenCalledTimes(1);
      expect(audioEngine.playSfx).toHaveBeenCalledWith("cardPlay");

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

      act(() => {
        useMatchStore.getState().setPendingResolvedTrick(aSnapshot);
        useMatchStore.getState().setDeclarationReveal(decls);
      });
      expect(screen.queryByTestId("declaration-reveal")).not.toBeInTheDocument();

      act(() => {
        useMatchStore.getState().setPendingResolvedTrick(null);
      });
      expect(screen.getByTestId("declaration-reveal")).toBeInTheDocument();

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

      act(() => {
        useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "paused" });
      });
      expect(screen.queryByTestId("declaration-reveal")).not.toBeInTheDocument();

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
        vi.advanceTimersByTime(2000);
      });
      expect(screen.getByTestId("score-reveal")).toBeInTheDocument();

      act(() => {
        fireEvent.click(screen.getByTestId("score-reveal-continue"));
      });
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

      act(() => {
        useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "bidding" });
      });
      expect(screen.queryByTestId("score-reveal")).not.toBeInTheDocument();
    });

    it("reconstructs the score dialog from lastHandResult on reconnect", () => {
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

    it("names only the card points on a fall when the room's declarations do not transfer", () => {
      useMatchStore.getState().setMatchState({
        ...mockMatchState,
        phase: "hand_complete",
        declarationsTransfer: false,
        lastHandResult: {
          ...handScore,
          failedContract: true,
          contractingTeam: 1,
          teamAHandTotal: 162,
          teamBHandTotal: 0,
        },
        teamScores: [162, 0],
      });
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      act(() => {
        vi.advanceTimersByTime(2000);
      });
      expect(screen.getByTestId("score-reveal")).toHaveTextContent("Went down · card points to us");
    });
  });
  describe("declaration prompt overlap wiring", () => {
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
      expect(within(prompt).getAllByTestId("playing-card-JS")).toHaveLength(2);
      expect(within(prompt).getAllByTestId("playing-card-QS")).toHaveLength(1);
      expect(within(prompt).getAllByTestId("playing-card-JC")).toHaveLength(1);
      expect(within(prompt).getByTestId("declaration-prompt-total")).toHaveTextContent("250");
    });

    it("shows no prompt to a Bitola seat that is not on the clock", () => {
      useMatchStore.getState().setMatchState({
        ...overlapPromptState("bitola"),
        trickNumber: 1,
        activePlayerSeat: 1,
      });
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      expect(screen.queryByTestId("declaration-prompt")).not.toBeInTheDocument();
    });

    it("lists one deduped meld and totals 200 in a Bitola match", () => {
      useMatchStore.getState().setMatchState(overlapPromptState("bitola"));
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      const prompt = screen.getByTestId("declaration-prompt");
      expect(within(prompt).getAllByTestId("playing-card-JS")).toHaveLength(1);
      expect(within(prompt).queryByTestId("playing-card-QS")).not.toBeInTheDocument();
      expect(within(prompt).getByTestId("declaration-prompt-total")).toHaveTextContent("200");
    });
  });

  describe("rules overlay", () => {
    function openRules(variant: MatchState["variant"]) {
      useMatchStore.getState().setMatchState({ ...mockMatchState, variant });
      renderMatchPage();
      fireEvent.click(screen.getByTestId("rules-button"));
    }

    it("opens the rules overlay on the Croatian tab in a Croatian match", () => {
      openRules("croatia");
      expect(screen.getByTestId("rules-dialog")).toBeInTheDocument();
      expect(screen.getByTestId("rules-dialog-variant-croatia")).toHaveAttribute(
        "aria-selected",
        "true",
      );
      expect(screen.getByText("Deal all eight up front")).toBeInTheDocument();
      expect(screen.queryByText("Deal five each, then turn one up")).not.toBeInTheDocument();
    });

    it("opens the rules overlay on the Bitola tab in a Bitola match", () => {
      openRules("bitola");
      expect(screen.getByTestId("rules-dialog-variant-bitola")).toHaveAttribute(
        "aria-selected",
        "true",
      );
      expect(screen.getByText("Deal five each, then turn one up")).toBeInTheDocument();
      expect(screen.queryByText("Deal all eight up front")).not.toBeInTheDocument();
    });

    it("notes the voided declarations on a table whose declarations do not transfer", () => {
      useMatchStore.getState().setMatchState({ ...mockMatchState, declarationsTransfer: false });
      renderMatchPage();
      fireEvent.click(screen.getByTestId("rules-button"));

      expect(screen.getByTestId("rules-dialog-no-declarations-transfer-note")).toBeInTheDocument();
    });

    it("notes the leftward play on a clockwise table", () => {
      useMatchStore.getState().setMatchState({ ...mockMatchState, clockwise: true });
      renderMatchPage();
      fireEvent.click(screen.getByTestId("rules-button"));

      expect(screen.getByTestId("rules-dialog-clockwise-note")).toBeInTheDocument();
    });
  });

  describe("audio", () => {
    const collectSnapshot: { trick: TrickCard[]; winnerSeat: number } = {
      trick: [
        { card: { rank: "K", suit: "S" }, playerSeat: 0 },
        { card: { rank: "7", suit: "H" }, playerSeat: 1 },
        { card: { rank: "A", suit: "D" }, playerSeat: 2 },
        { card: { rank: "9", suit: "C" }, playerSeat: 3 },
      ],
      winnerSeat: 2,
    };

    function audioPrefs() {
      const u = useAuthStore.getState().user;
      return { sound: u?.soundEnabled, music: u?.musicEnabled };
    }

    function collectCalls() {
      return audioEngine.playSfx.mock.calls.filter(([name]) => name === "trickCollect");
    }

    beforeEach(() => {
      Object.values(audioEngine).forEach((fn) => fn.mockClear());
      mockUpdatePreferences.mockClear();
      resetAudioPreferenceRequestSequence();
      useMatchStore.getState().setMatchState(mockMatchState);
      useMatchStore.getState().setMyPlayerSeat(0);
    });

    it("opens the audio session with music on entry, and stops the music on leaving", () => {
      const { unmount } = renderMatchPage();

      expect(audioEngine.openAudioSession).toHaveBeenCalledTimes(1);
      expect(audioEngine.startMusic).toHaveBeenCalledTimes(1);
      expect(audioEngine.stopMusic).not.toHaveBeenCalled();

      unmount();
      expect(audioEngine.stopMusic).toHaveBeenCalledTimes(1);
    });

    it("mutes BOTH switches when both are on, then un-mutes both", () => {
      renderMatchPage();
      const mute = screen.getByTestId("mute-button");
      expect(mute).toHaveAccessibleName("Mute");

      fireEvent.click(mute);

      expect(audioPrefs()).toEqual({ sound: false, music: false });
      expect(mockUpdatePreferences).toHaveBeenCalledTimes(1);
      expect(mockUpdatePreferences).toHaveBeenCalledWith(10, {
        soundEnabled: false,
        musicEnabled: false,
      });
      expect(audioEngine.stopMusic).toHaveBeenCalledTimes(1);
      expect(screen.getByTestId("mute-button")).toHaveAccessibleName("Unmute");

      fireEvent.click(screen.getByTestId("mute-button"));

      expect(audioPrefs()).toEqual({ sound: true, music: true });
      expect(mockUpdatePreferences).toHaveBeenLastCalledWith(10, {
        soundEnabled: true,
        musicEnabled: true,
      });
      expect(audioEngine.startMusic).toHaveBeenCalledTimes(2);
      expect(screen.getByTestId("mute-button")).toHaveAccessibleName("Mute");
    });

    it("brings the chosen volumes back after a mute and unmute", () => {
      useAuthStore.setState({
        user: makeUser({ id: 10, soundVolume: 30, musicVolume: 80 }),
      });
      renderMatchPage();
      expect(audioEngine.setMusicVolume).toHaveBeenLastCalledWith(80);

      fireEvent.click(screen.getByTestId("mute-button"));
      fireEvent.click(screen.getByTestId("mute-button"));

      const u = useAuthStore.getState().user;
      expect({ sound: u?.soundVolume, music: u?.musicVolume }).toEqual({ sound: 30, music: 80 });
      expect(audioEngine.setMusicVolume).toHaveBeenLastCalledWith(80);
      expect(audioEngine.startMusic).toHaveBeenCalledTimes(2);
      for (const [, body] of mockUpdatePreferences.mock.calls) {
        expect(body).not.toHaveProperty("soundVolume");
        expect(body).not.toHaveProperty("musicVolume");
      }
    });

    it("treats ANY switch on as unmuted — one press turns both off", () => {
      useAuthStore.setState({
        user: makeUser({ id: 10, soundEnabled: false, musicEnabled: true }),
      });
      renderMatchPage();
      expect(screen.getByTestId("mute-button")).toHaveAccessibleName("Mute");

      fireEvent.click(screen.getByTestId("mute-button"));

      expect(audioPrefs()).toEqual({ sound: false, music: false });
      expect(mockUpdatePreferences).toHaveBeenCalledWith(10, { musicEnabled: false });
    });

    it("offers the same mute in the phone HUD menu", () => {
      renderMatchPage();
      fireEvent.click(screen.getByTestId("hud-menu-button"));

      const item = screen.getByTestId("hud-menu-mute");
      expect(item).toHaveTextContent("Mute");
      fireEvent.click(item);

      expect(audioPrefs()).toEqual({ sound: false, music: false });
      expect(screen.queryByTestId("hud-menu")).not.toBeInTheDocument();

      fireEvent.click(screen.getByTestId("hud-menu-button"));
      expect(screen.getByTestId("hud-menu-mute")).toHaveTextContent("Unmute");
    });

    it("sounds the player's own card at the click, before the server echo", () => {
      renderMatchPage();

      act(() => {
        fireEvent.click(within(screen.getByTestId("hand-cards")).getByTestId("playing-card-KS"));
      });

      expect(mockSendMessage).toHaveBeenCalledWith("action:play_card", { cardId: "KS" });
      expect(audioEngine.playSfx).toHaveBeenCalledTimes(1);
      expect(audioEngine.playSfx).toHaveBeenCalledWith("cardPlay");
    });

    it("sounds the trick collect once, as the cards leave the table", () => {
      renderMatchPage();

      act(() => {
        useMatchStore.getState().setPendingResolvedTrick(collectSnapshot);
      });
      const receivedAt = useMatchStore.getState().pendingResolvedTrick!.receivedAt;
      expect(collectCalls()).toHaveLength(0);

      act(() => {
        vi.advanceTimersByTime(MOTION.TRICK_RESOLVE_PAUSE);
      });
      expect(collectCalls()).toEqual([["trickCollect", { dedupeKey: String(receivedAt) }]]);
    });

    it("re-asks with the SAME dedupe key when a remount re-runs the collect for a live snapshot", () => {
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

      try {
        const first = renderMatchPage();
        act(() => {
          useMatchStore.getState().setPendingResolvedTrick(collectSnapshot);
        });
        act(() => {
          vi.advanceTimersByTime(MOTION.TRICK_RESOLVE_PAUSE);
        });
        const snapshot = useMatchStore.getState().pendingResolvedTrick;
        expect(snapshot).not.toBeNull();
        first.unmount();

        renderMatchPage();
        act(() => {
          vi.advanceTimersByTime(MOTION.TRICK_RESOLVE_PAUSE);
        });

        const calls = collectCalls();
        expect(calls).toHaveLength(2);
        expect(calls[0]![1]).toEqual({ dedupeKey: String(snapshot!.receivedAt) });
        expect(calls[1]![1]).toEqual(calls[0]![1]);
      } finally {
        gbcrSpy.mockRestore();
      }
    });

    it("sounds the collect under reduced motion too", () => {
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
      renderMatchPage();

      act(() => {
        useMatchStore.getState().setPendingResolvedTrick(collectSnapshot);
      });
      expect(collectCalls()).toHaveLength(0);

      act(() => {
        vi.advanceTimersByTime(MOTION.TRICK_RESOLVE_PAUSE);
      });
      expect(collectCalls()).toHaveLength(1);
      expect(useMatchStore.getState().pendingResolvedTrick).toBeNull();
    });

    it("plays nothing for a resync snapshot", () => {
      renderMatchPage();

      act(() => {
        useMatchStore.getState().setMatchState({
          ...mockMatchState,
          activePlayerSeat: 3,
          currentTrick: [
            { playerSeat: 1, card: { rank: "7", suit: "H" } },
            { playerSeat: 2, card: { rank: "A", suit: "D" } },
          ],
        });
      });

      expect(audioEngine.playSfx).not.toHaveBeenCalled();
    });
  });

  describe("dedicated declaration phase", () => {
    function declaringState(overrides: Partial<MatchState> = {}): MatchState {
      return {
        ...mockMatchState,
        phase: "declaring",
        trumpSuit: "H",
        trickNumber: 0,
        activePlayerSeat: 1,
        awaitingDeclaration: false,
        turnExpiresAt: null,
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

    function meldlessOverride(): Partial<MatchState> {
      return {
        players: mockMatchState.players.map((p) =>
          p.seat === 0
            ? {
                ...p,
                declarations: [],
                hand: [
                  { rank: "9", suit: "S" },
                  { rank: "Q", suit: "H" },
                  { rank: "7", suit: "D" },
                ] as MatchState["players"][number]["hand"],
              }
            : p,
        ) as MatchState["players"],
      };
    }

    it("shows the declaration prompt to a seat holding a meld", () => {
      renderDeclaring();

      const prompt = screen.getByTestId("declaration-prompt");
      expect(within(prompt).getByTestId("declaration-prompt-total")).toHaveTextContent("20");
      expect(within(prompt).getByTestId("declaration-prompt-declare")).toBeEnabled();
      expect(within(prompt).getByTestId("declaration-prompt-skip")).toBeEnabled();
    });

    it("answers at once for a seat holding nothing and shows it waiting", () => {
      renderDeclaring(meldlessOverride());

      expect(mockSendMessage).toHaveBeenCalledWith("action:skip_declare", {});
      expect(mockSendMessage).toHaveBeenCalledTimes(1);
      const prompt = screen.getByTestId("declaration-prompt");
      expect(within(prompt).getByText("No declarations")).toBeInTheDocument();
      expect(within(prompt).getByTestId("declaration-prompt-none")).toBeInTheDocument();
      expect(within(prompt).queryByTestId("declaration-prompt-total")).not.toBeInTheDocument();
      expect(within(prompt).getByTestId("declaration-prompt-declare")).toBeDisabled();
      const skip = within(prompt).getByTestId("declaration-prompt-skip");
      expect(skip).toBeDisabled();
      expect(skip).toHaveTextContent("1/4");

      act(() => {
        vi.advanceTimersByTime(MOTION.DECLARATION_PHASE_AUTO_SKIP + 100);
      });
      expect(mockSendMessage).toHaveBeenCalledTimes(1);
    });

    it("shows the prompt to a seat that is not the pinned activePlayerSeat", () => {
      renderDeclaring();

      expect(screen.getByTestId("declaration-prompt")).toBeInTheDocument();
    });

    it("holds the dialog open in a waiting state once the viewer has answered", () => {
      renderDeclaring({
        players: declaringState().players.map((p) =>
          p.seat === 0 ? { ...p, declarationAnswered: true } : p,
        ) as MatchState["players"],
      });

      const prompt = screen.getByTestId("declaration-prompt");
      expect(prompt).toBeInTheDocument();
      expect(within(prompt).getByTestId("declaration-prompt-declare")).toBeDisabled();
      const skip = within(prompt).getByTestId("declaration-prompt-skip");
      expect(skip).toBeDisabled();
      expect(skip).toHaveTextContent("Waiting for the others");
    });

    it("reports how many seats have answered", () => {
      renderDeclaring({
        players: declaringState().players.map((p) =>
          p.seat === 0 || p.seat === 2 ? { ...p, declarationAnswered: true } : p,
        ) as MatchState["players"],
      });

      expect(
        within(screen.getByTestId("declaration-prompt")).getByTestId("declaration-prompt-skip"),
      ).toHaveTextContent("2/4");
    });

    it("auto-skips once the phase window elapses", () => {
      renderDeclaring();

      expect(mockSendMessage).not.toHaveBeenCalledWith("action:skip_declare", expect.anything());
      act(() => {
        vi.advanceTimersByTime(MOTION.DECLARATION_PHASE_AUTO_SKIP + 100);
      });
      expect(mockSendMessage).toHaveBeenCalledWith("action:skip_declare", {});
    });

    it("does not auto-skip again once the viewer has answered", () => {
      renderDeclaring({
        players: declaringState().players.map((p) =>
          p.seat === 0 ? { ...p, declarationAnswered: true } : p,
        ) as MatchState["players"],
      });

      act(() => {
        vi.advanceTimersByTime(MOTION.DECLARATION_PHASE_AUTO_SKIP + 100);
      });
      expect(mockSendMessage).not.toHaveBeenCalledWith("action:skip_declare", expect.anything());
    });

    it("flags no seat as active and shows no turn countdown", () => {
      renderDeclaring();

      expect(screen.queryByTestId("player-seat-timer-seconds")).not.toBeInTheDocument();
      for (const compass of [0, 1, 2, 3]) {
        expect(screen.getByTestId(`player-seat-${compass}`)).toHaveAttribute(
          "data-active",
          "false",
        );
      }
    });

    it("withdraws the pause control — the phase is a fixed window, not a turn", () => {
      renderDeclaring();

      expect(screen.queryByTestId("pause-button")).not.toBeInTheDocument();
      expect(screen.getByTestId("surrender-button")).toBeInTheDocument();
    });

    it("withdraws the pause control from the phone HUD menu too", () => {
      renderDeclaring();
      fireEvent.click(screen.getByTestId("hud-menu-button"));

      expect(screen.getByTestId("hud-menu")).toBeInTheDocument();
      expect(screen.queryByTestId("hud-menu-pause")).not.toBeInTheDocument();
    });

    it("does not make cards playable — no trick is open yet", () => {
      renderDeclaring();

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

  describe("opponent card counts with face-down cards outstanding", () => {
    function biddingState(faceDownCount: number): MatchState {
      return {
        ...mockMatchState,
        phase: "bidding",
        trumpSuit: null,
        trumpCallerSeat: null,
        trumpCandidate: null,
        biddingRound: 1,
        trickNumber: 0,
        activePlayerSeat: 1,
        players: mockMatchState.players.map((p) => ({
          ...p,
          hand:
            p.seat === 0
              ? ([
                  { rank: "7", suit: "S" },
                  { rank: "8", suit: "S" },
                  { rank: "9", suit: "S" },
                  { rank: "T", suit: "S" },
                  { rank: "7", suit: "H" },
                  { rank: "8", suit: "H" },
                ] as MatchState["players"][number]["hand"])
              : [],
          handCount: 6,
          declarationAnswered: false,
          faceDownCount,
        })) as MatchState["players"],
      };
    }

    it("reads 8 while each opponent holds six open cards plus two face-down", () => {
      useMatchStore.getState().setMatchState(biddingState(2));
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      const counts = screen.getAllByTestId("player-seat-card-count");
      expect(counts).toHaveLength(3);
      for (const c of counts) {
        expect(c).toHaveTextContent("×8");
      }
    });

    it("renders the viewer's own two face-down backs for the whole of bidding", () => {
      useMatchStore.getState().setMatchState(biddingState(2));
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      expect(screen.getAllByTestId("hand-card-face-down")).toHaveLength(2);
    });

    it("renders no backs once trump resolves and the snapshot carries all 8", () => {
      const st = biddingState(0);
      useMatchStore.getState().setMatchState({
        ...st,
        phase: "playing",
        trumpSuit: "S",
        trumpCallerSeat: 0,
        trickNumber: 1,
        players: st.players.map((p) => ({
          ...p,
          hand:
            p.seat === 0
              ? ([
                  ...p.hand,
                  { rank: "J", suit: "S" },
                  { rank: "Q", suit: "S" },
                ] as MatchState["players"][number]["hand"])
              : [],
          handCount: 8,
          declarationAnswered: false,
          faceDownCount: 0,
        })) as MatchState["players"],
      });
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      expect(screen.queryAllByTestId("hand-card-face-down")).toHaveLength(0);
      expect(screen.getByTestId("hand-card-JS")).toBeInTheDocument();
      expect(screen.getByTestId("hand-card-QS")).toBeInTheDocument();
    });

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
      const st = biddingState(2);
      useMatchStore.getState().setMatchState({
        ...st,
        activePlayerSeat: 1,
        mustPickTrump: true,
      });
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      const prompt = screen.getByTestId("trump-prompt");
      expect(prompt).toHaveTextContent(/choose a suit/i);
      expect(prompt).not.toHaveTextContent(/or pass/i);
    });

    it("keeps room rules out of the scoreboard header", () => {
      useMatchStore.getState().setMatchState({
        ...biddingState(0),
        declarationsEnabled: false,
        stopAtTarget: true,
        declarationsTransfer: false,
        clockwise: true,
      });
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      const meta = screen.getByTestId("score-meta");
      expect(meta).not.toHaveTextContent(/declarations|target|transfer|clockwise/i);
      expect(meta.textContent?.split(" · ")).toHaveLength(2);
    });

    it("reads the open hand alone when nothing is held face-down", () => {
      useMatchStore.getState().setMatchState(biddingState(0));
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      for (const c of screen.getAllByTestId("player-seat-card-count")) {
        expect(c).toHaveTextContent("×6");
      }
    });

    it("renders the full masked wire shape — empty foreign declarations and null pendingBelotSeat included", () => {
      const masked: MatchState = {
        ...mockMatchState,
        declarationsResolved: false,
        pendingBelotSeat: null,
        players: mockMatchState.players.map((p): MatchState["players"][number] =>
          p.seat === 0 ? { ...p, declarations: [] } : { ...p, hand: [], declarations: [] },
        ) as MatchState["players"],
      };
      useMatchStore.getState().setMatchState(masked);
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      for (const compass of [0, 1, 2, 3]) {
        expect(screen.getByTestId(`player-seat-${compass}-wrapper`)).toBeInTheDocument();
      }
      const counts = screen.getAllByTestId("player-seat-card-count");
      expect(counts).toHaveLength(3);
      for (const c of counts) {
        expect(c).toHaveTextContent("×1");
      }
      expect(screen.getByTestId("playing-card-KS")).toBeInTheDocument();
      expect(screen.queryByTestId("belot-prompt")).not.toBeInTheDocument();
    });

    it("falls back to hand.length for a server that predates handCount", () => {
      const legacy = biddingState(0);
      useMatchStore.getState().setMatchState({
        ...legacy,
        players: legacy.players.map((p) => {
          const rest: Partial<MatchState["players"][number]> = { ...p };
          delete rest.handCount;
          return {
            ...rest,
            hand: [
              { rank: "7", suit: "D" },
              { rank: "8", suit: "D" },
              { rank: "9", suit: "D" },
              { rank: "T", suit: "D" },
              { rank: "J", suit: "D" },
              { rank: "Q", suit: "D" },
            ],
          };
        }) as unknown as MatchState["players"],
      });
      useMatchStore.getState().setMyPlayerSeat(0);
      renderMatchPage();

      for (const c of screen.getAllByTestId("player-seat-card-count")) {
        expect(c).toHaveTextContent("×6");
      }
    });
  });
  describe("match polish: deal, collect sequencing, event sounds, belote", () => {
    const sfxCalls = (name: string) =>
      audioEngine.playSfx.mock.calls.filter(([called]) => called === name);

    beforeEach(() => {
      audioEngine.playSfx.mockClear();
    });

    function seatCount(seat: number): string {
      return within(screen.getByTestId(`player-seat-${seat}`)).getByTestId("player-seat-card-count")
        .textContent!;
    }

    function handCardIds(): string[] {
      return Array.from(
        within(screen.getByTestId("hand-cards")).queryAllByTestId(/^hand-card-(?!face-down)/),
      ).map((el) => el.getAttribute("data-testid")!.slice("hand-card-".length));
    }

    const FIVE: MatchState["players"][number]["hand"] = [
      { rank: "7", suit: "S" },
      { rank: "8", suit: "S" },
      { rank: "9", suit: "S" },
      { rank: "T", suit: "H" },
      { rank: "J", suit: "H" },
    ];

    function dealtState(overrides: Partial<MatchState> = {}): MatchState {
      return {
        ...mockMatchState,
        phase: "bidding",
        handNumber: 2,
        dealerSeat: 1,
        activePlayerSeat: 2,
        trumpSuit: null,
        trumpCallerSeat: null,
        trumpCandidate: { rank: "A", suit: "H" },
        biddingRound: 1,
        biddingPassCount: 0,
        trickNumber: 0,
        players: mockMatchState.players.map((p) => ({
          ...p,
          hand: p.seat === 0 ? FIVE : [],
          handCount: 5,
        })) as MatchState["players"],
        ...overrides,
      };
    }

    describe("deal animation", () => {
      let gbcrSpy: ReturnType<typeof vi.spyOn>;
      beforeEach(() => {
        gbcrSpy = vi.spyOn(Element.prototype, "getBoundingClientRect").mockReturnValue({
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
      });
      afterEach(() => {
        gbcrSpy.mockRestore();
      });

      const dealFlights = () =>
        document.querySelectorAll('[data-testid^="card-flight-deal-"][data-testid$="-animated"]');

      it("deals the next hand packet by packet before the bidding prompt", () => {
        useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "hand_complete" });
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();

        act(() => {
          useMatchStore.getState().setMatchState(dealtState());
        });

        expect(dealFlights()).toHaveLength(20);
        expect(screen.getByTestId("deal-animation")).toBeInTheDocument();
        expect(screen.queryByTestId("trump-prompt")).not.toBeInTheDocument();
        expect(seatCount(2)).toBe("×0");
        expect(handCardIds()).toEqual([]);

        act(() => {
          vi.advanceTimersByTime(MOTION.DEAL_LEAD_IN + MOTION.DEAL_PACKET_FLIGHT);
        });
        expect(sfxCalls("deal")).toHaveLength(1);
        expect(sfxCalls("dealPacket")).toHaveLength(1);
        expect(seatCount(2)).toBe("×3");
        expect(seatCount(3)).toBe("×0");

        act(() => {
          vi.advanceTimersByTime(2 * MOTION.DEAL_PACKET_STAGGER);
        });
        expect(handCardIds()).toHaveLength(3);
        expect(screen.queryByTestId("trump-prompt")).not.toBeInTheDocument();

        act(() => {
          vi.advanceTimersByTime(MOTION.DEAL_DURATION_CANDIDATE_FIRST);
        });
        expect(sfxCalls("dealPacket")).toHaveLength(8);
        expect(screen.queryByTestId("deal-animation")).not.toBeInTheDocument();
        expect(dealFlights()).toHaveLength(0);
        expect(screen.getByTestId("trump-prompt")).toBeInTheDocument();
        expect(seatCount(2)).toBe("×5");
        expect(handCardIds()).toHaveLength(5);
      });

      it("deals a clockwise hand from the seat on the dealer's other side", () => {
        useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "hand_complete" });
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();

        act(() => {
          useMatchStore
            .getState()
            .setMatchState(dealtState({ clockwise: true, dealerSeat: 1, activePlayerSeat: 0 }));
        });

        const packetLand = (i: number) =>
          MOTION.DEAL_LEAD_IN + i * MOTION.DEAL_PACKET_STAGGER + MOTION.DEAL_PACKET_FLIGHT;
        act(() => {
          vi.advanceTimersByTime(packetLand(0));
        });
        expect(handCardIds()).toHaveLength(3);
        expect(seatCount(3)).toBe("×0");
        expect(seatCount(2)).toBe("×0");

        act(() => {
          vi.advanceTimersByTime(packetLand(1) - packetLand(0));
        });
        expect(seatCount(3)).toBe("×3");
        expect(seatCount(2)).toBe("×0");

        act(() => {
          vi.advanceTimersByTime(packetLand(2) - packetLand(1));
        });
        expect(seatCount(2)).toBe("×3");
        expect(seatCount(1)).toBe("×0");
      });

      it("turns the candidate face-up at the end of the deal", () => {
        useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "hand_complete" });
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();
        act(() => {
          useMatchStore.getState().setMatchState(dealtState());
        });

        expect(screen.queryByTestId("deal-candidate")).not.toBeInTheDocument();
        act(() => {
          vi.advanceTimersByTime(MOTION.DEAL_DURATION_CANDIDATE_FIRST - MOTION.DEAL_CANDIDATE_FLIP);
        });
        expect(
          within(screen.getByTestId("deal-candidate")).getByTestId("playing-card-AH"),
        ).toBeInTheDocument();
      });

      it("holds the first bidder's seat ring until the deal is down", () => {
        useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "hand_complete" });
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();
        act(() => {
          useMatchStore.getState().setMatchState(
            dealtState({
              turnExpiresAt: new Date(Date.now() + 12_100).toISOString(),
              timerDurationSec: 10,
            }),
          );
        });

        expect(screen.getByTestId("player-seat-2")).toHaveAttribute("data-active", "false");
        expect(screen.queryByTestId("timer-ring")).not.toBeInTheDocument();

        act(() => {
          vi.advanceTimersByTime(MOTION.DEAL_DURATION_CANDIDATE_FIRST);
        });
        expect(screen.getByTestId("player-seat-2")).toHaveAttribute("data-active", "true");
        expect(screen.getByTestId("timer-ring")).toBeInTheDocument();
      });

      it("animates a reshuffle (same hand, rotated dealer) and says so", () => {
        useMatchStore
          .getState()
          .setMatchState(dealtState({ biddingRound: 2, biddingPassCount: 3, activePlayerSeat: 1 }));
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();
        expect(screen.queryByTestId("deal-animation")).not.toBeInTheDocument();

        act(() => {
          useMatchStore
            .getState()
            .setMatchState(dealtState({ dealerSeat: 2, activePlayerSeat: 3 }));
        });

        expect(screen.getByTestId("deal-reshuffle-caption")).toHaveTextContent("Reshuffling deck…");
        expect(screen.queryByTestId("trump-prompt")).not.toBeInTheDocument();
        act(() => {
          vi.advanceTimersByTime(MOTION.DEAL_DURATION_CANDIDATE_FIRST);
        });
        expect(screen.getByTestId("trump-prompt")).toBeInTheDocument();
      });

      it("deals 3, 3 and 2 face-down in a candidate-less (all-before-bidding) hand", () => {
        const SIX = [...FIVE, { rank: "Q" as const, suit: "H" as const }];
        useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "hand_complete" });
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();
        act(() => {
          useMatchStore.getState().setMatchState(
            dealtState({
              variant: "croatia",
              trumpCandidate: null,
              dealerSeat: 3,
              activePlayerSeat: 0,
              players: dealtState().players.map((p) => ({
                ...p,
                hand: p.seat === 0 ? SIX : [],
                handCount: 6,
                faceDownCount: 2,
              })) as MatchState["players"],
            }),
          );
        });

        const packetLand = (i: number) =>
          MOTION.DEAL_LEAD_IN + i * MOTION.DEAL_PACKET_STAGGER + MOTION.DEAL_PACKET_FLIGHT;
        act(() => {
          vi.advanceTimersByTime(packetLand(0));
        });
        expect(handCardIds()).toHaveLength(3);
        expect(screen.queryAllByTestId("hand-card-face-down")).toHaveLength(0);
        act(() => {
          vi.advanceTimersByTime(packetLand(4) - packetLand(0));
        });
        expect(handCardIds()).toHaveLength(6);
        expect(screen.queryAllByTestId("hand-card-face-down")).toHaveLength(0);
        act(() => {
          vi.advanceTimersByTime(packetLand(8) - packetLand(4));
        });
        expect(screen.queryAllByTestId("hand-card-face-down")).toHaveLength(2);
        expect(screen.queryByTestId("trump-prompt")).not.toBeInTheDocument();

        act(() => {
          vi.advanceTimersByTime(MOTION.DEAL_DURATION_ALL_BEFORE_BIDDING - packetLand(8));
        });
        expect(sfxCalls("dealPacket")).toHaveLength(12);
        expect(seatCount(1)).toBe("×8");
        expect(screen.getByTestId("trump-prompt")).toBeInTheDocument();
      });

      it("animates the second deal and holds the leader's cards until it lands", () => {
        const bidding = dealtState({ dealerSeat: 3, activePlayerSeat: 1, handNumber: 1 });
        useMatchStore.getState().setMatchState(bidding);
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();

        const EIGHT = [
          ...FIVE,
          { rank: "Q" as const, suit: "D" as const },
          { rank: "K" as const, suit: "D" as const },
          { rank: "A" as const, suit: "D" as const },
        ];
        act(() => {
          useMatchStore.getState().setMatchState({
            ...bidding,
            phase: "playing",
            trumpSuit: "H",
            trumpCallerSeat: 1,
            trumpCandidate: null,
            activePlayerSeat: 0,
            trickNumber: 1,
            currentTrick: [],
            players: bidding.players.map((p) => ({
              ...p,
              hand: p.seat === 0 ? EIGHT : [],
              handCount: 8,
            })) as MatchState["players"],
          });
        });

        expect(
          document.querySelector('[data-testid^="card-flight-deal-"][data-testid$="-candidate"]'),
        ).not.toBeNull();
        expect(handCardIds()).toHaveLength(5);
        expect(seatCount(1)).toBe("×5");
        expect(sfxCalls("deal")).toHaveLength(0);

        act(() => {
          vi.advanceTimersByTime(MOTION.DEAL_PACKET_FLIGHT);
        });
        expect(handCardIds()).toHaveLength(8);
        expect(screen.getByTestId("playing-card-7S")).toHaveAttribute("tabindex", "-1");

        act(() => {
          vi.advanceTimersByTime(MOTION.DEAL_DURATION_CANDIDATE_SECOND);
        });
        expect(seatCount(1)).toBe("×8");
        expect(sfxCalls("dealPacket")).toHaveLength(4);
        expect(screen.getByTestId("playing-card-7S")).toHaveAttribute("tabindex", "0");
      });

      it("never replays a deal when mounting into a hand in progress (reconnect mid-bidding)", () => {
        useMatchStore.getState().setMatchState(dealtState());
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();

        expect(screen.queryByTestId("deal-animation")).not.toBeInTheDocument();
        expect(screen.getByTestId("trump-prompt")).toBeInTheDocument();
        act(() => {
          vi.advanceTimersByTime(MOTION.DEAL_DURATION_CANDIDATE_FIRST);
        });
        expect(audioEngine.playSfx).not.toHaveBeenCalled();
      });

      it("deals the opening hand after the splash when arriving from the room", () => {
        useMatchStore.getState().setMatchState(dealtState({ handNumber: 1 }));
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage({ fromRoom: true });

        expect(screen.getByTestId("deal-animation")).toBeInTheDocument();
        expect(screen.queryByTestId("trump-prompt")).not.toBeInTheDocument();
        act(() => {
          vi.advanceTimersByTime(MOTION.DEAL_DURATION_CANDIDATE_FIRST);
        });
        expect(screen.getByTestId("trump-prompt")).toBeInTheDocument();
        expect(sfxCalls("deal")).toHaveLength(1);
      });

      it("drops the arrival flag from the history entry, so a reload replays neither splash nor deal", () => {
        useMatchStore.getState().setMatchState(dealtState({ handNumber: 1 }));
        useMatchStore.getState().setMyPlayerSeat(0);
        const { router } = renderMatchPage({ fromRoom: true });

        expect(screen.getByTestId("deal-animation")).toBeInTheDocument();
        expect(router.state.location.state).toBeNull();
        expect(router.state.location.pathname).toBe("/match/1");
      });

      it("keeps the beat and the sounds under reduced motion, without flights", () => {
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
        useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "hand_complete" });
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();
        act(() => {
          useMatchStore.getState().setMatchState(dealtState());
        });

        expect(document.querySelector('[data-testid^="card-flight-deal-"]')).toBeNull();
        expect(screen.queryByTestId("trump-prompt")).not.toBeInTheDocument();
        act(() => {
          vi.advanceTimersByTime(MOTION.DEAL_DURATION_CANDIDATE_FIRST);
        });
        expect(sfxCalls("dealPacket")).toHaveLength(8);
        expect(screen.getByTestId("trump-prompt")).toBeInTheDocument();
      });
    });

    describe("trick-collect sequencing", () => {
      const trickA: { trick: TrickCard[]; winnerSeat: number } = {
        trick: [
          { card: { rank: "8", suit: "C" }, playerSeat: 0 },
          { card: { rank: "7", suit: "H" }, playerSeat: 1 },
          { card: { rank: "A", suit: "D" }, playerSeat: 2 },
          { card: { rank: "9", suit: "C" }, playerSeat: 3 },
        ],
        winnerSeat: 2,
      };
      const trickB: { trick: TrickCard[]; winnerSeat: number } = {
        trick: [
          { card: { rank: "Q", suit: "S" }, playerSeat: 2 },
          { card: { rank: "8", suit: "H" }, playerSeat: 3 },
          { card: { rank: "J", suit: "D" }, playerSeat: 0 },
          { card: { rank: "T", suit: "C" }, playerSeat: 1 },
        ],
        winnerSeat: 3,
      };
      const handScore: HandScoredPayload = {
        teamACardPoints: 90,
        teamBCardPoints: 72,
        teamADeclPoints: 0,
        teamBDeclPoints: 0,
        lastTrickTeam: 1,
        lastTrickBonus: 10,
        capot: false,
        capotTeam: null,
        capotBonus: 0,
        failedContract: false,
        contractingTeam: 0,
        teamAHandTotal: 90,
        teamBHandTotal: 82,
        teamAMatchScore: 90,
        teamBMatchScore: 82,
      };

      let gbcrSpy: ReturnType<typeof vi.spyOn>;
      beforeEach(() => {
        gbcrSpy = vi.spyOn(Element.prototype, "getBoundingClientRect").mockReturnValue({
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
      });
      afterEach(() => {
        gbcrSpy.mockRestore();
      });

      const collectFlights = () =>
        Array.from(document.querySelectorAll('[data-testid^="card-flight-collect-"]')).filter(
          (el) => !el.getAttribute("data-testid")!.endsWith("-animated"),
        );

      it("finalises the old sweep at once when the next trick resolves, and gives the new one its full glow and sweep", () => {
        useMatchStore.getState().setMatchState(mockMatchState);
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();

        act(() => {
          useMatchStore.getState().setPendingResolvedTrick(trickA);
        });
        act(() => {
          vi.advanceTimersByTime(MOTION.TRICK_RESOLVE_PAUSE);
        });
        expect(collectFlights()).toHaveLength(4);

        act(() => {
          useMatchStore.getState().setPendingResolvedTrick(trickB);
          useMatchStore.getState().setScoreRevealData(handScore);
        });
        expect(collectFlights()).toHaveLength(0);
        expect(screen.getAllByTestId(/^trick-slot-card-\d-resolved$/)).toHaveLength(4);
        expect(screen.getByTestId("playing-card-QS")).toBeInTheDocument();
        expect(screen.queryByTestId("score-reveal")).not.toBeInTheDocument();

        act(() => {
          vi.advanceTimersByTime(MOTION.TRICK_RESOLVE_PAUSE - 1);
        });
        expect(collectFlights()).toHaveLength(0);
        act(() => {
          vi.advanceTimersByTime(1);
        });
        expect(collectFlights()).toHaveLength(4);
        expect(screen.queryByTestId("score-reveal")).not.toBeInTheDocument();

        act(() => {
          for (const el of document.querySelectorAll(
            '[data-testid^="card-flight-collect-"][data-testid$="-animated"]',
          )) {
            el.dispatchEvent(new Event("animationend", { bubbles: true }));
          }
        });
        expect(useMatchStore.getState().pendingResolvedTrick).toBeNull();
        expect(screen.getByTestId("score-reveal")).toBeInTheDocument();
      });

      it("still clears on the fallback timer when no flight reports its end", () => {
        useMatchStore.getState().setMatchState(mockMatchState);
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();

        act(() => {
          useMatchStore.getState().setPendingResolvedTrick(trickA);
        });
        act(() => {
          vi.advanceTimersByTime(MOTION.TRICK_RESOLVE_PAUSE + 100);
        });
        act(() => {
          useMatchStore.getState().setPendingResolvedTrick(trickB);
        });
        act(() => {
          vi.advanceTimersByTime(MOTION.TRICK_RESOLVE_PAUSE + MOTION.CARD_FLIGHT_COLLECT + 400);
        });
        expect(useMatchStore.getState().pendingResolvedTrick).toBeNull();
      });

      it("never lets a late finish of the old trick clear the new one", () => {
        const store = useMatchStore.getState();
        store.setPendingResolvedTrick(trickA);
        const a = useMatchStore.getState().pendingResolvedTrick!.receivedAt;
        store.setPendingResolvedTrick(trickB);
        const b = useMatchStore.getState().pendingResolvedTrick!.receivedAt;

        expect(b).toBeGreaterThan(a);
        useMatchStore.getState().clearPendingResolvedTrick(a);
        expect(useMatchStore.getState().pendingResolvedTrick?.receivedAt).toBe(b);
        useMatchStore.getState().clearPendingResolvedTrick(b);
        expect(useMatchStore.getState().pendingResolvedTrick).toBeNull();
      });

      it("holds the viewer's card until the last trick has swept away", () => {
        useMatchStore.getState().setMatchState({ ...mockMatchState, activePlayerSeat: 0 });
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();
        act(() => {
          useMatchStore.getState().setPendingResolvedTrick(trickA);
        });

        const card = () => screen.getByTestId("playing-card-KS");
        expect(
          within(screen.getByTestId("hand-cards")).getByTestId("playing-card-KS"),
        ).toHaveAttribute("tabindex", "-1");
        fireEvent.click(within(screen.getByTestId("hand-cards")).getByTestId("playing-card-KS"));
        expect(mockSendMessage).not.toHaveBeenCalledWith("action:play_card", expect.anything());

        act(() => {
          useMatchStore.getState().setPendingResolvedTrick(null);
        });
        expect(card()).toHaveAttribute("tabindex", "0");
      });
    });

    describe("event sounds", () => {
      const endPayload = {
        winnerTeam: 0,
        teamAFinalScore: 1020,
        teamBFinalScore: 850,
        matchDurationSec: 300,
      };

      it.each([
        ["matchWin", 0],
        ["matchLose", 1],
      ] as const)("plays %s once when the result opens for seat %i", (sound, seat) => {
        useMatchStore.getState().setMatchState({
          ...mockMatchState,
          phase: "match_end",
          players: mockMatchState.players.map((p) => ({
            ...p,
            userId: p.seat === seat ? 10 : p.userId === 10 ? 99 : p.userId,
          })) as MatchState["players"],
        });
        useMatchStore.getState().setMatchEndData(endPayload);
        const { unmount } = renderMatchPage();

        expect(screen.getByTestId("match-result")).toBeInTheDocument();
        expect(sfxCalls(sound)).toHaveLength(1);
        const key = sfxCalls(sound)[0]![1];
        unmount();
        renderMatchPage();
        expect(sfxCalls(sound).at(-1)![1]).toEqual(key);
        expect(sfxCalls(sound === "matchWin" ? "matchLose" : "matchWin")).toHaveLength(0);
      });

      it.each([
        ["the team left at the table wins", 0, 1, "matchWin"],
        ["the abandoner's partner loses", 3, 1, "matchLose"],
      ] as const)("abandonment: %s", (_label, viewerSeat, abandoner, sound) => {
        useMatchStore.getState().setMatchState({
          ...mockMatchState,
          players: mockMatchState.players.map((p) => ({
            ...p,
            userId: p.seat === viewerSeat ? 10 : p.userId === 10 ? 99 : p.userId,
          })) as MatchState["players"],
        });
        useMatchStore.getState().setMatchAbandonedData({
          abandonedByPlayer: abandoner,
          teamAFinalScore: 300,
          teamBFinalScore: 200,
          matchDurationSec: 200,
        });
        renderMatchPage();

        expect(screen.getByTestId("reconnect-overlay")).toBeInTheDocument();
        expect(sfxCalls(sound)).toHaveLength(1);
      });

      it("keeps the abandoner's own late view of the result silent", () => {
        useMatchStore.getState().setMatchState(mockMatchState);
        useMatchStore.getState().setMatchAbandonedData({
          abandonedByPlayer: 0,
          teamAFinalScore: 300,
          teamBFinalScore: 200,
          matchDurationSec: 200,
        });
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();

        expect(sfxCalls("matchWin")).toHaveLength(0);
        expect(sfxCalls("matchLose")).toHaveLength(0);
      });

      const capotScore: HandResult = {
        teamACardPoints: 162,
        teamBCardPoints: 0,
        teamADeclPoints: 0,
        teamBDeclPoints: 0,
        lastTrickTeam: 0,
        lastTrickBonus: 10,
        capot: true,
        capotTeam: 0,
        capotBonus: 90,
        failedContract: false,
        contractingTeam: 0,
        teamAHandTotal: 252,
        teamBHandTotal: 0,
      };

      it("plays the capot jingle once when the banner opens", () => {
        useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "hand_complete" });
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();

        act(() => {
          useMatchStore
            .getState()
            .setScoreRevealData({ ...capotScore, teamAMatchScore: 252, teamBMatchScore: 0 });
        });
        expect(screen.getByTestId("capot-animation")).toBeInTheDocument();
        expect(sfxCalls("capot")).toHaveLength(1);
      });

      it("keeps a capot banner rebuilt on reconnect silent", () => {
        useMatchStore.getState().setMatchState({
          ...mockMatchState,
          phase: "hand_complete",
          lastHandResult: capotScore,
          teamScores: [252, 0],
        });
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();

        expect(screen.getByTestId("capot-animation")).toBeInTheDocument();
        expect(sfxCalls("capot")).toHaveLength(0);
      });

      it("plays the declaration sound once for a declaration reveal and for a Belote reveal", () => {
        useMatchStore.getState().setMatchState(mockMatchState);
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();

        act(() => {
          useMatchStore.getState().setDeclarationReveal({
            winnerTeam: 0,
            contested: false,
            declarations: [
              { playerSeat: 0, type: "sequence", cards: ["9S", "TS", "JS"], value: 20 },
            ],
          });
        });
        expect(screen.getByTestId("declaration-reveal")).toBeInTheDocument();
        expect(sfxCalls("declaration")).toHaveLength(1);

        act(() => {
          useMatchStore.getState().setBelotReveal({ playerSeat: 1, team: 1, cardId: "KS" });
        });
        expect(screen.getByTestId("belot-reveal")).toBeInTheDocument();
        expect(sfxCalls("declaration")).toHaveLength(2);
        expect(sfxCalls("declaration")[0]![1]).not.toEqual(sfxCalls("declaration")[1]![1]);
      });
    });

    describe("urgent clock tick", () => {
      function onTheClock(seat: number, remainingMs: number): MatchState {
        return {
          ...mockMatchState,
          activePlayerSeat: seat,
          timerDurationSec: 24,
          turnExpiresAt: new Date(Date.now() + remainingMs).toISOString(),
        };
      }

      it("ticks once per second in the viewer's red zone, down to 1", () => {
        useMatchStore.getState().setMatchState(onTheClock(0, 5_000));
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();
        expect(sfxCalls("clockTick")).toHaveLength(0);

        act(() => {
          vi.advanceTimersByTime(2_100);
        });
        expect(sfxCalls("clockTick")).toHaveLength(1);
        act(() => {
          vi.advanceTimersByTime(1_000);
        });
        act(() => {
          vi.advanceTimersByTime(1_000);
        });
        expect(sfxCalls("clockTick")).toHaveLength(3);
        act(() => {
          vi.advanceTimersByTime(2_000);
        });
        expect(sfxCalls("clockTick")).toHaveLength(3);
        expect(new Set(sfxCalls("clockTick").map(([, opts]) => opts.dedupeKey)).size).toBe(3);
      });

      it("only asks for the seconds still to come when remounting into a red timer", () => {
        useMatchStore.getState().setMatchState(onTheClock(0, 2_500));
        useMatchStore.getState().setMyPlayerSeat(0);
        const { unmount } = renderMatchPage();
        expect(sfxCalls("clockTick")).toHaveLength(1);
        unmount();

        renderMatchPage();
        const keys = () => sfxCalls("clockTick").map(([, opts]) => opts.dedupeKey);
        expect(new Set(keys()).size).toBe(1);

        for (let s = 0; s < 4; s++) {
          act(() => {
            vi.advanceTimersByTime(1_000);
          });
        }
        expect(new Set(keys()).size).toBe(3);
      });

      it("stays silent on another seat's red timer", () => {
        useMatchStore.getState().setMatchState(onTheClock(1, 2_500));
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();
        act(() => {
          vi.advanceTimersByTime(3_000);
        });
        expect(sfxCalls("clockTick")).toHaveLength(0);
      });

      const bidding = (overrides: Partial<MatchState> = {}): Partial<MatchState> => ({
        phase: "bidding",
        trumpSuit: null,
        trumpCallerSeat: null,
        trumpCandidate: { rank: "A", suit: "H" },
        biddingRound: 1,
        biddingPassCount: 0,
        trickNumber: 0,
        ...overrides,
      });
      it.each<[string, Partial<MatchState>, () => HTMLElement]>([
        [
          "a card",
          {},
          () => within(screen.getByTestId("hand-cards")).getByTestId("playing-card-KS"),
        ],
        ["a pass", bidding(), () => screen.getByTestId("trump-prompt-pass")],
        ["a pick", bidding(), () => screen.getByTestId("trump-prompt-pick")],
        [
          "a Belote answer",
          {
            pendingBelotSeat: 0,
            currentTrick: [{ playerSeat: 0, card: { rank: "Q", suit: "S" } }],
          },
          () => screen.getByTestId("belot-prompt-announce"),
        ],
      ])("stops as soon as the viewer answers with %s", (_label, patch, target) => {
        useMatchStore.getState().setMatchState({ ...onTheClock(0, 2_500), ...patch });
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();
        expect(sfxCalls("clockTick")).toHaveLength(1);

        act(() => {
          fireEvent.click(target());
        });
        act(() => {
          vi.advanceTimersByTime(3_000);
        });
        expect(sfxCalls("clockTick")).toHaveLength(1);
      });

      it("stays silent for a declaration window the viewer has already skipped", () => {
        useMatchStore.getState().setMatchState({
          ...mockMatchState,
          phase: "declaring",
          trumpSuit: "S",
          players: mockMatchState.players.map((p) =>
            p.seat === 0
              ? {
                  ...p,
                  hand: [
                    { rank: "9", suit: "S" },
                    { rank: "T", suit: "S" },
                    { rank: "J", suit: "S" },
                  ],
                }
              : p,
          ) as MatchState["players"],
        });
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();

        act(() => {
          fireEvent.click(screen.getByTestId("declaration-prompt-skip"));
        });
        act(() => {
          vi.advanceTimersByTime(MOTION.DECLARATION_PHASE_AUTO_SKIP - 900);
        });
        expect(sfxCalls("clockTick")).toHaveLength(0);
      });

      it("ticks again for a turn whose answer the server refused", () => {
        useMatchStore.getState().setMatchState(onTheClock(0, 2_500));
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();
        act(() => {
          fireEvent.click(within(screen.getByTestId("hand-cards")).getByTestId("playing-card-KS"));
        });

        act(() => {
          useMatchStore.getState().setLastError("error:illegal_play");
        });
        act(() => {
          vi.advanceTimersByTime(1_000);
        });
        const seconds = new Set(sfxCalls("clockTick").map(([, opts]) => opts.dedupeKey));
        expect(seconds.size).toBe(2);
      });

      it("ticks for the declaration window while it is unanswered", () => {
        useMatchStore.getState().setMatchState({
          ...mockMatchState,
          phase: "declaring",
          trumpSuit: "S",
          players: mockMatchState.players.map((p) =>
            p.seat === 0
              ? {
                  ...p,
                  hand: [
                    { rank: "9", suit: "S" },
                    { rank: "T", suit: "S" },
                    { rank: "J", suit: "S" },
                  ],
                }
              : p,
          ) as MatchState["players"],
        });
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();

        act(() => {
          vi.advanceTimersByTime(MOTION.DECLARATION_PHASE_AUTO_SKIP - 900);
        });
        expect(sfxCalls("clockTick")).toHaveLength(1);
      });
    });

    describe("belote prompt", () => {
      function belotTurn(overrides: Partial<MatchState> = {}): MatchState {
        return {
          ...mockMatchState,
          trumpSuit: "S",
          activePlayerSeat: 0,
          timerDurationSec: 10,
          turnExpiresAt: new Date(Date.now() + 8_000).toISOString(),
          players: mockMatchState.players.map((p) =>
            p.seat === 0
              ? {
                  ...p,
                  hand: [
                    { rank: "K", suit: "S" },
                    { rank: "Q", suit: "S" },
                    { rank: "7", suit: "H" },
                  ],
                }
              : p,
          ) as MatchState["players"],
          ...overrides,
        };
      }

      it("shows the running per-move ring on the local prompt", () => {
        useMatchStore.getState().setMatchState(belotTurn());
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();

        act(() => {
          fireEvent.click(screen.getByTestId("playing-card-KS"));
        });
        const prompt = screen.getByTestId("belot-prompt");
        expect(within(prompt).getByTestId("button-timer-ring")).toBeInTheDocument();
      });

      it("closes the local prompt when the server auto-plays the card, keeping no choice", () => {
        useMatchStore.getState().setMatchState(belotTurn());
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();
        act(() => {
          fireEvent.click(screen.getByTestId("playing-card-KS"));
        });
        expect(screen.getByTestId("belot-prompt")).toBeInTheDocument();

        act(() => {
          useMatchStore.getState().setMatchState(
            belotTurn({
              activePlayerSeat: 1,
              currentTrick: [{ playerSeat: 0, card: { rank: "K", suit: "S" } }],
              players: belotTurn().players.map((p) =>
                p.seat === 0
                  ? {
                      ...p,
                      hand: [
                        { rank: "Q", suit: "S" },
                        { rank: "7", suit: "H" },
                      ],
                    }
                  : p,
              ) as MatchState["players"],
            }),
          );
        });
        expect(screen.queryByTestId("belot-prompt")).not.toBeInTheDocument();

        act(() => {
          useMatchStore.getState().setMatchState(belotTurn({ pendingBelotSeat: 0 }));
        });
        expect(mockSendMessage).not.toHaveBeenCalledWith("action:announce_belot", {});
        expect(mockSendMessage).not.toHaveBeenCalledWith("action:decline_belot", {});
      });

      it("drops a stored answer whose play the server never took", () => {
        useMatchStore.getState().setMatchState(belotTurn());
        useMatchStore.getState().setMyPlayerSeat(0);
        renderMatchPage();
        act(() => {
          fireEvent.click(screen.getByTestId("playing-card-KS"));
        });
        act(() => {
          fireEvent.click(screen.getByTestId("belot-prompt-announce"));
        });
        expect(mockSendMessage).toHaveBeenCalledWith("action:play_card", { cardId: "KS" });

        act(() => {
          useMatchStore.getState().setMatchState(
            belotTurn({
              activePlayerSeat: 1,
              currentTrick: [{ playerSeat: 0, card: { rank: "7", suit: "H" } }],
            }),
          );
        });
        act(() => {
          useMatchStore.getState().setMatchState(belotTurn({ pendingBelotSeat: 0 }));
        });
        expect(mockSendMessage).not.toHaveBeenCalledWith("action:announce_belot", {});
      });
    });
  });
});
