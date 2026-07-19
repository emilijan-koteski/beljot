import { renderHook } from "@testing-library/react";
import React from "react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useAuthStore } from "@/shared/stores/authStore";
import { useMatchStore } from "@/shared/stores/matchStore";
import type { MatchState } from "@/shared/types/matchTypes";

import { useReconnectionRedirect } from "./useReconnectionRedirect";

vi.mock("@/shared/api/auth", () => ({
  logout: vi.fn(),
}));

const mockNavigate = vi.fn();
let currentPathname = "/lobby";

vi.mock("react-router", async () => {
  const actual = await vi.importActual<typeof import("react-router")>("react-router");
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useLocation: () => ({ pathname: currentPathname }),
  };
});

function wrapper({ children }: { children: React.ReactNode }) {
  return <MemoryRouter>{children}</MemoryRouter>;
}

const minimalMatchState: MatchState = {
  id: 1,
  roomId: 42,
  variant: "bitola",
  matchMode: "1001",
  phase: "playing",
  handNumber: 1,
  dealerSeat: 0,
  trumpCandidate: null,
  trumpSuit: null,
  trumpCallerSeat: null,
  biddingRound: 1,
  biddingPassCount: 0,
  deck: [],
  activePlayerSeat: 0,
  trickNumber: 1,
  currentTrick: [],
  trickWinnerSeat: null,
  declarationsResolved: false,
  belotAnnounced: false,
  awaitingDeclaration: false,
  pendingBelotSeat: null,
  leadSuit: null,
  players: [
    {
      hand: [],
      seat: 0,
      userId: 1,
      username: "P1",
      team: "teamA",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
    },
    {
      hand: [],
      seat: 1,
      userId: 2,
      username: "P2",
      team: "teamB",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
    },
    {
      hand: [],
      seat: 2,
      userId: 3,
      username: "P3",
      team: "teamA",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
    },
    {
      hand: [],
      seat: 3,
      userId: 4,
      username: "P4",
      team: "teamB",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
    },
  ],
  teamScores: [0, 0],
  handPoints: [0, 0],
  tricksWon: [0, 0],
  declarationPoints: [0, 0],
  belotPoints: [0, 0],
  winnerTeam: null,
  lastHandResult: null,
  turnExpiresAt: null,
  timerDurationSec: 0,
  turnTimeRemaining: 0,
  ownerSeat: -1,
  pausedPlayers: [false, false, false, false],
  pauseUsed: [false, false, false, false],
  surrenderProposerSeat: null,
  surrenderUsed: [false, false, false, false],
  previousPhase: "",
  disconnectedSeat: -1,
  reconnectExpiresAt: null,
  playerReconnectExpiresAt: [null, null, null, null] as [
    string | null,
    string | null,
    string | null,
    string | null,
  ],
};

describe("useReconnectionRedirect", () => {
  beforeEach(() => {
    mockNavigate.mockClear();
    currentPathname = "/lobby";
    useMatchStore.setState({ matchState: null, roomId: null });
  });

  it("redirects to game page when game state arrives while on lobby (push, so back can pop)", () => {
    useMatchStore.setState({ matchState: minimalMatchState, roomId: 42 });

    renderHook(() => useReconnectionRedirect(), { wrapper });

    // From the lobby the redirect PUSHES so the stack becomes [lobby, match]
    // and a later return to the lobby can pop back to the live entry.
    expect(mockNavigate).toHaveBeenCalledWith("/match/42", { replace: false });
  });

  it("redirects with replace when game state arrives away from the lobby", () => {
    currentPathname = "/profile";
    useMatchStore.setState({ matchState: minimalMatchState, roomId: 42 });

    renderHook(() => useReconnectionRedirect(), { wrapper });

    expect(mockNavigate).toHaveBeenCalledWith("/match/42", { replace: true });
  });

  it("does not redirect when already on game page", () => {
    currentPathname = "/match/42";
    useMatchStore.setState({ matchState: minimalMatchState, roomId: 42 });

    renderHook(() => useReconnectionRedirect(), { wrapper });

    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("does not redirect when no game state", () => {
    renderHook(() => useReconnectionRedirect(), { wrapper });

    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("does not redirect when game phase is match_end", () => {
    const endedState = { ...minimalMatchState, phase: "match_end" as const };
    useMatchStore.setState({ matchState: endedState, roomId: 42 });

    renderHook(() => useReconnectionRedirect(), { wrapper });

    expect(mockNavigate).not.toHaveBeenCalled();
  });

  // Story 8.5-1 AC6 (D66): regression coverage. Before the fix, a 401 on a
  // finished match → refresh-fail → authStore.logout left matchStore.matchState
  // populated, and on the next login this hook would redirect the freshly
  // re-logged-in user to the (no-longer-existing) /game/{roomId}. The fix is
  // store-level: authStore.logout() now wipes matchStore. After logout, this
  // hook MUST observe a clean store and NOT navigate.
  it("does not redirect after logout (D66 regression)", () => {
    useMatchStore.setState({ matchState: minimalMatchState, roomId: 42 });
    useAuthStore.setState({ token: "expired", user: null, isLoading: false });

    useAuthStore.getState().logout();

    renderHook(() => useReconnectionRedirect(), { wrapper });

    expect(mockNavigate).not.toHaveBeenCalled();
    expect(useMatchStore.getState().matchState).toBeNull();
  });
});
