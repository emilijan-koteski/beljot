import "@/shared/i18n/i18n";

import { render, screen, waitFor } from "@testing-library/react";
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

/**
 * Every field but name/variant/match-mode/timer/buy-in lives inside the
 * "Advanced settings" accordion (Create Room redesign) and simply doesn't
 * exist in the DOM until it's opened — so any test touching declarations,
 * match end, privacy, the honour gate or new-player policy opens it first.
 */
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
    expect(screen.getByTestId("coin-buy-in-chips")).toBeInTheDocument();
    expect(screen.getByTestId("advanced-settings-toggle")).toBeInTheDocument();
    expect(screen.getByTestId("create-room-button")).toBeInTheDocument();
    expect(screen.getByTestId("cancel-button")).toBeInTheDocument();
  });

  // --- Room name (auto-generated, Create Room redesign) ---

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
    // Extremely unlikely to reroll to the exact same combo twice in a row
    // given the word-list size, but the helper explicitly avoids the repeat.
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
        // Story 9.8: the ungated defaults are always sent, so the wire is
        // explicit about what the modal chose rather than relying on the
        // server's nil-pointer fallbacks.
        minHonor: 0,
        allowNewPlayers: true,
        // Declarations default ON and are always sent, for the same reason.
        declarationsEnabled: true,
        // "Dosta" defaults OFF (finish the hand) and is always sent too.
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
      // raw server message that must NOT leak into the name field
      new FetchError(409, "ALREADY_IN_ROOM", "player is already in a room"),
    );

    renderModal(true);
    await setRoomName(user, "My Room");
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

  // --- Buy-in (preset chips, Create Room redesign) ---

  it("defaults the coin buy-in chip to 500 and shows it in the preview", () => {
    renderModal(true);

    expect(screen.getByTestId("coin-buy-in-chips-500")).toHaveAttribute("aria-checked", "true");
    expect(screen.queryByTestId("coin-buy-in-input")).not.toBeInTheDocument();
    // Preview mirrors the chosen stake.
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

    // The onChange clamps to >= 0 — the field never holds a negative value.
    expect(Number(buyInInput.value)).toBeGreaterThanOrEqual(0);
  });

  it("blocks creating a room with a buy-in above the creator's balance", async () => {
    const user = userEvent.setup();
    setBalance(100);
    renderModal(true);
    await setRoomName(user, "High Roller");

    // 500 is already the default preset, which now exceeds the 100 balance.
    expect(screen.getByTestId("buy-in-error")).toBeInTheDocument();
    expect(screen.getByTestId("create-room-button")).toBeDisabled();

    // Picking the 100 preset instead clears the guard.
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

  // --- Advanced settings accordion (Create Room redesign) ---

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

  // --- Private rooms (Story 9.6) ---

  it("reveals the password field when the private toggle is on", async () => {
    const user = userEvent.setup();
    renderModal(true);
    await openAdvanced(user);

    expect(screen.queryByTestId("room-password-input")).toBeNull();
    await user.click(screen.getByTestId("private-room-toggle-private"));
    expect(screen.getByTestId("room-password-input")).toBeInTheDocument();
  });

  // It has to appear WITH the toggle that revealed it. Rendered last (after the
  // honour gate) a required field grew two fields below the tap that caused it —
  // off screen on a phone — and the host met it as a validation error instead.
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
    // Empty password — submit disabled.
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

  // --- Honor gate (Story 9.8 AC3/AC5, D7; preset tier chips per the Create
  // Room redesign) ---

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
    // 0 reads as a WORD, so the default looks like a deliberate choice rather than
    // an empty field waiting to be filled in.
    expect(screen.getByTestId("min-honor-chips-0")).toHaveTextContent("Anyone");
    expect(screen.queryByTestId("preview-min-honor")).toBeNull();
    expect(screen.queryByTestId("preview-veterans-only")).toBeNull();
  });

  it("labels each threshold chip with the tier it falls in", async () => {
    const user = userEvent.setup();
    renderModal(true);
    await openAdvanced(user);

    // Each chip IS the tier — a host picks a tier, not a digit — which is what
    // replaced the deleted continuous slider and its ticks.
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

    // The self-gate becomes a place you can see rather than an error you trip.
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
    // Independent gates: barring newcomers does not imply a threshold.
    expect(screen.queryByTestId("preview-min-honor")).toBeNull();
  });

  it("submits the configured honor gate", async () => {
    const user = userEvent.setup();
    mockCreateRoom.mockResolvedValueOnce({ id: 4 });
    // The owner must clear their OWN threshold (D7), or the cosmetic self-gate
    // disables submit and nothing is sent — which is the correct behaviour, just
    // not what this test is about.
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

  // D7: the creator is auto-seated, so a gate they cannot pass would eject them
  // from their own room at the first Start. Cosmetic guard; the server re-checks.
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

  // Review 2026-07-30 (PO decision), replacing a test that asserted the opposite:
  // a New Player creator IS score-checked against their own score, mirroring the
  // server. D1's "never score-check a newcomer" is right at the JOIN gate but made
  // D7's self-gate a no-op for exactly the accounts whose score is about to move —
  // a newcomer could set a bar of 95, then graduate below it and be ejected from
  // their own room.
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

  // The cosmetic mirror must not turn "unknown" into "denied". honorIsNewPlayer and
  // honorScoreOrPrior default to suppressed/80 for DISPLAY, which would have blocked
  // a veteran from creating a veterans-only room on a request the server allows.
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

    // The boundary is inclusive on both sides of the wire.
    expect(screen.getByTestId("create-room-button")).toBeEnabled();
  });

  // Story 12.8: the Croatian option was a deliberately disabled placeholder
  // while the variant's rules were still landing. Nothing asserted that it was
  // disabled, so nothing broke when it opened — these two tests are what now
  // hold the enabled state in place.
  it("offers the Croatian variant as a selectable option", async () => {
    const user = userEvent.setup();
    renderModal(true);

    const croatia = screen.getByTestId("variant-segmented-croatia");
    expect(croatia).toBeEnabled();
    expect(croatia).not.toHaveAttribute("aria-disabled");

    // Bitola is the default; selecting Croatian actually moves the selection,
    // which a disabled option could never do.
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

  // --- Declarations toggle ---

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

    // The permanent hint became a "(?)" popover in the redesign — this is the
    // one consequence a player cannot read off the On/Off label, so it must
    // still be reachable in BOTH states, not just discoverable after flipping
    // the toggle first. The popover itself dismisses on an outside click (a
    // real popover, not a hand-rolled one), so it's reopened for each check
    // rather than expected to survive the segmented-control click.
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

    // Cancel goes through handleOpenChange, which is where every field is reset.
    // The test parent never flips `open`, so the modal stays mounted and the
    // reset is directly observable — which is the point: the NEXT owner to open
    // this modal must not inherit "off".
    await user.click(screen.getByTestId("cancel-button"));
    expect(onOpenChange).toHaveBeenCalledWith(false);

    // The accordion itself resets closed too, so reopen it to observe the reset.
    await openAdvanced(user);
    expect(screen.getByTestId("declarations-segmented-on")).toHaveAttribute("aria-checked", "true");
    expect(screen.queryByTestId("preview-no-declarations")).not.toBeInTheDocument();
  });

  // --- Stop-at-target ("dosta") toggle ---
  //
  // The mirror of the declarations block above, with the polarity flipped: the
  // default here is "finish the hand", and the chip appears on the opt-in.

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

    // The popover dismisses on an outside click, so reopen it after the
    // segmented-control click rather than expecting it to survive.
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

    // Order alone is not enough: justify-between would keep this order while
    // pushing the pair back to opposite ends of the footer.
    const footer = screen.getByTestId("create-room-button").parentElement;
    expect(footer?.className).toContain("justify-end");
    expect(footer?.className).not.toContain("justify-between");
  });
});
