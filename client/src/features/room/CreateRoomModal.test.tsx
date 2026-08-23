import "@/shared/i18n/i18n";

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
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
        isPrivate: false,
        password: undefined,
        // Story 9.8: the ungated defaults are always sent, so the wire is
        // explicit about what the modal chose rather than relying on the
        // server's nil-pointer fallbacks.
        minHonor: 0,
        allowNewPlayers: true,
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
        isPrivate: false,
        password: undefined,
        // Story 9.8: the ungated defaults are always sent, so the wire is
        // explicit about what the modal chose rather than relying on the
        // server's nil-pointer fallbacks.
        minHonor: 0,
        allowNewPlayers: true,
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

  // --- Private rooms (Story 9.6) ---

  it("reveals the password field when the private toggle is on", async () => {
    const user = userEvent.setup();
    renderModal(true);

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

    await user.click(screen.getByTestId("private-room-toggle-private"));

    const toggle = screen.getByTestId("private-room-toggle");
    const password = screen.getByTestId("room-password-input");
    const honorGate = screen.getByTestId("min-honor-input");

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

    await user.type(screen.getByTestId("room-name-input"), "Secret Room");
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

    await user.type(screen.getByTestId("room-name-input"), "Secret Room");
    await user.click(screen.getByTestId("private-room-toggle-private"));
    await user.type(screen.getByTestId("room-password-input"), "hunter2");
    await user.click(screen.getByTestId("create-room-button"));

    await waitFor(() => {
      expect(mockCreateRoom).toHaveBeenCalledWith(
        expect.objectContaining({ isPrivate: true, password: "hunter2" }),
      );
    });
  });
  // --- Honor gate (Story 9.8 AC3/AC5, D7; slider per the honour redesign R5) ---

  /**
   * Drive the honour threshold. It is a range input now, not a text field, so
   * clear()/type() do not apply — a range is not an editable element. fireEvent
   * .change is the standard way to move a slider in jsdom, and it exercises the
   * same onChange the thumb and the arrow keys do.
   */
  function setMinHonor(value: number) {
    fireEvent.change(screen.getByTestId("min-honor-input"), { target: { value: String(value) } });
  }

  it("renders the honor-gate controls", () => {
    renderModal(true);

    expect(screen.getByTestId("min-honor-input")).toBeInTheDocument();
    expect(screen.getByTestId("allow-new-players-toggle")).toBeInTheDocument();
  });

  it("defaults to an ungated room and shows neither preview chip", () => {
    renderModal(true);

    // Read .value off the element rather than via toHaveValue: for type="range"
    // jest-dom reports a string, so a numeric matcher is misleading here.
    expect((screen.getByTestId("min-honor-input") as HTMLInputElement).value).toBe("0");
    // 0 reads as a WORD, so the default looks like a deliberate choice rather than
    // an empty field waiting to be filled in.
    expect(screen.getByTestId("min-honor-input-value")).toHaveTextContent("Anyone");
    expect(screen.queryByTestId("preview-min-honor")).toBeNull();
    expect(screen.queryByTestId("preview-veterans-only")).toBeNull();
  });

  it("labels the chosen threshold with the tier it falls in", () => {
    renderModal(true);

    // The ticks sit on tier boundaries and the caption names the tier, so a host
    // picks a TIER rather than a digit — which is what replaced the deleted hint.
    // The tier word is the readout value's sibling unit label, so anchor there
    // rather than walking a parent chain from the input (which breaks whenever
    // the slider's internal DOM gains a wrapper).
    setMinHonor(85);
    expect(screen.getByTestId("min-honor-input-value")).toHaveAttribute("data-value", "85");
    expect(screen.getByTestId("min-honor-input-value").parentElement).toHaveTextContent("Trusted");
  });

  it("marks the owner's own score on the track", () => {
    useAuthStore.setState({
      user: makeUser({ id: 5, username: "owner", honorScore: 96, isNewPlayer: false }),
    });
    renderModal(true);

    // The self-gate becomes a place you can see rather than an error you trip.
    expect(screen.getByTestId("min-honor-input-marker")).toHaveAttribute("data-value", "96");
  });

  it("mirrors the honor threshold into the live preview card", () => {
    renderModal(true);

    setMinHonor(85);

    expect(screen.getByTestId("preview-min-honor")).toHaveAttribute("data-min-honor", "85");
    expect(screen.queryByTestId("preview-veterans-only")).toBeNull();
  });

  it("mirrors the veterans-only toggle into the live preview card", async () => {
    const user = userEvent.setup();
    renderModal(true);

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

    await user.type(screen.getByTestId("room-name-input"), "Veterans Table");
    setMinHonor(90);
    await user.click(screen.getByTestId("allow-new-players-toggle-veterans"));
    await user.click(screen.getByTestId("create-room-button"));

    await waitFor(() => {
      expect(mockCreateRoom).toHaveBeenCalledWith(
        expect.objectContaining({ minHonor: 90, allowNewPlayers: false }),
      );
    });
  });

  it("clamps the threshold to the 0-100 range the server validates", () => {
    renderModal(true);

    // The slider cannot express an out-of-range value at all — min/max are wired
    // to the same interval the server validates and the rooms.min_honor CHECK
    // enforces. Belt and braces: effectiveMinHonor also clamps in JS.
    const input = screen.getByTestId("min-honor-input") as HTMLInputElement;
    expect(input.min).toBe("0");
    expect(input.max).toBe("100");
    expect(input.step).toBe("5");

    setMinHonor(150);
    expect(screen.getByTestId("min-honor-input-value")).toHaveAttribute("data-value", "100");
  });

  // D7: the creator is auto-seated, so a gate they cannot pass would eject them
  // from their own room at the first Start. Cosmetic guard; the server re-checks.
  it("disables submit when the owner's own honor is below their chosen threshold", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({
      user: makeUser({ id: 5, username: "owner", honorScore: 60, isNewPlayer: false }),
    });
    renderModal(true);

    await user.type(screen.getByTestId("room-name-input"), "Self Locked");
    setMinHonor(95);

    expect(screen.getByTestId("create-room-button")).toBeDisabled();
    expect(screen.getByTestId("min-honor-error")).toBeInTheDocument();
  });

  it("disables submit when a new-player owner bars new players", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({
      user: makeUser({ id: 5, username: "owner", isNewPlayer: true }),
    });
    renderModal(true);

    await user.type(screen.getByTestId("room-name-input"), "Self Barred");
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

    await user.type(screen.getByTestId("room-name-input"), "High Bar");
    setMinHonor(95);

    expect(screen.getByTestId("create-room-button")).toBeDisabled();
    expect(screen.getByTestId("min-honor-error")).toBeInTheDocument();
  });

  it("lets a new-player owner set a bar at or below their own score", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({
      user: makeUser({ id: 5, username: "owner", honorScore: 80, isNewPlayer: true }),
    });
    renderModal(true);

    await user.type(screen.getByTestId("room-name-input"), "Fair Bar");
    setMinHonor(80);

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

    await user.type(screen.getByTestId("room-name-input"), "Unknown Honor");
    await user.click(screen.getByTestId("allow-new-players-toggle-veterans"));

    expect(screen.getByTestId("create-room-button")).toBeEnabled();
    expect(screen.queryByTestId("allow-new-players-error")).toBeNull();
  });

  it("keeps submit enabled for an owner who clears their own threshold", async () => {
    const user = userEvent.setup();
    useAuthStore.setState({
      user: makeUser({ id: 5, username: "owner", honorScore: 95, isNewPlayer: false }),
    });
    renderModal(true);

    await user.type(screen.getByTestId("room-name-input"), "Fine");
    setMinHonor(95);

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
    await user.type(screen.getByTestId("room-name-input"), "Hrvatska Ekipa");
    await user.click(screen.getByTestId("variant-segmented-croatia"));
    await user.click(screen.getByTestId("create-room-button"));

    await waitFor(() => {
      expect(mockCreateRoom).toHaveBeenCalledWith(
        expect.objectContaining({ name: "Hrvatska Ekipa", variant: "croatia" }),
      );
    });
  });

  it("surfaces the server's INVALID_MIN_HONOR rejection in the form banner", async () => {
    const user = userEvent.setup();
    mockCreateRoom.mockRejectedValueOnce(new FetchError(400, "INVALID_MIN_HONOR", "bad"));
    renderModal(true);

    await user.type(screen.getByTestId("room-name-input"), "Bad Gate");
    await user.click(screen.getByTestId("create-room-button"));

    expect(await screen.findByTestId("create-room-form-error")).toBeInTheDocument();
  });

  it("surfaces the server's own-gate rejections in the form banner", async () => {
    const user = userEvent.setup();
    mockCreateRoom.mockRejectedValueOnce(new FetchError(409, "NEW_PLAYER_NOT_ALLOWED", "nope"));
    renderModal(true);

    await user.type(screen.getByTestId("room-name-input"), "Server Says No");
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
