// Diamond layout invariant test. The waiting room pins seat indices to
// fixed cardinal positions (seat 0 → south, 1 → east, 2 → north, 3 → west)
// regardless of which seat the viewer occupies. This is intentionally
// different from the in-game table which DOES rotate to put the viewer at
// the bottom — in the lobby, rotating after a click made seat selection
// feel unpredictable ("I clicked the top tile but landed at the bottom").
//
// JSDOM caveat: `getBoundingClientRect()` returns zeroed coordinates, so
// asserting visual position via geometry is meaningless here. We instead
// read the inline `gridArea` style — that is what the CSS Grid
// `grid-template-areas` resolution depends on, and it survives JSDOM's
// render. (Visual geometry is re-checked in a real browser via Playwright
// in the verification step of the redesign change.)

import "@/shared/i18n/i18n";

import { render, screen, waitFor } from "@testing-library/react";
import { BrowserRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useAuthStore } from "@/shared/stores/authStore";
import { useChatStore } from "@/shared/stores/chatStore";
import { QueryWrapper } from "@/test-utils";

import { RoomPage } from "./RoomPage";

// Mirror the existing RoomPage.test.tsx mock surface — RoomPage pulls
// router, room API, WS context, and toasts. Without these the render
// throws before we get to the seat tiles.
const mockNavigate = vi.fn();
vi.mock("react-router", async () => {
  const actual = await vi.importActual("react-router");
  return {
    ...actual,
    useParams: () => ({ id: "1" }),
    useNavigate: () => mockNavigate,
  };
});

vi.mock("@/shared/api/rooms", () => ({
  addBot: vi.fn(),
  removeBot: vi.fn(),
  getRoom: vi.fn(),
  leaveRoom: vi.fn(),
  selectSeat: vi.fn(),
  startMatch: vi.fn(),
  kickPlayer: vi.fn(),
  swapSeats: vi.fn(),
}));

vi.mock("@/shared/providers/WebSocketContext", () => ({
  useWsConnectionState: () => "connected",
  useWsSendMessage: () => vi.fn(),
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

import { getRoom, leaveRoom } from "@/shared/api/rooms";
const mockGetRoom = vi.mocked(getRoom);
const mockLeaveRoom = vi.mocked(leaveRoom);

// fourSeated builds a fully-seated lobby: seats 0,2 → teamA, 1,3 → teamB.
// `viewerUserId` selects which seat the rendered viewer occupies.
function fourSeated(viewerSeat: number) {
  // viewerSeat → userId map: viewer is always userId 100 for clarity.
  const usernames = ["alice", "bob", "carol", "dave"];
  const players = [0, 1, 2, 3].map((seat) => ({
    id: seat + 1,
    roomId: 1,
    userId: seat === viewerSeat ? 100 : 200 + seat,
    username: usernames[seat]!,
    seat,
    team: seat % 2 === 0 ? "teamA" : "teamB",
    isBot: false,
    createdAt: "",
  }));
  return {
    room: {
      id: 1,
      name: "Test Room",
      code: "ABC123",
      ownerId: 100,
      ownerUsername: "owner",
      variant: "bitola",
      matchMode: "1001",
      timerStyle: "relaxed",
      timerDurationSeconds: null,
      status: "waiting",
      playerCount: 4,
      isQuickPlay: false,
      coinBuyIn: 0,
      createdAt: "",
      updatedAt: "",
    },
    players,
  };
}

function renderRoomPage() {
  return render(
    <QueryWrapper>
      <BrowserRouter>
        <RoomPage />
      </BrowserRouter>
    </QueryWrapper>,
  );
}

beforeEach(() => {
  mockLeaveRoom.mockResolvedValue(undefined);
  Element.prototype.scrollIntoView = vi.fn();
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  });
});

afterEach(() => {
  vi.clearAllMocks();
  useAuthStore.setState({ user: null, token: null });
  useChatStore.setState({ lobbyMessages: [], matchMessages: [], roomMessages: [] });
});

describe("RoomPage diamond layout invariant", () => {
  it.each([0, 1, 2, 3])(
    "viewerSeat=%i → seats are pinned to fixed cardinals (0=south, 1=east, 2=north, 3=west)",
    async (viewerSeat) => {
      useAuthStore.setState({
        user: {
          id: 100,
          username: ["alice", "bob", "carol", "dave"][viewerSeat]!,
          email: "v@b.com",
          languagePreference: "en",
          walletBalance: 5000,
          loginStreakDays: 1,
          createdAt: "",
        },
        token: "tok",
      });

      mockGetRoom.mockResolvedValue(fourSeated(viewerSeat));

      renderRoomPage();

      await waitFor(() => {
        expect(screen.getByTestId("seats-grid")).toBeInTheDocument();
      });

      const south = screen.getByTestId("seat-position-south");
      const north = screen.getByTestId("seat-position-north");
      const east = screen.getByTestId("seat-position-east");
      const west = screen.getByTestId("seat-position-west");

      // Placement source — JSDOM doesn't compute layout, but the responsive
      // `sm:[grid-area:<cardinal>]` class survives the React render and is what
      // drives the CSS-Grid `grid-template-areas` resolution at sm+. (It's
      // scoped to sm so the mobile 2-col grid auto-flows instead of collapsing
      // every tile into one cell.)
      expect(south.className).toContain("sm:[grid-area:south]");
      expect(north.className).toContain("sm:[grid-area:north]");
      expect(east.className).toContain("sm:[grid-area:east]");
      expect(west.className).toContain("sm:[grid-area:west]");

      function seatIndexAt(el: HTMLElement): number {
        const button = el.querySelector("[data-testid^='player-seat-']") as HTMLElement | null;
        expect(button, "expected nested player-seat-N button").not.toBeNull();
        const match = button!.getAttribute("data-testid")!.match(/^player-seat-(\d)$/);
        expect(
          match,
          `seat testid did not match: ${button!.getAttribute("data-testid")}`,
        ).not.toBeNull();
        return Number(match![1]);
      }

      // Fixed-position contract: seat indices map 1:1 to cardinals
      // regardless of viewer. Click target visually matches click result.
      expect(seatIndexAt(south as HTMLElement)).toBe(0);
      expect(seatIndexAt(east as HTMLElement)).toBe(1);
      expect(seatIndexAt(north as HTMLElement)).toBe(2);
      expect(seatIndexAt(west as HTMLElement)).toBe(3);

      // Partnership data-team contract still holds: vertical pair (0+2) =
      // Team A, horizontal pair (1+3) = Team B. This is the layout's
      // visual partnership cue (vertical lane = Team A, horizontal lane =
      // Team B) and downstream styles depend on `data-team` being set.
      expect((south as HTMLElement).getAttribute("data-team")).toBe("teamA");
      expect((north as HTMLElement).getAttribute("data-team")).toBe("teamA");
      expect((east as HTMLElement).getAttribute("data-team")).toBe("teamB");
      expect((west as HTMLElement).getAttribute("data-team")).toBe("teamB");
    },
  );
});
