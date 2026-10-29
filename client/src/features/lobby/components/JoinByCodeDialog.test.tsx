import "@/shared/i18n/i18n";

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BrowserRouter } from "react-router";
import { toast } from "sonner";
import { afterEach, describe, expect, it, vi } from "vitest";

import { JoinByCodeDialog } from "@/features/lobby/components/JoinByCodeDialog";
import { FetchError } from "@/shared/api/axiosClient";
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

function renderDialog(onClose = vi.fn()) {
  render(
    <QueryWrapper>
      <BrowserRouter>
        <JoinByCodeDialog open onClose={onClose} />
      </BrowserRouter>
    </QueryWrapper>,
  );
  return onClose;
}

afterEach(() => vi.clearAllMocks());

describe("JoinByCodeDialog", () => {
  it("joins a public room and closes the sheet", async () => {
    const user = userEvent.setup();
    mockGetRoomByCode.mockResolvedValueOnce(room(false));
    mockJoinRoom.mockResolvedValueOnce({ id: 9 });
    const onClose = renderDialog();

    expect(screen.getByTestId("join-by-code-button")).toBeDisabled();
    await user.type(screen.getByTestId("join-by-code-input"), "abc123");
    await user.click(screen.getByTestId("join-by-code-button"));

    await waitFor(() => expect(mockJoinRoom).toHaveBeenCalledWith(9, undefined));
    expect(mockNavigate).toHaveBeenCalledWith("/rooms/9");
    expect(onClose).toHaveBeenCalled();
  });

  it("closes the sheet and asks for the password of a private room", async () => {
    const user = userEvent.setup();
    mockGetRoomByCode.mockResolvedValueOnce(room(true));
    mockJoinRoom.mockResolvedValueOnce({ id: 9 });
    const onClose = renderDialog();

    await user.type(screen.getByTestId("join-by-code-input"), "PRV123");
    await user.click(screen.getByTestId("join-by-code-button"));

    expect(await screen.findByTestId("password-prompt-dialog")).toBeInTheDocument();
    expect(onClose).toHaveBeenCalled();
    await user.type(screen.getByTestId("password-prompt-input"), "secret");
    await user.click(screen.getByTestId("password-prompt-submit"));

    await waitFor(() => expect(mockJoinRoom).toHaveBeenCalledWith(9, "secret"));
  });

  it("stays open and toasts when the code finds no room", async () => {
    const user = userEvent.setup();
    mockGetRoomByCode.mockRejectedValueOnce(new FetchError(404, "ROOM_NOT_FOUND", "nope"));
    const onClose = renderDialog();

    await user.type(screen.getByTestId("join-by-code-input"), "NOP123");
    await user.click(screen.getByTestId("join-by-code-button"));

    await waitFor(() => expect(toast.error).toHaveBeenCalled());
    expect(onClose).not.toHaveBeenCalled();
  });
});
