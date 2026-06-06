import "@/shared/i18n/i18n";

import { act, fireEvent, render, screen } from "@testing-library/react";
import React from "react";
import { BrowserRouter, MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { MOTION } from "@/shared/lib/motion";
import { useAuthStore } from "@/shared/stores/authStore";
import { useMatchStore } from "@/shared/stores/matchStore";
import type { MatchState } from "@/shared/types/matchTypes";

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
    },
    {
      hand: [{ rank: "7", suit: "H" }],
      seat: 1,
      userId: 20,
      username: "Bob",
      team: "teamB",
      declarations: [],
      connected: true,
    },
    {
      hand: [{ rank: "A", suit: "D" }],
      seat: 2,
      userId: 30,
      username: "Carol",
      team: "teamA",
      declarations: [],
      connected: true,
    },
    {
      hand: [{ rank: "9", suit: "C" }],
      seat: 3,
      userId: 40,
      username: "Dave",
      team: "teamB",
      declarations: [],
      connected: true,
    },
  ],
  teamScores: [0, 0],
  handPoints: [0, 0],
  declarationPoints: [0, 0],
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
function renderMatchPage({ fromRoom = false, skipSplash = true } = {}) {
  const result = render(
    fromRoom ? (
      <MemoryRouter initialEntries={[{ pathname: "/match/1", state: { fromRoom: true } }]}>
        <MatchPage />
      </MemoryRouter>
    ) : (
      <BrowserRouter>
        <MatchPage />
      </BrowserRouter>
    ),
  );
  if (fromRoom && skipSplash) {
    act(() => {
      vi.advanceTimersByTime(MOTION.GAME_STARTING_SPLASH);
    });
  }
  return result;
}

describe("MatchPage", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    mockSendMessage.mockClear();
    useMatchStore.getState().reset();
    useAuthStore.setState({
      token: "test-token",
      user: {
        id: 10,
        email: "a@b.com",
        username: "Alice",
        languagePreference: "en",
        createdAt: "",
      },
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

    const { rerender } = renderMatchPage();
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
    rerender(
      <BrowserRouter>
        <MatchPage />
      </BrowserRouter>,
    );

    expect(screen.queryByTestId("surrender-button")).not.toBeInTheDocument();
  });

  it("shows SurrenderPrompt for the partner when surrenderProposerSeat is set", () => {
    // Local player is seat 2 (Carol); proposer is seat 0 (Alice). Partner of
    // proposer is (0 + 2) % 4 == 2, i.e. the local player.
    useAuthStore.setState({
      token: "test-token",
      user: {
        id: 30,
        email: "c@b.com",
        username: "Carol",
        languagePreference: "en",
        createdAt: "",
      },
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
      user: {
        id: 20,
        email: "b@b.com",
        username: "Bob",
        languagePreference: "en",
        createdAt: "",
      },
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
      user: {
        id: 30,
        email: "c@b.com",
        username: "Carol",
        languagePreference: "en",
        createdAt: "",
      },
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

    const { rerender } = renderMatchPage();

    act(() => {
      useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "match_end" });
      useMatchStore.getState().setMatchEndData({
        winnerTeam: 0,
        teamAFinalScore: 1020,
        teamBFinalScore: 850,
        matchDurationSec: 300,
      });
    });
    rerender(
      <BrowserRouter>
        <MatchPage />
      </BrowserRouter>,
    );

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
      user: {
        id: 20,
        email: "b@b.com",
        username: "Bob",
        languagePreference: "en",
        createdAt: "",
      },
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

    const { rerender } = renderMatchPage();

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
    rerender(
      <BrowserRouter>
        <MatchPage />
      </BrowserRouter>,
    );

    expect(screen.queryByTestId("emote-bubble-2")).not.toBeInTheDocument();
  });

  it("elevates seat wrappers above the trick area so emotes/surrender banners aren't hidden by thrown cards", () => {
    // Regression: on phones the EmoteBubble / SurrenderOpponentBanner render
    // INSIDE the seat wrapper, whose `-translate-*` transform forms a stacking
    // context — so their own `z-20` is trapped there and can't beat the center
    // TrickArea (rendered later in the DOM at z-auto). The wrapper itself must
    // carry `z-20` to outrank the thrown cards; otherwise the cards paint over
    // the emote/banner (most visible on mobile, where seats overlap the table).
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    for (const compass of [0, 1, 2, 3] as const) {
      expect(screen.getByTestId(`player-seat-${compass}-wrapper`).className).toContain("z-20");
    }
  });

  it("hides declarationReveal while the table is paused (AC3)", () => {
    useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "paused" });
    useMatchStore.getState().setMyPlayerSeat(0);
    useMatchStore.getState().setDeclarationReveal({
      winnerTeam: 0,
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

    const { rerender } = renderMatchPage();

    act(() => {
      useMatchStore.getState().setMatchState({ ...mockMatchState, phase: "match_end" });
      useMatchStore.getState().setMatchEndData({
        winnerTeam: 0,
        teamAFinalScore: 1020,
        teamBFinalScore: 850,
        matchDurationSec: 300,
      });
    });
    rerender(
      <BrowserRouter>
        <MatchPage />
      </BrowserRouter>,
    );

    expect(screen.queryByTestId("belot-reveal")).not.toBeInTheDocument();
  });

  it("shows confirm dialog on browser back button and stays if declined", () => {
    useMatchStore.getState().setMatchState(mockMatchState);
    useMatchStore.getState().setMyPlayerSeat(0);

    renderMatchPage();

    // Mock window.confirm to decline leaving
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(false);

    // Simulate popstate event (browser back button)
    act(() => {
      window.dispatchEvent(new PopStateEvent("popstate"));
    });

    expect(confirmSpy).toHaveBeenCalledWith("Leave the game? You may lose your progress.");
    // Game state should not be cleared
    expect(useMatchStore.getState().matchState).not.toBeNull();

    confirmSpy.mockRestore();
  });
});
