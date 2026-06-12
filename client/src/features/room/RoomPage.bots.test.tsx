import "@/shared/i18n/i18n";

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BrowserRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useAuthStore } from "@/shared/stores/authStore";
import { useChatStore } from "@/shared/stores/chatStore";
import { QueryWrapper } from "@/test-utils";

import { RoomPage } from "./RoomPage";

// Story 10.3 — bot seating in the room lobby: owner add/remove affordances,
// localized seat-derived identity, swap-flow participation, and the start
// gate counting bot-covered seats.

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
  joinRoom: vi.fn(),
  leaveRoom: vi.fn(),
  selectSeat: vi.fn(),
  startMatch: vi.fn(),
  kickPlayer: vi.fn(),
  swapSeats: vi.fn(),
  leaveSeat: vi.fn(),
  transferOwnership: vi.fn(),
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

import { addBot, getRoom, joinRoom, leaveRoom, removeBot, swapSeats } from "@/shared/api/rooms";
import type { RoomPlayer } from "@/shared/types/apiTypes";

const mockGetRoom = vi.mocked(getRoom);
const mockJoinRoom = vi.mocked(joinRoom);
const mockLeaveRoom = vi.mocked(leaveRoom);
const mockAddBot = vi.mocked(addBot);
const mockRemoveBot = vi.mocked(removeBot);
const mockSwapSeats = vi.mocked(swapSeats);

function renderRoomPage() {
  render(
    <QueryWrapper>
      <BrowserRouter>
        <RoomPage />
      </BrowserRouter>
    </QueryWrapper>,
  );
}

const room = {
  id: 1,
  name: "Bot Room",
  code: "BOTBOT",
  ownerId: 10,
  ownerUsername: "alice",
  variant: "bitola",
  matchMode: "501",
  timerStyle: "relaxed",
  timerDurationSeconds: null,
  status: "waiting",
  playerCount: 1,
  isQuickPlay: false,
  createdAt: "",
  updatedAt: "",
};

const owner = {
  id: 10,
  username: "alice",
  email: "a@b.com",
  languagePreference: "en",
  createdAt: "",
};
const guest = {
  id: 20,
  username: "bob",
  email: "b@b.com",
  languagePreference: "en",
  createdAt: "",
};

function human(userId: number, username: string, seat: number): RoomPlayer {
  return {
    id: userId,
    roomId: 1,
    userId,
    username,
    seat,
    team: seat % 2 === 0 ? "teamA" : "teamB",
    isBot: false,
    createdAt: "",
  };
}

function bot(seat: number): RoomPlayer {
  return {
    id: 0,
    roomId: 1,
    userId: 0,
    username: "",
    seat,
    team: seat % 2 === 0 ? "teamA" : "teamB",
    isBot: true,
    createdAt: "",
  };
}

