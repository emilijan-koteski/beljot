import "@/shared/i18n/i18n";

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BrowserRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useAuthStore } from "@/shared/stores/authStore";
import { QueryWrapper } from "@/test-utils";

import { CreateRoomModal } from "./CreateRoomModal";

function setBalance(walletBalance: number) {
  useAuthStore.setState({
    user: {
      id: 5,
      username: "owner",
      email: "owner@test.dev",
      languagePreference: "en",
      walletBalance,
      loginStreakDays: 0,
      createdAt: "2026-06-18T00:00:00Z",
    },
  });
}

const mockCreateRoom = vi.fn();
const mockNavigate = vi.fn();

vi.mock("@/shared/api/rooms", () => ({
  addBot: vi.fn(),
  removeBot: vi.fn(),
  createRoom: (...args: unknown[]) => mockCreateRoom(...args),
}));

vi.mock("react-router", async () => {
  const actual = await vi.importActual("react-router");
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

function renderModal(open = true) {
  const onOpenChange = vi.fn();
  render(
    <QueryWrapper>
      <BrowserRouter>
        <CreateRoomModal open={open} onOpenChange={onOpenChange} />
      </BrowserRouter>
    </QueryWrapper>,
  );
  return { onOpenChange };
}

describe("CreateRoomModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Ample balance by default so submit-path tests aren't blocked by the
    // create-time affordability guard; individual tests override as needed.
    setBalance(1_000_000);
  });

  it("renders modal with form controls when open", () => {
    renderModal(true);

    expect(screen.getByTestId("room-name-input")).toBeInTheDocument();
    expect(screen.getByTestId("variant-segmented")).toBeInTheDocument();
    expect(screen.getByTestId("match-mode-segmented")).toBeInTheDocument();
    expect(screen.getByTestId("timer-style-segmented")).toBeInTheDocument();
    expect(screen.getByTestId("create-room-button")).toBeInTheDocument();
    expect(screen.getByTestId("cancel-button")).toBeInTheDocument();
  });

  it("disables create button when name is empty", () => {
    renderModal(true);

    const createButton = screen.getByTestId("create-room-button");
    expect(createButton).toBeDisabled();
  });

  it("enables create button when name has text", async () => {
    const user = userEvent.setup();
    renderModal(true);

    const nameInput = screen.getByTestId("room-name-input");
    await user.type(nameInput, "My Room");

    const createButton = screen.getByTestId("create-room-button");
    expect(createButton).not.toBeDisabled();
  });

  it("submits form with correct payload", async () => {
    const user = userEvent.setup();
    mockCreateRoom.mockResolvedValueOnce({
      id: 1,
      name: "Test Room",
      code: "ABC123",
      ownerId: 5,
      ownerUsername: "owner",
      variant: "bitola",
      matchMode: "1001",
      timerStyle: "relaxed",
      timerDurationSeconds: null,
      status: "waiting",
      playerCount: 1,
      createdAt: "2026-04-11T14:30:00Z",
      updatedAt: "2026-04-11T14:30:00Z",
    });

    renderModal(true);

    const nameInput = screen.getByTestId("room-name-input");
    await user.type(nameInput, "Test Room");

    const createButton = screen.getByTestId("create-room-button");
    await user.click(createButton);

    await waitFor(() => {
      expect(mockCreateRoom).toHaveBeenCalledWith({
        name: "Test Room",
        variant: "bitola",
        matchMode: "1001",
        timerStyle: "relaxed",
        timerDurationSeconds: null,
        coinBuyIn: 500,
      });
    });
  });

  it("submits matchMode 501 when the 501 segment is selected", async () => {
    const user = userEvent.setup();
    mockCreateRoom.mockResolvedValueOnce({
      id: 2,
      name: "Quick Room",
      code: "DEF456",
      ownerId: 5,
      ownerUsername: "owner",
      variant: "bitola",
      matchMode: "501",
      timerStyle: "relaxed",
      timerDurationSeconds: null,
      status: "waiting",
      playerCount: 1,
      createdAt: "2026-06-11T14:30:00Z",
      updatedAt: "2026-06-11T14:30:00Z",
    });

    renderModal(true);

    await user.type(screen.getByTestId("room-name-input"), "Quick Room");
    await user.click(screen.getByTestId("match-mode-segmented-501"));
    await user.click(screen.getByTestId("create-room-button"));

    await waitFor(() => {
      expect(mockCreateRoom).toHaveBeenCalledWith({
        name: "Quick Room",
        variant: "bitola",
        matchMode: "501",
        timerStyle: "relaxed",
        timerDurationSeconds: null,
        coinBuyIn: 500,
      });
    });
  });

  it("navigates to room page after successful creation", async () => {
    const user = userEvent.setup();
    mockCreateRoom.mockResolvedValueOnce({
      id: 42,
      name: "Nav Room",
      code: "XYZ789",
      ownerId: 1,
      ownerUsername: "owner",
      variant: "bitola",
      matchMode: "1001",
      timerStyle: "relaxed",
      timerDurationSeconds: null,
      status: "waiting",
      playerCount: 1,
      createdAt: "2026-04-11T14:30:00Z",
      updatedAt: "2026-04-11T14:30:00Z",
    });

    renderModal(true);

    const nameInput = screen.getByTestId("room-name-input");
    await user.type(nameInput, "Nav Room");

    const createButton = screen.getByTestId("create-room-button");
    await user.click(createButton);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/rooms/42");
    });
  });

  it("displays error when API returns ROOM_NAME_TAKEN", async () => {
    const user = userEvent.setup();
    const { FetchError } = await import("@/shared/api/axiosClient");
    mockCreateRoom.mockRejectedValueOnce(
      new FetchError(409, "ROOM_NAME_TAKEN", "a room with this name already exists"),
    );

    renderModal(true);

    const nameInput = screen.getByTestId("room-name-input");
    await user.type(nameInput, "Taken Room");

    const createButton = screen.getByTestId("create-room-button");
    await user.click(createButton);

    await waitFor(() => {
      expect(screen.getByTestId("room-name-error")).toBeInTheDocument();
    });
  });

  it("shows ALREADY_IN_ROOM in the general banner, not under the room name", async () => {
    const user = userEvent.setup();
    const { FetchError } = await import("@/shared/api/axiosClient");
    mockCreateRoom.mockRejectedValueOnce(
      // raw server message that must NOT leak into the name field
      new FetchError(409, "ALREADY_IN_ROOM", "player is already in a room"),
    );

    renderModal(true);
    await user.type(screen.getByTestId("room-name-input"), "My Room");
    await user.click(screen.getByTestId("create-room-button"));

    await waitFor(() => {
      expect(screen.getByTestId("create-room-form-error")).toBeInTheDocument();
    });
    // The misplaced raw error is gone from the name field.
    expect(screen.queryByTestId("room-name-error")).toBeNull();
    expect(screen.getByTestId("create-room-form-error")).not.toHaveTextContent(
      "player is already in a room",
    );
  });

  it("shows timer duration slider when per-move selected and hides for relaxed", async () => {
    const user = userEvent.setup();
    renderModal(true);

    // Initially relaxed — duration slider should not be present
    expect(screen.queryByTestId("timer-duration-slider")).not.toBeInTheDocument();

    // The timer style is a segmented control — pick the "Per move" segment
    await user.click(screen.getByText("Per move"));

    // Timer duration slider should now be present
    expect(screen.getByTestId("timer-duration-slider")).toBeInTheDocument();
  });

  it("defaults the coin buy-in field to 500 and shows it in the preview", () => {
    renderModal(true);

    const buyInInput = screen.getByTestId("coin-buy-in-input") as HTMLInputElement;
    expect(buyInInput.value).toBe("500");
    // Preview mirrors the chosen stake.
    expect(screen.getByTestId("preview-buy-in")).toHaveTextContent("500");
  });

  it("submits the chosen coin buy-in value", async () => {
    const user = userEvent.setup();
    mockCreateRoom.mockResolvedValueOnce({
      id: 7,
      name: "Stake Room",
      code: "STK001",
      ownerId: 5,
      ownerUsername: "owner",
      variant: "bitola",
      matchMode: "1001",
      timerStyle: "relaxed",
      timerDurationSeconds: null,
      coinBuyIn: 1500,
      status: "waiting",
      playerCount: 1,
      createdAt: "2026-06-18T14:30:00Z",
      updatedAt: "2026-06-18T14:30:00Z",
    });

    renderModal(true);
    await user.type(screen.getByTestId("room-name-input"), "Stake Room");

    const buyInInput = screen.getByTestId("coin-buy-in-input");
    await user.clear(buyInInput);
    await user.type(buyInInput, "1500");

    await user.click(screen.getByTestId("create-room-button"));

    await waitFor(() => {
      expect(mockCreateRoom).toHaveBeenCalledWith(
        expect.objectContaining({ name: "Stake Room", coinBuyIn: 1500 }),
      );
    });
  });

  it("clamps a negative buy-in to zero (cosmetic guard; server is authority)", async () => {
    const user = userEvent.setup();
    renderModal(true);

    const buyInInput = screen.getByTestId("coin-buy-in-input") as HTMLInputElement;
    await user.clear(buyInInput);
    await user.type(buyInInput, "-50");

    // The onChange clamps to >= 0 — the field never holds a negative value.
    expect(Number(buyInInput.value)).toBeGreaterThanOrEqual(0);
  });

  it("blocks creating a room with a buy-in above the creator's balance", async () => {
    const user = userEvent.setup();
    setBalance(100);
    renderModal(true);
    await user.type(screen.getByTestId("room-name-input"), "High Roller");

    const buyInInput = screen.getByTestId("coin-buy-in-input");
    await user.clear(buyInInput);
    await user.type(buyInInput, "500");

    expect(screen.getByTestId("buy-in-error")).toBeInTheDocument();
    expect(screen.getByTestId("create-room-button")).toBeDisabled();

    // Lowering the stake to within balance clears the guard.
    await user.clear(buyInInput);
    await user.type(buyInInput, "100");
    expect(screen.queryByTestId("buy-in-error")).toBeNull();
    expect(screen.getByTestId("create-room-button")).toBeEnabled();
  });

  it("calls onOpenChange with false when cancel is clicked", async () => {
    const user = userEvent.setup();
    const { onOpenChange } = renderModal(true);

    const cancelButton = screen.getByTestId("cancel-button");
    await user.click(cancelButton);

    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});
