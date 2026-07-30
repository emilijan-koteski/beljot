import "@/shared/i18n/i18n";

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BrowserRouter } from "react-router";
import { toast } from "sonner";
import { afterEach, describe, expect, it, vi } from "vitest";

import { JoinByCodeTile } from "@/features/lobby/components/JoinByCodeTile";
import { FetchError } from "@/shared/api/axiosClient";
import { i18n } from "@/shared/i18n/i18n";
import { QueryWrapper } from "@/test-utils";

const mockGetRoomByCode = vi.fn();
const mockJoinRoom = vi.fn();
const mockNavigate = vi.fn();

vi.mock("react-router", async () => {
  const actual = await vi.importActual("react-router");
  return { ...actual, useNavigate: () => mockNavigate };
});

vi.mock("@/shared/api/rooms", () => ({
  getRoomByCode: (...args: unknown[]) => mockGetRoomByCode(...args),
  joinRoom: (...args: unknown[]) => mockJoinRoom(...args),
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

function room(isPrivate: boolean) {
  return {
    room: { id: 9, name: "Code Table", isPrivate },
    players: [],
    returnedUserIds: [],
  };
}

function renderTile() {
  render(
    <QueryWrapper>
      <BrowserRouter>
        <JoinByCodeTile />
      </BrowserRouter>
    </QueryWrapper>,
  );
}

afterEach(() => vi.clearAllMocks());

describe("JoinByCodeTile", () => {
  it("joins a public room directly without prompting for a password", async () => {
    const user = userEvent.setup();
    mockGetRoomByCode.mockResolvedValueOnce(room(false));
    mockJoinRoom.mockResolvedValueOnce({ id: 9 });
    renderTile();

    await user.type(screen.getByTestId("join-by-code-input"), "ABC123");
    await user.click(screen.getByTestId("join-by-code-button"));

    await waitFor(() => expect(mockJoinRoom).toHaveBeenCalledWith(9, undefined));
    expect(screen.queryByTestId("password-prompt-dialog")).toBeNull();
    expect(mockNavigate).toHaveBeenCalledWith("/rooms/9");
  });

  it("prompts for a password when the code resolves to a private room", async () => {
    const user = userEvent.setup();
    mockGetRoomByCode.mockResolvedValueOnce(room(true));
    mockJoinRoom.mockResolvedValueOnce({ id: 9 });
    renderTile();

    await user.type(screen.getByTestId("join-by-code-input"), "PRV123");
    await user.click(screen.getByTestId("join-by-code-button"));

    expect(await screen.findByTestId("password-prompt-dialog")).toBeInTheDocument();
    expect(mockJoinRoom).not.toHaveBeenCalled();

    await user.type(screen.getByTestId("password-prompt-input"), "secret");
    await user.click(screen.getByTestId("password-prompt-submit"));

    await waitFor(() => expect(mockJoinRoom).toHaveBeenCalledWith(9, "secret"));
    expect(mockNavigate).toHaveBeenCalledWith("/rooms/9");
  });

  // Honor gate (Story 9.8 AC9). Join-by-code is the entry point with NO room
  // object in scope on the public path, so HONOR_TOO_LOW must use the param-less
  // generic variant — the precedent room.errors.insufficientCoinsGeneric set.
  it("toasts the generic honor message when the gate rejects a by-code join", async () => {
    const user = userEvent.setup();
    mockGetRoomByCode.mockResolvedValueOnce(room(false));
    mockJoinRoom.mockRejectedValueOnce(new FetchError(409, "HONOR_TOO_LOW", "nope"));
    renderTile();

    await user.type(screen.getByTestId("join-by-code-input"), "ABC123");
    await user.click(screen.getByTestId("join-by-code-button"));

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(i18n.t("room.errors.honorTooLowGeneric")),
    );
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("toasts the veterans-only message when the newcomer gate rejects a by-code join", async () => {
    const user = userEvent.setup();
    mockGetRoomByCode.mockResolvedValueOnce(room(false));
    mockJoinRoom.mockRejectedValueOnce(new FetchError(409, "NEW_PLAYER_NOT_ALLOWED", "nope"));
    renderTile();

    await user.type(screen.getByTestId("join-by-code-input"), "ABC123");
    await user.click(screen.getByTestId("join-by-code-button"));

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(i18n.t("room.errors.newPlayerNotAllowed")),
    );
    expect(mockNavigate).not.toHaveBeenCalled();
  });
});
