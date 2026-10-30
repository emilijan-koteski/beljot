import "@/shared/i18n/i18n";

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BrowserRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { FetchError } from "@/shared/api/axiosClient";
import { useAuthStore } from "@/shared/stores/authStore";
import {
  expectActionBeforeStatusQuo,
  makeUser,
  mockViewportWidth,
  QueryWrapper,
} from "@/test-utils";

import { CreateRoomModal } from "./CreateRoomModal";

function setOwner(fields: Partial<Parameters<typeof makeUser>[0]> = {}) {
  useAuthStore.setState({
    user: makeUser({
      id: 5,
      username: "owner",
      email: "owner@test.dev",
      walletBalance: 1_000_000,
      createdAt: "2026-06-18T00:00:00Z",
      ...fields,
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

const DEFAULT_PAYLOAD = {
  name: "",
  variant: "croatia",
  matchMode: "1001",
  timerStyle: "per-move",
  timerDurationSeconds: 20,
  coinBuyIn: 500,
  isPrivate: false,
  password: undefined,
  minHonor: 0,
  allowNewPlayers: true,
  declarationsEnabled: true,
  stopAtTarget: false,
  declarationsTransfer: true,
  clockwise: false,
};

function renderModal() {
  const onOpenChange = vi.fn();
  render(
    <QueryWrapper>
      <BrowserRouter>
        <CreateRoomModal open onOpenChange={onOpenChange} />
      </BrowserRouter>
    </QueryWrapper>,
  );
  return { onOpenChange };
}

async function setRoomName(user: ReturnType<typeof userEvent.setup>, name: string) {
  const input = screen.getByTestId("room-name-input");
  await user.clear(input);
  await user.type(input, name);
}

async function submit(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByTestId("create-room-button"));
}

function checked(testId: string) {
  return screen.getByTestId(testId).getAttribute("aria-checked") === "true";
}

function field(segmentedTestId: string) {
  return screen.getByTestId(`field-${segmentedTestId}`);
}

function previewCard(testId: string) {
  return within(screen.getByTestId("create-room-preview")).getByTestId(testId);
}

function previewRule(key: string) {
  return within(screen.getByTestId("create-room-preview-rules")).getByTestId(`rule-${key}`);
}

let restoreViewport: (() => void) | null = null;

function atWidth(width: number) {
  restoreViewport = mockViewportWidth(width);
}

beforeEach(() => {
  vi.clearAllMocks();
  setOwner();
  mockCreateRoom.mockResolvedValue({ id: 1 });
});

afterEach(() => {
  restoreViewport?.();
  restoreViewport = null;
});

describe("CreateRoomModal on a phone", () => {
  it("opens full screen with a back arrow, every setting in view and Create pinned below", () => {
    renderModal();

    expect(screen.getByTestId("create-room-modal")).toHaveAttribute("data-layout", "phone");
    expect(screen.getByRole("heading", { name: "New room" })).toBeInTheDocument();
    expect(screen.getByTestId("cancel-button")).toHaveAccessibleName("Cancel");
    for (const id of [
      "room-name-input",
      "variant-segmented",
      "match-mode-segmented",
      "timer-segmented",
      "coin-buy-in-segmented",
      "declarations-segmented",
      "declarations-transfer-segmented",
      "match-end-segmented",
      "direction-segmented",
      "private-room-toggle",
      "allow-new-players-toggle",
      "min-honor-segmented",
    ]) {
      expect(screen.getByTestId(id)).toBeInTheDocument();
    }
    expect(screen.queryByTestId("create-room-preview")).not.toBeInTheDocument();
  });

  it("closes from the back arrow", async () => {
    const user = userEvent.setup();
    const { onOpenChange } = renderModal();

    await user.click(screen.getByTestId("cancel-button"));

    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});

describe("CreateRoomModal on wider screens", () => {
  it("shows a dialog with Create before Cancel and no preview on a tablet", () => {
    atWidth(820);
    renderModal();

    expect(screen.getByTestId("create-room-modal")).toHaveAttribute("data-layout", "tablet");
    expect(screen.getByTestId("cancel-button")).toHaveTextContent("Cancel");
    expectActionBeforeStatusQuo("create-room-button", "cancel-button");
    expect(screen.queryByTestId("create-room-preview")).not.toBeInTheDocument();
  });

  it("adds the live lobby preview beside the form on a desktop", () => {
    atWidth(1280);
    renderModal();

    expect(screen.getByTestId("create-room-modal")).toHaveAttribute("data-layout", "desktop");
    expect(previewCard("room-card-honor")).toHaveTextContent("Any");
    expect(screen.getByTestId("create-room-preview")).toHaveTextContent("Room - owner");
  });
});

describe("CreateRoomModal name", () => {
  it("suggests the host's name and sends it blank so the server names the room", async () => {
    const user = userEvent.setup();
    renderModal();

    expect(screen.getByTestId("room-name-input")).toHaveValue("Room - owner");
    await submit(user);

    await waitFor(() => expect(mockCreateRoom).toHaveBeenCalledWith(DEFAULT_PAYLOAD));
  });

  it("sends a typed name, trimmed", async () => {
    const user = userEvent.setup();
    renderModal();

    await setRoomName(user, "  Evening Table ");
    await submit(user);

    await waitFor(() =>
      expect(mockCreateRoom).toHaveBeenCalledWith(
        expect.objectContaining({ name: "Evening Table" }),
      ),
    );
  });

  it("falls back to the suggested name when the field is cleared", async () => {
    const user = userEvent.setup();
    renderModal();

    await user.clear(screen.getByTestId("room-name-input"));
    expect(screen.getByTestId("room-name-input")).toHaveAttribute("placeholder", "Room - owner");
    await submit(user);

    await waitFor(() =>
      expect(mockCreateRoom).toHaveBeenCalledWith(expect.objectContaining({ name: "" })),
    );
  });

  it("disables Create for a name shorter than three characters", async () => {
    const user = userEvent.setup();
    renderModal();

    await setRoomName(user, "ab");

    expect(screen.getByTestId("create-room-button")).toBeDisabled();
  });

  it("shows ROOM_NAME_TAKEN under the name", async () => {
    const user = userEvent.setup();
    mockCreateRoom.mockRejectedValueOnce(new FetchError(409, "ROOM_NAME_TAKEN", "taken"));
    renderModal();

    await setRoomName(user, "Taken Name");
    await submit(user);

    expect(await screen.findByTestId("room-name-error")).toHaveTextContent("That name is taken");
  });

  it("shows ALREADY_IN_ROOM in the form banner", async () => {
    const user = userEvent.setup();
    mockCreateRoom.mockRejectedValueOnce(new FetchError(409, "ALREADY_IN_ROOM", "in a room"));
    renderModal();

    await submit(user);

    expect(await screen.findByTestId("create-room-form-error")).toBeInTheDocument();
    expect(screen.queryByTestId("room-name-error")).not.toBeInTheDocument();
  });

  it("opens the new room after creating it", async () => {
    const user = userEvent.setup();
    mockCreateRoom.mockResolvedValueOnce({ id: 42 });
    const { onOpenChange } = renderModal();

    await submit(user);

    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith("/rooms/42"));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});

describe("CreateRoomModal game settings", () => {
  it("defaults to the Croatian variant and 1001, and submits Bitola and 501 when chosen", async () => {
    const user = userEvent.setup();
    renderModal();

    expect(checked("variant-segmented-croatia")).toBe(true);
    expect(checked("match-mode-segmented-1001")).toBe(true);
    await user.click(screen.getByTestId("variant-segmented-bitola"));
    await user.click(screen.getByTestId("match-mode-segmented-501"));
    await submit(user);

    await waitFor(() =>
      expect(mockCreateRoom).toHaveBeenCalledWith(
        expect.objectContaining({ variant: "bitola", matchMode: "501" }),
      ),
    );
  });

  it("offers 10 to 90 seconds with relaxed last, 20 seconds by default", () => {
    renderModal();

    const options = within(screen.getByTestId("timer-segmented")).getAllByRole("radio");
    expect(options.map((o) => o.getAttribute("data-testid"))).toEqual([
      "timer-segmented-10",
      "timer-segmented-20",
      "timer-segmented-30",
      "timer-segmented-60",
      "timer-segmented-90",
      "timer-segmented-relaxed",
    ]);
    expect(checked("timer-segmented-20")).toBe(true);
    expect(screen.getByTestId("timer-segmented-relaxed")).toHaveAccessibleName("No timer");
  });

  it.each([
    ["timer-segmented-60", { timerStyle: "per-move", timerDurationSeconds: 60 }],
    ["timer-segmented-relaxed", { timerStyle: "relaxed", timerDurationSeconds: null }],
  ])("submits the move time picked with %s", async (testId, expected) => {
    const user = userEvent.setup();
    renderModal();

    await user.click(screen.getByTestId(testId));
    await submit(user);

    await waitFor(() =>
      expect(mockCreateRoom).toHaveBeenCalledWith(expect.objectContaining(expected)),
    );
  });

  it("submits a preset buy-in", async () => {
    const user = userEvent.setup();
    renderModal();

    expect(checked("coin-buy-in-segmented-500")).toBe(true);
    await user.click(screen.getByTestId("coin-buy-in-segmented-100"));
    await submit(user);

    await waitFor(() =>
      expect(mockCreateRoom).toHaveBeenCalledWith(expect.objectContaining({ coinBuyIn: 100 })),
    );
  });

  it("takes a buy-in typed into the last cell of the row, digits only, and deselects the presets", async () => {
    const user = userEvent.setup();
    renderModal();

    await user.type(screen.getByTestId("coin-buy-in-input"), "7a5-0");

    expect(screen.getByTestId("coin-buy-in-input")).toHaveValue("750");
    expect(checked("coin-buy-in-segmented-500")).toBe(false);
    await submit(user);
    await waitFor(() =>
      expect(mockCreateRoom).toHaveBeenCalledWith(expect.objectContaining({ coinBuyIn: 750 })),
    );
  });

  it("returns to the preset when the typed buy-in is cleared or a preset is picked", async () => {
    const user = userEvent.setup();
    renderModal();

    await user.type(screen.getByTestId("coin-buy-in-input"), "900");
    await user.clear(screen.getByTestId("coin-buy-in-input"));
    expect(checked("coin-buy-in-segmented-500")).toBe(true);

    await user.type(screen.getByTestId("coin-buy-in-input"), "900");
    await user.click(screen.getByTestId("coin-buy-in-segmented-1000"));
    expect(screen.getByTestId("coin-buy-in-input")).toHaveValue("");
    expect(checked("coin-buy-in-segmented-1000")).toBe(true);
  });

  it("blocks a buy-in above the host's balance", async () => {
    const user = userEvent.setup();
    setOwner({ walletBalance: 300 });
    renderModal();

    expect(screen.getByTestId("buy-in-error")).toBeInTheDocument();
    expect(screen.getByTestId("create-room-button")).toBeDisabled();
    await user.click(screen.getByTestId("coin-buy-in-segmented-100"));
    expect(screen.queryByTestId("buy-in-error")).not.toBeInTheDocument();
    expect(screen.getByTestId("create-room-button")).toBeEnabled();
  });
});

describe("CreateRoomModal rules", () => {
  it.each([
    [
      "declarations-segmented-off",
      "declarations-segmented",
      { declarationsEnabled: false, declarationsTransfer: true },
    ],
    [
      "declarations-transfer-segmented-void",
      "declarations-transfer-segmented",
      { declarationsTransfer: false },
    ],
    ["match-end-segmented-stop", "match-end-segmented", { stopAtTarget: true }],
    ["direction-segmented-clockwise", "direction-segmented", { clockwise: true }],
    ["allow-new-players-toggle-veterans", "allow-new-players-toggle", { allowNewPlayers: false }],
  ])("marks %s as changed from the standard and submits it", async (option, control, expected) => {
    const user = userEvent.setup();
    renderModal();

    expect(field(control)).not.toHaveAttribute("data-changed");
    await user.click(screen.getByTestId(option));
    expect(field(control)).toHaveAttribute("data-changed", "true");
    await submit(user);

    await waitFor(() =>
      expect(mockCreateRoom).toHaveBeenCalledWith(expect.objectContaining(expected)),
    );
  });

  it("hides declarations on a fall while declarations are off", async () => {
    const user = userEvent.setup();
    renderModal();

    await user.click(screen.getByTestId("declarations-transfer-segmented-void"));
    await user.click(screen.getByTestId("declarations-segmented-off"));

    expect(screen.queryByTestId("declarations-transfer-segmented")).not.toBeInTheDocument();
  });

  it("names the options in the shared rule words", () => {
    renderModal();

    expect(screen.getByTestId("match-end-segmented-finish")).toHaveTextContent("Finish the hand");
    expect(screen.getByTestId("direction-segmented-counterClockwise")).toHaveTextContent(
      "To the right",
    );
    expect(screen.getByTestId("private-room-toggle-public")).toHaveTextContent("Open");
  });

  it("mirrors changed rules into the desktop preview", async () => {
    const user = userEvent.setup();
    atWidth(1280);
    renderModal();

    await user.click(screen.getByTestId("match-end-segmented-stop"));
    await user.click(screen.getByTestId("direction-segmented-clockwise"));
    await user.click(screen.getByTestId("allow-new-players-toggle-veterans"));

    expect(previewRule("end")).toHaveAttribute("data-changed", "true");
    expect(previewRule("direction")).toHaveTextContent("To the left");
    expect(previewRule("newPlayers")).toHaveTextContent("Veterans only");
  });

  it("resets every setting when dismissed", async () => {
    const user = userEvent.setup();
    renderModal();

    await setRoomName(user, "Something");
    await user.click(screen.getByTestId("variant-segmented-bitola"));
    await user.click(screen.getByTestId("timer-segmented-relaxed"));
    await user.click(screen.getByTestId("declarations-segmented-off"));
    await user.click(screen.getByTestId("direction-segmented-clockwise"));
    await user.click(screen.getByTestId("cancel-button"));

    expect(screen.getByTestId("room-name-input")).toHaveValue("Room - owner");
    expect(checked("variant-segmented-croatia")).toBe(true);
    expect(checked("timer-segmented-20")).toBe(true);
    expect(checked("declarations-segmented-on")).toBe(true);
    expect(checked("direction-segmented-counterClockwise")).toBe(true);
  });
});

describe("CreateRoomModal room settings", () => {
  it("asks for a password under Room type and blocks Create until it is long enough", async () => {
    const user = userEvent.setup();
    renderModal();

    await user.click(screen.getByTestId("private-room-toggle-private"));
    expect(field("private-room-toggle")).toHaveAttribute("data-changed", "true");
    await user.type(screen.getByTestId("room-password-input"), "abc");
    expect(screen.getByTestId("create-room-button")).toBeDisabled();

    await user.type(screen.getByTestId("room-password-input"), "d");
    expect(screen.getByTestId("create-room-button")).toBeEnabled();
    await submit(user);
    await waitFor(() =>
      expect(mockCreateRoom).toHaveBeenCalledWith(
        expect.objectContaining({ isPrivate: true, password: "abcd" }),
      ),
    );
  });

  it("offers any honor and each honor tier floor, naming the tier for screen readers", () => {
    renderModal();

    expect(checked("min-honor-segmented-0")).toBe(true);
    expect(screen.getByTestId("min-honor-segmented-0")).toHaveTextContent("Any");
    expect(screen.getByTestId("min-honor-segmented-85")).toHaveTextContent("85+");
    expect(screen.getByTestId("min-honor-segmented-85")).toHaveAccessibleName(/Trusted/);
  });

  it("shows the host's own honor in the minimum-honor note", async () => {
    const user = userEvent.setup();
    setOwner({ honorScore: 96, isNewPlayer: false });
    renderModal();

    await user.click(screen.getByTestId("min-honor-info"));

    expect(screen.getByTestId("min-honor-info-content")).toHaveTextContent("96");
  });

  it("submits the configured honor gate", async () => {
    const user = userEvent.setup();
    setOwner({ honorScore: 96, isNewPlayer: false });
    renderModal();

    await user.click(screen.getByTestId("min-honor-segmented-95"));
    await user.click(screen.getByTestId("allow-new-players-toggle-veterans"));
    await submit(user);

    await waitFor(() =>
      expect(mockCreateRoom).toHaveBeenCalledWith(
        expect.objectContaining({ minHonor: 95, allowNewPlayers: false }),
      ),
    );
  });

  it.each([
    [
      "their honor is below the floor",
      { honorScore: 60, isNewPlayer: false },
      "min-honor-segmented-95",
      "min-honor-error",
    ],
    [
      "a new player bars new players",
      { isNewPlayer: true },
      "allow-new-players-toggle-veterans",
      "allow-new-players-error",
    ],
    [
      "a new player sets a floor above their score",
      { honorScore: 80, isNewPlayer: true },
      "min-honor-segmented-95",
      "min-honor-error",
    ],
  ])("blocks Create when %s", async (_case, owner, option, errorId) => {
    const user = userEvent.setup();
    setOwner(owner);
    renderModal();

    await user.click(screen.getByTestId(option));

    expect(screen.getByTestId("create-room-button")).toBeDisabled();
    expect(screen.getByTestId(errorId)).toBeInTheDocument();
  });

  it.each([
    [
      "the floor equals their honor",
      { honorScore: 95, isNewPlayer: false },
      "min-honor-segmented-95",
    ],
    [
      "a new player's floor is at their score",
      { honorScore: 85, isNewPlayer: true },
      "min-honor-segmented-85",
    ],
    [
      "the auth envelope carries no honor",
      { honorScore: undefined, isNewPlayer: undefined },
      "allow-new-players-toggle-veterans",
    ],
  ])("keeps Create enabled when %s", async (_case, owner, option) => {
    const user = userEvent.setup();
    setOwner(owner);
    renderModal();

    await user.click(screen.getByTestId(option));

    expect(screen.getByTestId("create-room-button")).toBeEnabled();
  });

  it.each([
    ["INVALID_MIN_HONOR", "Minimum honor must be between 0 and 100."],
    ["NEW_PLAYER_NOT_ALLOWED", "You're still a new player"],
  ])("shows the server's %s rejection in the form banner", async (code, text) => {
    const user = userEvent.setup();
    mockCreateRoom.mockRejectedValueOnce(new FetchError(400, code, code));
    renderModal();

    await submit(user);

    expect(await screen.findByTestId("create-room-form-error")).toHaveTextContent(text);
  });
});
