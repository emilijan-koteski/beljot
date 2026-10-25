import "@/shared/i18n/i18n";

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BrowserRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { FetchError } from "@/shared/api/axiosClient";
import { useAuthStore } from "@/shared/stores/authStore";
import { expectActionBeforeStatusQuo, makeUser, QueryWrapper } from "@/test-utils";

import { CreateRoomModal } from "./CreateRoomModal";

function setBalance(walletBalance: number) {
  useAuthStore.setState({
    user: makeUser({
      id: 5,
      username: "owner",
      email: "owner@test.dev",
      walletBalance,
      createdAt: "2026-06-18T00:00:00Z",
    }),
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

/** Opens the Advanced settings accordion, whose fields are not in the DOM until it is expanded. */
async function openAdvanced(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByTestId("advanced-settings-toggle"));
}

/** Replace the pre-filled random name with an exact, known value. */
async function setRoomName(user: ReturnType<typeof userEvent.setup>, name: string) {
  const input = screen.getByTestId("room-name-input");
  await user.clear(input);
  await user.type(input, name);
}

describe("CreateRoomModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setBalance(1_000_000);
  });

  it("renders modal with form controls when open", () => {
    renderModal(true);

    expect(screen.getByTestId("room-name-input")).toBeInTheDocument();
    expect(screen.getByTestId("variant-segmented")).toBeInTheDocument();
    expect(screen.getByTestId("match-mode-segmented")).toBeInTheDocument();
    expect(screen.getByTestId("timer-style-segmented")).toBeInTheDocument();
    expect(screen.getByTestId("coin-buy-in-chips")).toBeInTheDocument();
    expect(screen.getByTestId("advanced-settings-toggle")).toBeInTheDocument();
    expect(screen.getByTestId("create-room-button")).toBeInTheDocument();
    expect(screen.getByTestId("cancel-button")).toBeInTheDocument();
  });

  it("enables the create button by default because the name is pre-filled", () => {
    renderModal(true);

    const nameInput = screen.getByTestId("room-name-input") as HTMLInputElement;
    expect(nameInput.value.trim().length).toBeGreaterThan(0);
    expect(screen.getByTestId("create-room-button")).toBeEnabled();
  });

  it("disables the create button when the name is cleared", async () => {
    const user = userEvent.setup();
    renderModal(true);

    await user.clear(screen.getByTestId("room-name-input"));
    expect(screen.getByTestId("create-room-button")).toBeDisabled();
  });

  it("rerolls the name when the shuffle button is clicked", async () => {
    const user = userEvent.setup();
    renderModal(true);

    const nameInput = screen.getByTestId("room-name-input") as HTMLInputElement;
    const before = nameInput.value;
    await user.click(screen.getByTestId("room-name-shuffle"));

    expect(nameInput.value.trim().length).toBeGreaterThan(0);
    expect(nameInput.value).not.toBe(before);
  });

  it("enables create button when name has text", async () => {
    const user = userEvent.setup();
    renderModal(true);

    await setRoomName(user, "My Room");

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
    await setRoomName(user, "Test Room");

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
        isPrivate: false,
        password: undefined,
        minHonor: 0,
        allowNewPlayers: true,
        declarationsEnabled: true,
        stopAtTarget: false,
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
    await setRoomName(user, "Quick Room");
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
        isPrivate: false,
        password: undefined,
        minHonor: 0,
        allowNewPlayers: true,
        declarationsEnabled: true,
        stopAtTarget: false,
      });
    });
  });

  it("offers 501, 701 and 1001 in that order with 1001 selected", () => {
    renderModal(true);

    const options = within(screen.getByTestId("match-mode-segmented")).getAllByRole("radio");
    expect(options.map((o) => o.textContent)).toEqual(["501 points", "701 points", "1001 points"]);
    expect(screen.getByTestId("match-mode-segmented-1001")).toHaveAttribute("aria-checked", "true");
    expect(screen.getByTestId("match-mode-segmented-701")).toHaveAttribute("aria-checked", "false");
  });

  it("submits matchMode 701 and previews it when the 701 segment is selected", async () => {
    const user = userEvent.setup();
    mockCreateRoom.mockResolvedValueOnce({
      id: 3,
      name: "Medium Room",
      code: "GHJ789",
      ownerId: 5,
      ownerUsername: "owner",
      variant: "bitola",
      matchMode: "701",
      timerStyle: "relaxed",
      timerDurationSeconds: null,
      status: "waiting",
      playerCount: 1,
      createdAt: "2026-10-25T14:30:00Z",
      updatedAt: "2026-10-25T14:30:00Z",
    });

    renderModal(true);
    await setRoomName(user, "Medium Room");
    await user.click(screen.getByTestId("match-mode-segmented-701"));

    expect(screen.getByTestId("match-mode-segmented-701")).toHaveAttribute("aria-checked", "true");
    expect(screen.getByTestId("create-room-preview")).toHaveTextContent("701 pts");

    await user.click(screen.getByTestId("create-room-button"));

    await waitFor(() => {
      expect(mockCreateRoom).toHaveBeenCalledWith({
        name: "Medium Room",
        variant: "bitola",
        matchMode: "701",
        timerStyle: "relaxed",
        timerDurationSeconds: null,
        coinBuyIn: 500,
        isPrivate: false,
        password: undefined,
        minHonor: 0,
        allowNewPlayers: true,
        declarationsEnabled: true,
        stopAtTarget: false,
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
    await setRoomName(user, "Nav Room");

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
    await setRoomName(user, "Taken Room");

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
      new FetchError(409, "ALREADY_IN_ROOM", "player is already in a room"),
    );

    renderModal(true);
    await setRoomName(user, "My Room");
    await user.click(screen.getByTestId("create-room-button"));

    await waitFor(() => {
      expect(screen.getByTestId("create-room-form-error")).toBeInTheDocument();
    });
    expect(screen.queryByTestId("room-name-error")).toBeNull();
    expect(screen.getByTestId("create-room-form-error")).not.toHaveTextContent(
      "player is already in a room",
    );
  });

  it("shows timer duration slider when per-move selected and hides for relaxed", async () => {
    const user = userEvent.setup();
    renderModal(true);

    expect(screen.queryByTestId("timer-duration-slider")).not.toBeInTheDocument();

    await user.click(screen.getByText("Per move"));

    expect(screen.getByTestId("timer-duration-slider")).toBeInTheDocument();
  });

  it("defaults the coin buy-in chip to 500 and shows it in the preview", () => {
    renderModal(true);

    expect(screen.getByTestId("coin-buy-in-chips-500")).toHaveAttribute("aria-checked", "true");
    expect(screen.queryByTestId("coin-buy-in-input")).not.toBeInTheDocument();
    expect(screen.getByTestId("preview-buy-in")).toHaveTextContent("500");
  });

  it("submits a preset buy-in chosen from the chips", async () => {
    const user = userEvent.setup();
    mockCreateRoom.mockResolvedValueOnce({ id: 6 });
    renderModal(true);

    await setRoomName(user, "Free Table");
    await user.click(screen.getByTestId("coin-buy-in-chips-0"));
    expect(screen.getByTestId("preview-buy-in")).toHaveTextContent("No stake");

    await user.click(screen.getByTestId("create-room-button"));

    await waitFor(() => {
      expect(mockCreateRoom).toHaveBeenCalledWith(
        expect.objectContaining({ name: "Free Table", coinBuyIn: 0 }),
      );
    });
  });

  it("reveals a number input and submits the chosen coin buy-in value when Custom is picked", async () => {
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
    await setRoomName(user, "Stake Room");

    await user.click(screen.getByTestId("coin-buy-in-chips-custom"));
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

  it("clamps a negative custom buy-in to zero (cosmetic guard; server is authority)", async () => {
    const user = userEvent.setup();
    renderModal(true);

    await user.click(screen.getByTestId("coin-buy-in-chips-custom"));
    const buyInInput = screen.getByTestId("coin-buy-in-input") as HTMLInputElement;
    await user.clear(buyInInput);
    await user.type(buyInInput, "-50");

    expect(Number(buyInInput.value)).toBeGreaterThanOrEqual(0);
  });

  it("blocks creating a room with a buy-in above the creator's balance", async () => {
    const user = userEvent.setup();
    setBalance(100);
    renderModal(true);
    await setRoomName(user, "High Roller");

    expect(screen.getByTestId("buy-in-error")).toBeInTheDocument();
    expect(screen.getByTestId("create-room-button")).toBeDisabled();

    await user.click(screen.getByTestId("coin-buy-in-chips-100"));
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

  it("keeps declarations, match end, privacy and the honour gate collapsed by default", () => {
    renderModal(true);

    expect(screen.queryByTestId("declarations-segmented")).not.toBeInTheDocument();
    expect(screen.queryByTestId("match-end-segmented")).not.toBeInTheDocument();
    expect(screen.queryByTestId("private-room-toggle")).not.toBeInTheDocument();
    expect(screen.queryByTestId("min-honor-chips")).not.toBeInTheDocument();
    expect(screen.getByTestId("advanced-settings-toggle")).toHaveAttribute(
      "aria-expanded",
      "false",
    );
  });

  it("reveals every advanced field once the accordion is opened", async () => {
    const user = userEvent.setup();
    renderModal(true);

    await openAdvanced(user);

    expect(screen.getByTestId("declarations-segmented")).toBeInTheDocument();
    expect(screen.getByTestId("match-end-segmented")).toBeInTheDocument();
    expect(screen.getByTestId("private-room-toggle")).toBeInTheDocument();
    expect(screen.getByTestId("min-honor-chips")).toBeInTheDocument();
    expect(screen.getByTestId("allow-new-players-toggle")).toBeInTheDocument();
  });

  it("shows no changed-count badge until a default is actually changed", async () => {
    const user = userEvent.setup();
    renderModal(true);

    expect(screen.queryByTestId("advanced-settings-badge")).not.toBeInTheDocument();

    await openAdvanced(user);
    await user.click(screen.getByTestId("declarations-segmented-off"));

    expect(screen.getByTestId("advanced-settings-badge")).toHaveTextContent("1");
  });

  it("reveals the password field when the private toggle is on", async () => {
    const user = userEvent.setup();
    renderModal(true);
    await openAdvanced(user);

    expect(screen.queryByTestId("room-password-input")).toBeNull();
    await user.click(screen.getByTestId("private-room-toggle-private"));
    expect(screen.getByTestId("room-password-input")).toBeInTheDocument();
  });

  it("puts the password field directly under the privacy toggle, above the honour gate", async () => {
    const user = userEvent.setup();
    renderModal(true);
    await openAdvanced(user);

    await user.click(screen.getByTestId("private-room-toggle-private"));

    const toggle = screen.getByTestId("private-room-toggle");
    const password = screen.getByTestId("room-password-input");
    const honorGate = screen.getByTestId("min-honor-chips");

    expect(
      toggle.compareDocumentPosition(password) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      password.compareDocumentPosition(honorGate) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("blocks submit when private is on but the password is too short", async () => {
    const user = userEvent.setup();
    renderModal(true);
    await openAdvanced(user);

    await setRoomName(user, "Secret Room");
    await user.click(screen.getByTestId("private-room-toggle-private"));
    expect(screen.getByTestId("create-room-button")).toBeDisabled();
    await user.type(screen.getByTestId("room-password-input"), "ab");
    expect(screen.getByTestId("create-room-button")).toBeDisabled();
    await user.type(screen.getByTestId("room-password-input"), "cd");
    expect(screen.getByTestId("create-room-button")).toBeEnabled();
  });

  it("includes isPrivate and the password in the create payload", async () => {
    const user = userEvent.setup();
    mockCreateRoom.mockResolvedValueOnce({ id: 3 });
    renderModal(true);
    await openAdvanced(user);

    await setRoomName(user, "Secret Room");
    await user.click(screen.getByTestId("private-room-toggle-private"));
    await user.type(screen.getByTestId("room-password-input"), "hunter2");
    await user.click(screen.getByTestId("create-room-button"));

    await waitFor(() => {
      expect(mockCreateRoom).toHaveBeenCalledWith(
        expect.objectContaining({ isPrivate: true, password: "hunter2" }),
      );
    });
  });

  it("renders the honor-gate controls", async () => {
    const user = userEvent.setup();
    renderModal(true);
    await openAdvanced(user);

    expect(screen.getByTestId("min-honor-chips")).toBeInTheDocument();
    expect(screen.getByTestId("allow-new-players-toggle")).toBeInTheDocument();
  });

  it("defaults to an ungated room and shows neither preview chip", async () => {
    const user = userEvent.setup();
    renderModal(true);
    await openAdvanced(user);

    expect(screen.getByTestId("min-honor-chips-0")).toHaveAttribute("aria-checked", "true");
    expect(screen.getByTestId("min-honor-chips-0")).toHaveTextContent("Anyone");
    expect(screen.queryByTestId("preview-min-honor")).toBeNull();
    expect(screen.queryByTestId("preview-veterans-only")).toBeNull();
  });

  it("labels each threshold chip with the tier it falls in", async () => {
    const user = userEvent.setup();
    renderModal(true);
    await openAdvanced(user);

    const trusted = screen.getByTestId("min-honor-chips-85");
    expect(trusted).toHaveTextContent("Trusted");
    expect(trusted).toHaveTextContent("85+");

    await user.click(trusted);
    expect(trusted).toHaveAttribute("aria-checked", "true");
  });

  it("shows the owner's own honor score in the min-honor info popover", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({
      user: makeUser({ id: 5, username: "owner", honorScore: 96, isNewPlayer: false }),
    });
    renderModal(true);
    await openAdvanced(user);

    await user.click(screen.getByTestId("min-honor-info"));
    expect(screen.getByTestId("min-honor-info-content")).toHaveTextContent("96");
  });

  it("mirrors the honor threshold into the live preview card", async () => {
    const user = userEvent.setup();
    renderModal(true);
    await openAdvanced(user);

    await user.click(screen.getByTestId("min-honor-chips-85"));

    expect(screen.getByTestId("preview-min-honor")).toHaveAttribute("data-min-honor", "85");
    expect(screen.queryByTestId("preview-veterans-only")).toBeNull();
  });

  it("mirrors the veterans-only toggle into the live preview card", async () => {
    const user = userEvent.setup();
    renderModal(true);
    await openAdvanced(user);

    await user.click(screen.getByTestId("allow-new-players-toggle-veterans"));

    expect(screen.getByTestId("preview-veterans-only")).toBeInTheDocument();
    expect(screen.queryByTestId("preview-min-honor")).toBeNull();
  });

  it("submits the configured honor gate", async () => {
    const user = userEvent.setup();
    mockCreateRoom.mockResolvedValueOnce({ id: 4 });
    useAuthStore.setState({
      user: makeUser({ id: 5, username: "owner", honorScore: 96, isNewPlayer: false }),
    });
    renderModal(true);
    await openAdvanced(user);

    await setRoomName(user, "Veterans Table");
    await user.click(screen.getByTestId("min-honor-chips-95"));
    await user.click(screen.getByTestId("allow-new-players-toggle-veterans"));
    await user.click(screen.getByTestId("create-room-button"));

    await waitFor(() => {
      expect(mockCreateRoom).toHaveBeenCalledWith(
        expect.objectContaining({ minHonor: 95, allowNewPlayers: false }),
      );
    });
  });

  it("disables submit when the owner's own honor is below their chosen threshold", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({
      user: makeUser({ id: 5, username: "owner", honorScore: 60, isNewPlayer: false }),
    });
    renderModal(true);
    await openAdvanced(user);

    await setRoomName(user, "Self Locked");
    await user.click(screen.getByTestId("min-honor-chips-95"));

    expect(screen.getByTestId("create-room-button")).toBeDisabled();
    expect(screen.getByTestId("min-honor-error")).toBeInTheDocument();
  });

  it("disables submit when a new-player owner bars new players", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({
      user: makeUser({ id: 5, username: "owner", isNewPlayer: true }),
    });
    renderModal(true);
    await openAdvanced(user);

    await setRoomName(user, "Self Barred");
    await user.click(screen.getByTestId("allow-new-players-toggle-veterans"));

    expect(screen.getByTestId("create-room-button")).toBeDisabled();
    expect(screen.getByTestId("allow-new-players-error")).toBeInTheDocument();
  });

  it("disables submit when a new-player owner sets a bar above their own score", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({
      user: makeUser({ id: 5, username: "owner", honorScore: 80, isNewPlayer: true }),
    });
    renderModal(true);
    await openAdvanced(user);

    await setRoomName(user, "High Bar");
    await user.click(screen.getByTestId("min-honor-chips-95"));

    expect(screen.getByTestId("create-room-button")).toBeDisabled();
    expect(screen.getByTestId("min-honor-error")).toBeInTheDocument();
  });

  it("lets a new-player owner set a bar at or below their own score", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({
      user: makeUser({ id: 5, username: "owner", honorScore: 85, isNewPlayer: true }),
    });
    renderModal(true);
    await openAdvanced(user);

    await setRoomName(user, "Fair Bar");
    await user.click(screen.getByTestId("min-honor-chips-85"));

    expect(screen.getByTestId("create-room-button")).toBeEnabled();
    expect(screen.queryByTestId("min-honor-error")).toBeNull();
  });

  it("does not gate the owner when the auth envelope carries no honor", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({
      user: makeUser({ id: 5, username: "owner", honorScore: undefined, isNewPlayer: undefined }),
    });
    renderModal(true);
    await openAdvanced(user);

    await setRoomName(user, "Unknown Honor");
    await user.click(screen.getByTestId("allow-new-players-toggle-veterans"));

    expect(screen.getByTestId("create-room-button")).toBeEnabled();
    expect(screen.queryByTestId("allow-new-players-error")).toBeNull();
  });

  it("keeps submit enabled for an owner who sets their threshold to exactly their own score", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({
      user: makeUser({ id: 5, username: "owner", honorScore: 95, isNewPlayer: false }),
    });
    renderModal(true);
    await openAdvanced(user);

    await setRoomName(user, "Fine");
    await user.click(screen.getByTestId("min-honor-chips-95"));

    expect(screen.getByTestId("create-room-button")).toBeEnabled();
  });

  it("offers the Croatian variant as a selectable option", async () => {
    const user = userEvent.setup();
    renderModal(true);

    const croatia = screen.getByTestId("variant-segmented-croatia");
    expect(croatia).toBeEnabled();
    expect(croatia).not.toHaveAttribute("aria-disabled");

    expect(screen.getByTestId("variant-segmented-bitola")).toHaveAttribute("aria-checked", "true");
    await user.click(croatia);
    expect(croatia).toHaveAttribute("aria-checked", "true");
    expect(screen.getByTestId("variant-segmented-bitola")).toHaveAttribute("aria-checked", "false");
  });

  it("submits variant croatia when the Croatian option is chosen", async () => {
    const user = userEvent.setup();
    mockCreateRoom.mockResolvedValueOnce({
      id: 9,
      name: "Hrvatska Ekipa",
      code: "CRO001",
      ownerId: 5,
      ownerUsername: "owner",
      variant: "croatia",
      matchMode: "1001",
      timerStyle: "relaxed",
      timerDurationSeconds: null,
      coinBuyIn: 500,
      status: "waiting",
      playerCount: 1,
      createdAt: "2026-08-20T14:30:00Z",
      updatedAt: "2026-08-20T14:30:00Z",
    });

    renderModal(true);
    await setRoomName(user, "Hrvatska Ekipa");
    await user.click(screen.getByTestId("variant-segmented-croatia"));
    await user.click(screen.getByTestId("create-room-button"));

    await waitFor(() => {
      expect(mockCreateRoom).toHaveBeenCalledWith(
        expect.objectContaining({ name: "Hrvatska Ekipa", variant: "croatia" }),
      );
    });
  });

  it("defaults the declarations toggle to ON and shows no preview chip", async () => {
    const user = userEvent.setup();
    renderModal(true);
    await openAdvanced(user);

    expect(screen.getByTestId("declarations-segmented")).toBeInTheDocument();
    expect(screen.getByTestId("declarations-segmented-on")).toHaveAttribute("aria-checked", "true");
    expect(screen.queryByTestId("preview-no-declarations")).not.toBeInTheDocument();
  });

  it("mirrors a declarations-off choice into the live preview card", async () => {
    const user = userEvent.setup();
    renderModal(true);
    await openAdvanced(user);

    await user.click(screen.getByTestId("declarations-segmented-off"));

    expect(await screen.findByTestId("preview-no-declarations")).toBeInTheDocument();
  });

  it("submits declarationsEnabled false when the off segment is selected", async () => {
    const user = userEvent.setup();
    mockCreateRoom.mockResolvedValueOnce({
      id: 11,
      name: "Bez Zvanja",
      code: "NOD001",
      ownerId: 5,
      ownerUsername: "owner",
      variant: "bitola",
      matchMode: "1001",
      timerStyle: "relaxed",
      timerDurationSeconds: null,
      coinBuyIn: 500,
      status: "waiting",
      playerCount: 1,
      createdAt: "2026-08-24T09:00:00Z",
      updatedAt: "2026-08-24T09:00:00Z",
    });

    renderModal(true);
    await openAdvanced(user);
    await setRoomName(user, "Bez Zvanja");
    await user.click(screen.getByTestId("declarations-segmented-off"));
    await user.click(screen.getByTestId("create-room-button"));

    await waitFor(() => {
      expect(mockCreateRoom).toHaveBeenCalledWith(
        expect.objectContaining({ name: "Bez Zvanja", declarationsEnabled: false }),
      );
    });
  });

  it("always explains that Belote goes off with the declarations", async () => {
    const user = userEvent.setup();
    renderModal(true);
    await openAdvanced(user);

    await user.click(screen.getByTestId("declarations-info-toggle"));
    expect(screen.getByText(/no declarations and no Belote/i)).toBeInTheDocument();

    await user.click(screen.getByTestId("declarations-segmented-off"));
    await user.click(screen.getByTestId("declarations-info-toggle"));
    expect(screen.getByText(/no declarations and no Belote/i)).toBeInTheDocument();
  });

  it("resets the toggle back to ON when the modal is dismissed", async () => {
    const user = userEvent.setup();
    const { onOpenChange } = renderModal(true);
    await openAdvanced(user);

    await user.click(screen.getByTestId("declarations-segmented-off"));
    expect(await screen.findByTestId("preview-no-declarations")).toBeInTheDocument();

    await user.click(screen.getByTestId("cancel-button"));
    expect(onOpenChange).toHaveBeenCalledWith(false);

    await openAdvanced(user);
    expect(screen.getByTestId("declarations-segmented-on")).toHaveAttribute("aria-checked", "true");
    expect(screen.queryByTestId("preview-no-declarations")).not.toBeInTheDocument();
  });

  it("defaults the match-end control to finishing the hand and shows no preview chip", async () => {
    const user = userEvent.setup();
    renderModal(true);
    await openAdvanced(user);

    expect(screen.getByTestId("match-end-segmented")).toBeInTheDocument();
    expect(screen.getByTestId("match-end-segmented-finish")).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(screen.queryByTestId("preview-stop-at-target")).not.toBeInTheDocument();
  });

  it("mirrors a stop-at-target choice into the live preview card", async () => {
    const user = userEvent.setup();
    renderModal(true);
    await openAdvanced(user);

    await user.click(screen.getByTestId("match-end-segmented-stop"));

    expect(await screen.findByTestId("preview-stop-at-target")).toBeInTheDocument();
  });

  it("submits stopAtTarget true when the stop segment is selected", async () => {
    const user = userEvent.setup();
    mockCreateRoom.mockResolvedValueOnce({
      id: 12,
      name: "Dosta",
      code: "DST001",
      ownerId: 5,
      ownerUsername: "owner",
      variant: "bitola",
      matchMode: "1001",
      timerStyle: "relaxed",
      timerDurationSeconds: null,
      coinBuyIn: 500,
      status: "waiting",
      playerCount: 1,
      createdAt: "2026-08-24T09:00:00Z",
      updatedAt: "2026-08-24T09:00:00Z",
    });

    renderModal(true);
    await openAdvanced(user);
    await setRoomName(user, "Dosta");
    await user.click(screen.getByTestId("match-end-segmented-stop"));
    await user.click(screen.getByTestId("create-room-button"));

    await waitFor(() => {
      expect(mockCreateRoom).toHaveBeenCalledWith(
        expect.objectContaining({ name: "Dosta", stopAtTarget: true, declarationsEnabled: true }),
      );
    });
  });

  it("always explains that neither end-of-hand bonus is awarded", async () => {
    const user = userEvent.setup();
    renderModal(true);
    await openAdvanced(user);

    await user.click(screen.getByTestId("match-end-info-toggle"));
    expect(screen.getByText(/no last-trick or capot bonus/i)).toBeInTheDocument();

    await user.click(screen.getByTestId("match-end-segmented-stop"));
    await user.click(screen.getByTestId("match-end-info-toggle"));
    expect(screen.getByText(/no last-trick or capot bonus/i)).toBeInTheDocument();
  });

  it("resets the match-end control back to finish-the-hand when the modal is dismissed", async () => {
    const user = userEvent.setup();
    const { onOpenChange } = renderModal(true);
    await openAdvanced(user);

    await user.click(screen.getByTestId("match-end-segmented-stop"));
    expect(await screen.findByTestId("preview-stop-at-target")).toBeInTheDocument();

    await user.click(screen.getByTestId("cancel-button"));
    expect(onOpenChange).toHaveBeenCalledWith(false);

    await openAdvanced(user);
    expect(screen.getByTestId("match-end-segmented-finish")).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(screen.queryByTestId("preview-stop-at-target")).not.toBeInTheDocument();
  });

  it("surfaces the server's INVALID_MIN_HONOR rejection in the form banner", async () => {
    const user = userEvent.setup();
    mockCreateRoom.mockRejectedValueOnce(new FetchError(400, "INVALID_MIN_HONOR", "bad"));
    renderModal(true);
    await setRoomName(user, "Bad Gate");
    await user.click(screen.getByTestId("create-room-button"));

    expect(await screen.findByTestId("create-room-form-error")).toBeInTheDocument();
  });

  it("surfaces the server's own-gate rejections in the form banner", async () => {
    const user = userEvent.setup();
    mockCreateRoom.mockRejectedValueOnce(new FetchError(409, "NEW_PLAYER_NOT_ALLOWED", "nope"));
    renderModal(true);
    await setRoomName(user, "Server Says No");
    await user.click(screen.getByTestId("create-room-button"));

    expect(await screen.findByTestId("create-room-form-error")).toBeInTheDocument();
  });
});

describe("CreateRoomModal footer order", () => {
  it("places create before cancel so the action sits left of the status quo", () => {
    renderModal();
    expectActionBeforeStatusQuo("create-room-button", "cancel-button");

    const footer = screen.getByTestId("create-room-button").parentElement;
    expect(footer?.className).toContain("justify-end");
    expect(footer?.className).not.toContain("justify-between");
  });
});
