import "@/shared/i18n/i18n";

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { BrowserRouter } from "react-router";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { i18n } from "@/shared/i18n/i18n";
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
  toast: { success: vi.fn(), error: vi.fn() },
}));

import { getRoom, leaveRoom } from "@/shared/api/rooms";
const mockGetRoom = vi.mocked(getRoom);
const mockLeaveRoom = vi.mocked(leaveRoom);

function fourSeatedRoom() {
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
    players: [
      {
        id: 1,
        roomId: 1,
        userId: 100,
        username: "alice",
        avatarUrl: null,
        seat: 0,
        team: "teamA",
        isBot: false,
        createdAt: "",
      },
      {
        id: 2,
        roomId: 1,
        userId: 200,
        username: "bob",
        avatarUrl: null,
        seat: 1,
        team: "teamB",
        isBot: false,
        createdAt: "",
      },
      {
        id: 3,
        roomId: 1,
        userId: 300,
        username: "carol",
        avatarUrl: null,
        seat: 2,
        team: "teamA",
        isBot: false,
        createdAt: "",
      },
      {
        id: 4,
        roomId: 1,
        userId: 400,
        username: "dave",
        avatarUrl: null,
        seat: 3,
        team: "teamB",
        isBot: false,
        createdAt: "",
      },
    ],
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

beforeAll(async () => {
  await i18n.changeLanguage("en");
});

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
  useAuthStore.setState({
    user: makeUser({
      id: 100,
      username: "alice",
      email: "a@b.com",
      loginStreakDays: 1,
      createdAt: "",
    }),
    token: "tok",
  });
  mockGetRoom.mockResolvedValue(fourSeatedRoom());
});

afterEach(async () => {
  cleanup();
  vi.clearAllMocks();
  useAuthStore.setState({ user: null, token: null });
  useChatStore.setState({ lobbyMessages: [], matchMessages: [], roomMessages: [] });
  await i18n.changeLanguage("en");
});

describe("RoomPage locale overflow", () => {
  it.each(["en", "sr"] as const)(
    "renders Us / Them legend labels without missing-key fallback in %s locale",
    async (locale) => {
      await i18n.changeLanguage(locale);

      renderRoomPage();

      await waitFor(() => {
        expect(screen.getByTestId("seats-grid")).toBeInTheDocument();
      });

      const expectedUs = locale === "en" ? "Us" : "Mi";
      const expectedThem = locale === "en" ? "Them" : "Oni";
      expect(screen.getByText(expectedUs)).toBeInTheDocument();
      expect(screen.getByText(expectedThem)).toBeInTheDocument();

      const seatGrid = screen.getByTestId("seats-grid");
      const tiles = seatGrid.querySelectorAll<HTMLElement>("[data-testid^='seat-position-']");
      expect(tiles.length).toBe(4);
    },
  );

  it.each([
    ["mk", "Хрватска"],
    ["hr", "Hrvatska"],
    ["sr", "Hrvatska"],
    ["en", "Croatian"],
  ] as const)("renders the localized Croatian variant badge in %s locale", async (lang, label) => {
    const detail = fourSeatedRoom();
    detail.room.variant = "croatia";
    mockGetRoom.mockResolvedValue(detail);
    await i18n.changeLanguage(lang);

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("badge-variant")).toBeInTheDocument();
    });
    expect(screen.getByTestId("badge-variant")).toHaveTextContent(label);
    expect(screen.getByTestId("badge-variant")).not.toHaveTextContent("croatia");
  });

  it("renders the localized 501 match-mode badge in mk locale", async () => {
    const detail = fourSeatedRoom();
    detail.room.matchMode = "501";
    mockGetRoom.mockResolvedValue(detail);
    await i18n.changeLanguage("mk");

    renderRoomPage();

    await waitFor(() => {
      expect(screen.getByTestId("badge-match-mode")).toBeInTheDocument();
    });
    expect(screen.getByTestId("badge-match-mode")).toHaveTextContent("501 поен");
  });
});