beforeEach(() => {
  mockLeaveRoom.mockResolvedValue(undefined);
  mockJoinRoom.mockResolvedValue(room);
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

describe("RoomPage bot seating", () => {
  it("renders the localized bot name and badge on a bot-occupied seat", async () => {
    useAuthStore.setState({ user: owner, token: "tok" });
    mockGetRoom.mockResolvedValue({ room, players: [human(10, "alice", 0), bot(2)] });

    renderRoomPage();

    await waitFor(() => expect(screen.getByTestId("player-seat-2")).toBeInTheDocument());
    expect(screen.getByTestId("bot-name-2")).toHaveTextContent("Bot 3");
    expect(screen.getByTestId("bot-badge-2")).toBeInTheDocument();
    // The avatar renders the bot glyph, not a name initial.
    expect(
      within(screen.getByTestId("player-seat-2")).getByTestId("avatar-icon"),
    ).toBeInTheDocument();
  });

  it("owner sees the add-bot affordance on empty seats and it fires the mutation", async () => {
    useAuthStore.setState({ user: owner, token: "tok" });
    mockGetRoom.mockResolvedValue({ room, players: [human(10, "alice", 0)] });
    mockAddBot.mockResolvedValue({ players: [human(10, "alice", 0), bot(1)] });

    renderRoomPage();

    await waitFor(() => expect(screen.getByTestId("add-bot-seat-1")).toBeInTheDocument());
    await userEvent.click(screen.getByTestId("add-bot-seat-1"));

    await waitFor(() => expect(mockAddBot).toHaveBeenCalledWith(1, 1));
    // Players from the response land in the store — the bot tile renders.
    expect(await screen.findByTestId("bot-name-1")).toHaveTextContent("Bot 2");
  });

  it("non-owner sees no add-bot or remove-bot affordances", async () => {
    useAuthStore.setState({ user: guest, token: "tok" });
    mockGetRoom.mockResolvedValue({
      room,
      players: [human(10, "alice", 0), human(20, "bob", 1), bot(2)],
    });

    renderRoomPage();

    await waitFor(() => expect(screen.getByTestId("player-seat-2")).toBeInTheDocument());
    expect(screen.queryByTestId("add-bot-seat-3")).not.toBeInTheDocument();
    expect(screen.queryByTestId("remove-bot-2")).not.toBeInTheDocument();
  });

  it("owner removes a bot via the confirm dialog", async () => {
    useAuthStore.setState({ user: owner, token: "tok" });
    mockGetRoom.mockResolvedValue({ room, players: [human(10, "alice", 0), bot(2)] });
    mockRemoveBot.mockResolvedValue({ players: [human(10, "alice", 0)] });

    renderRoomPage();

    await waitFor(() => expect(screen.getByTestId("remove-bot-2")).toBeInTheDocument());
    await userEvent.click(screen.getByTestId("remove-bot-2"));

    // Confirm dialog opens with the localized bot name, then confirm.
    expect(await screen.findByTestId("remove-bot-dialog-title")).toHaveTextContent("Bot 3");
    await userEvent.click(screen.getByTestId("remove-bot-confirm"));

    await waitFor(() => expect(mockRemoveBot).toHaveBeenCalledWith(1, 2));
  });

  it("bot tiles never show kick or promote controls", async () => {
    useAuthStore.setState({ user: owner, token: "tok" });
    mockGetRoom.mockResolvedValue({ room, players: [human(10, "alice", 0), bot(2)] });

    renderRoomPage();

    await waitFor(() => expect(screen.getByTestId("player-seat-2")).toBeInTheDocument());
    expect(screen.queryByTestId("kick-player-2")).not.toBeInTheDocument();
    expect(screen.queryByTestId("promote-seat-2")).not.toBeInTheDocument();
  });

  it("owner swaps a bot to an empty seat through the existing swap flow", async () => {
    useAuthStore.setState({ user: owner, token: "tok" });
    mockGetRoom.mockResolvedValue({ room, players: [human(10, "alice", 0), bot(1)] });
    mockSwapSeats.mockResolvedValue({ players: [human(10, "alice", 0), bot(3)] });

    renderRoomPage();

    // Click the bot tile (swap source), then an empty seat (target).
    await waitFor(() => expect(screen.getByTestId("player-seat-1")).toBeInTheDocument());
    await userEvent.click(screen.getByTestId("player-seat-1"));
    await userEvent.click(screen.getByTestId("player-seat-3"));

    await waitFor(() => expect(mockSwapSeats).toHaveBeenCalledWith(1, 1, 3));
  });

  it("owner swaps themselves with a bot (own seat is a valid target for a bot source)", async () => {
    useAuthStore.setState({ user: owner, token: "tok" });
    mockGetRoom.mockResolvedValue({ room, players: [human(10, "alice", 0), bot(1)] });
    mockSwapSeats.mockResolvedValue({ players: [human(10, "alice", 1), bot(0)] });

    renderRoomPage();

    await waitFor(() => expect(screen.getByTestId("player-seat-1")).toBeInTheDocument());
    await userEvent.click(screen.getByTestId("player-seat-1")); // bot = source
    await userEvent.click(screen.getByTestId("player-seat-0")); // own seat = target

    await waitFor(() => expect(mockSwapSeats).toHaveBeenCalledWith(1, 1, 0));
  });

  it("owner swaps another human with a bot (human ↔ bot)", async () => {
    useAuthStore.setState({ user: owner, token: "tok" });
    mockGetRoom.mockResolvedValue({
      room: { ...room, playerCount: 2 },
      players: [human(10, "alice", 0), human(20, "bob", 2), bot(1)],
    });
    mockSwapSeats.mockResolvedValue({
      players: [human(10, "alice", 0), human(20, "bob", 1), bot(2)],
    });

    renderRoomPage();

    await waitFor(() => expect(screen.getByTestId("player-seat-1")).toBeInTheDocument());
    await userEvent.click(screen.getByTestId("player-seat-1")); // bot = source
    await userEvent.click(screen.getByTestId("player-seat-2")); // bob = target

    await waitFor(() => expect(mockSwapSeats).toHaveBeenCalledWith(1, 1, 2));
  });

  it("owner swaps a human onto a bot's seat (human source, bot target)", async () => {
    useAuthStore.setState({ user: owner, token: "tok" });
    mockGetRoom.mockResolvedValue({
      room: { ...room, playerCount: 2 },
      players: [human(10, "alice", 0), human(20, "bob", 2), bot(1)],
    });
    mockSwapSeats.mockResolvedValue({
      players: [human(10, "alice", 0), human(20, "bob", 1), bot(2)],
    });

    renderRoomPage();

    await waitFor(() => expect(screen.getByTestId("player-seat-2")).toBeInTheDocument());
    await userEvent.click(screen.getByTestId("player-seat-2")); // bob = source
    await userEvent.click(screen.getByTestId("player-seat-1")); // bot = target

    await waitFor(() => expect(mockSwapSeats).toHaveBeenCalledWith(1, 2, 1));
  });

  it("start button enables when bots cover the remaining seats", async () => {
    useAuthStore.setState({ user: owner, token: "tok" });
    mockGetRoom.mockResolvedValue({
      room,
      players: [human(10, "alice", 0), bot(1), bot(2), bot(3)],
    });

    renderRoomPage();

    await waitFor(() => expect(screen.getByTestId("start-game")).toBeInTheDocument());
    expect(screen.getByTestId("start-game")).toBeEnabled();
  });

  it("start button stays disabled with an uncovered seat", async () => {
    useAuthStore.setState({ user: owner, token: "tok" });
    mockGetRoom.mockResolvedValue({
      room,
      players: [human(10, "alice", 0), bot(1), bot(2)],
    });

    renderRoomPage();

    await waitFor(() => expect(screen.getByTestId("start-game")).toBeInTheDocument());
    expect(screen.getByTestId("start-game")).toBeDisabled();
  });
});
