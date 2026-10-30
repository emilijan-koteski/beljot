import "@/shared/i18n/i18n";

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { BrowserRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useAuthStore } from "@/shared/stores/authStore";
import { useChatStore } from "@/shared/stores/chatStore";
import { makeRoom, makeRoomDetail, makeUser, QueryWrapper } from "@/test-utils";

import { RoomPage } from "./RoomPage";

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

vi.mock("@/shared/api/matches", () => ({
  getRoomLastMatch: vi.fn(() => Promise.reject(new Error("no last match"))),
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

// Builds a fully seated lobby (seats 0,2 on team A, 1,3 on team B) with the viewer at viewerSeat.
function fourSeated(viewerSeat: number) {
  const usernames = ["alice", "bob", "carol", "dave"];
  const players = [0, 1, 2, 3].map((seat) => ({
    id: seat + 1,
    roomId: 1,
    userId: seat === viewerSeat ? 100 : 200 + seat,
    username: usernames[seat]!,
    avatarUrl: null,
    seat,
    team: seat % 2 === 0 ? "teamA" : "teamB",
    isBot: false,
    createdAt: "",
  }));
  return makeRoomDetail({
    room: makeRoom({
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
    }),
    players,
  });
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
  cleanup();
  vi.clearAllMocks();
  useAuthStore.setState({ user: null, token: null });
  useChatStore.setState({ lobbyMessages: [], matchMessages: [], roomMessages: [] });
});

describe("RoomPage table orientation", () => {
  it.each([0, 1, 2, 3])(
    "viewerSeat=%i → the viewer sits at the bottom and the next seat on their right, as at the match table",
    async (viewerSeat) => {
      useAuthStore.setState({
        user: makeUser({
          id: 100,
          username: ["alice", "bob", "carol", "dave"][viewerSeat]!,
          email: "v@b.com",
          loginStreakDays: 1,
          createdAt: "",
        }),
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

      expect(south.className).toContain("[grid-area:south]");
      expect(north.className).toContain("[grid-area:north]");
      expect(east.className).toContain("[grid-area:east]");
      expect(west.className).toContain("[grid-area:west]");

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

      expect(seatIndexAt(south as HTMLElement)).toBe(viewerSeat);
      expect(seatIndexAt(east as HTMLElement)).toBe((viewerSeat + 1) % 4);
      expect(seatIndexAt(north as HTMLElement)).toBe((viewerSeat + 2) % 4);
      expect(seatIndexAt(west as HTMLElement)).toBe((viewerSeat + 3) % 4);

      const ownTeam = viewerSeat % 2 === 0 ? "teamA" : "teamB";
      const otherTeam = ownTeam === "teamA" ? "teamB" : "teamA";
      expect((south as HTMLElement).getAttribute("data-team")).toBe(ownTeam);
      expect((north as HTMLElement).getAttribute("data-team")).toBe(ownTeam);
      expect((east as HTMLElement).getAttribute("data-team")).toBe(otherTeam);
      expect((west as HTMLElement).getAttribute("data-team")).toBe(otherTeam);
    },
  );
});
