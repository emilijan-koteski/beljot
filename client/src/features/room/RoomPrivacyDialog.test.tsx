import "@/shared/i18n/i18n";

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { RoomPrivacyDialog } from "@/features/room/RoomPrivacyDialog";
import type { Room } from "@/shared/types/apiTypes";
import { QueryWrapper } from "@/test-utils";

const mockUpdateRoomPrivacy = vi.fn();

vi.mock("@/shared/api/rooms", () => ({
  updateRoomPrivacy: (...args: unknown[]) => mockUpdateRoomPrivacy(...args),
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

const baseRoom: Room = {
  id: 7,
  name: "Owner Table",
  code: "OWN123",
  ownerId: 1,
  ownerUsername: "host",
  variant: "bitola",
  matchMode: "1001",
  timerStyle: "relaxed",
  timerDurationSeconds: null,
  status: "waiting",
  playerCount: 1,
  isQuickPlay: false,
  coinBuyIn: 0,
  isPrivate: false,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

function renderDialog(room: Room = baseRoom) {
  const onClose = vi.fn();
  render(
    <QueryWrapper>
      <RoomPrivacyDialog open room={room} onClose={onClose} />
    </QueryWrapper>,
  );
  return { onClose };
}

afterEach(() => vi.clearAllMocks());

describe("RoomPrivacyDialog", () => {
  it("makes a public room private with a password", async () => {
    const user = userEvent.setup();
    mockUpdateRoomPrivacy.mockResolvedValueOnce({ ...baseRoom, isPrivate: true });
    renderDialog();

    await user.click(screen.getByTestId("room-privacy-mode-private"));
    await user.type(screen.getByTestId("room-privacy-password-input"), "sesame");
    await user.click(screen.getByTestId("room-privacy-submit"));

    await waitFor(() =>
      expect(mockUpdateRoomPrivacy).toHaveBeenCalledWith(7, {
        isPrivate: true,
        password: "sesame",
      }),
    );
  });

  it("reverts a private room to public without a password", async () => {
    const user = userEvent.setup();
    mockUpdateRoomPrivacy.mockResolvedValueOnce({ ...baseRoom, isPrivate: false });
    renderDialog({ ...baseRoom, isPrivate: true });

    // Starts in "private" mode; switch to public and save.
    await user.click(screen.getByTestId("room-privacy-mode-public"));
    await user.click(screen.getByTestId("room-privacy-submit"));

    await waitFor(() =>
      expect(mockUpdateRoomPrivacy).toHaveBeenCalledWith(7, {
        isPrivate: false,
        password: undefined,
      }),
    );
  });

  it("blocks save while private with a too-short password", async () => {
    const user = userEvent.setup();
    renderDialog();

    await user.click(screen.getByTestId("room-privacy-mode-private"));
    expect(screen.getByTestId("room-privacy-submit")).toBeDisabled();
    await user.type(screen.getByTestId("room-privacy-password-input"), "ab");
    expect(screen.getByTestId("room-privacy-submit")).toBeDisabled();
  });
});
