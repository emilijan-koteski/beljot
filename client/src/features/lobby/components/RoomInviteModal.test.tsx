import "@/shared/i18n/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("sonner", () => ({
  toast: { info: vi.fn(), success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}));

const navigateSpy = vi.fn();
vi.mock("react-router", async () => {
  const actual = await vi.importActual<typeof import("react-router")>("react-router");
  return { ...actual, useNavigate: () => navigateSpy };
});

const joinRoomSpy = vi.fn();
vi.mock("@/shared/api/rooms", () => ({
  joinRoom: (id: number, password?: string) => joinRoomSpy(id, password),
}));

import { toast } from "sonner";

import { FetchError } from "@/shared/api/axiosClient";
import { useRoomStore } from "@/shared/stores/roomStore";
import { makeRoomInvite } from "@/test-utils";

import { RoomInviteModal } from "./RoomInviteModal";

function renderModal() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <RoomInviteModal />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const baseInvite = makeRoomInvite({
  inviteId: 1,
  roomId: 7,
  roomName: "Skopje Ekipa",
  inviterUserId: 3,
  inviterUsername: "ana",
  coinBuyIn: 500,
  isPrivate: false,
  isHostInvite: false,
  // Far enough out that the auto-dismiss timer never fires during a test.
  expiresAt: new Date(Date.now() + 60_000).toISOString(),
});

describe("RoomInviteModal", () => {
  beforeEach(() => {
    useRoomStore.getState().setRoomInvite(null);
    joinRoomSpy.mockReset();
    navigateSpy.mockReset();
    vi.mocked(toast.error).mockReset();
  });
  afterEach(() => {
    useRoomStore.getState().setRoomInvite(null);
  });

  it("renders nothing without an invite", () => {
    renderModal();
    expect(screen.queryByTestId("room-invite-modal")).not.toBeInTheDocument();
  });

  it("names the inviter and the room", () => {
    useRoomStore.getState().setRoomInvite(baseInvite);
    renderModal();

    expect(screen.getByTestId("room-invite-modal")).toBeInTheDocument();
    expect(screen.getByTestId("room-invite-body")).toHaveTextContent("ana");
    expect(screen.getByTestId("room-invite-body")).toHaveTextContent("Skopje Ekipa");
    expect(screen.getByTestId("room-invite-buyin")).toBeInTheDocument();
  });

  it("hides the buy-in chip for a free table", () => {
    useRoomStore.getState().setRoomInvite({ ...baseInvite, coinBuyIn: 0 });
    renderModal();
    expect(screen.queryByTestId("room-invite-buyin")).not.toBeInTheDocument();
  });

  // AC3: a host invite joins with NO password — the server grant does the work.
  it("joins with no password on a host invite into a private room", async () => {
    const user = userEvent.setup();
    joinRoomSpy.mockResolvedValue({ id: 7 });
    useRoomStore.getState().setRoomInvite({ ...baseInvite, isPrivate: true, isHostInvite: true });
    renderModal();

    await user.click(screen.getByTestId("room-invite-accept"));

    await waitFor(() => expect(joinRoomSpy).toHaveBeenCalledWith(7, undefined));
    expect(screen.queryByTestId("password-prompt-dialog")).not.toBeInTheDocument();
    await waitFor(() => expect(navigateSpy).toHaveBeenCalledWith("/rooms/7"));
    expect(useRoomStore.getState().roomInvite).toBeNull();
  });

  // AC4: a non-host invite into a private room still has to clear the password.
  it("opens the password prompt on a non-host invite into a private room", async () => {
    const user = userEvent.setup();
    useRoomStore.getState().setRoomInvite({ ...baseInvite, isPrivate: true, isHostInvite: false });
    renderModal();

    await user.click(screen.getByTestId("room-invite-accept"));

    expect(await screen.findByTestId("password-prompt-dialog")).toBeInTheDocument();
    expect(joinRoomSpy).not.toHaveBeenCalled();

    joinRoomSpy.mockResolvedValue({ id: 7 });
    await user.type(screen.getByTestId("password-prompt-input"), "hunter2");
    await user.click(screen.getByTestId("password-prompt-submit"));

    await waitFor(() => expect(joinRoomSpy).toHaveBeenCalledWith(7, "hunter2"));
  });

  it("keeps the password prompt open on a wrong password", async () => {
    const user = userEvent.setup();
    useRoomStore.getState().setRoomInvite({ ...baseInvite, isPrivate: true, isHostInvite: false });
    renderModal();

    await user.click(screen.getByTestId("room-invite-accept"));
    joinRoomSpy.mockRejectedValue(
      new FetchError(409, "WRONG_ROOM_PASSWORD", "incorrect room password"),
    );
    await user.type(screen.getByTestId("password-prompt-input"), "wrong");
    await user.click(screen.getByTestId("password-prompt-submit"));

    expect(await screen.findByTestId("password-prompt-error")).toBeInTheDocument();
    expect(screen.getByTestId("password-prompt-dialog")).toBeInTheDocument();
    expect(useRoomStore.getState().roomInvite).not.toBeNull();
  });

  // AC5: no password step at all for a non-host invite into a public room.
  it("joins directly on a non-host invite into a public room", async () => {
    const user = userEvent.setup();
    joinRoomSpy.mockResolvedValue({ id: 7 });
    useRoomStore.getState().setRoomInvite(baseInvite);
    renderModal();

    await user.click(screen.getByTestId("room-invite-accept"));

    await waitFor(() => expect(joinRoomSpy).toHaveBeenCalledWith(7, undefined));
    expect(screen.queryByTestId("password-prompt-dialog")).not.toBeInTheDocument();
  });

  // AC6/AC7: the honor gate still applies under an invite, and the failure is
  // routed through the ONE shared join-failure mapping (D4).
  it.each([
    ["ROOM_FULL", "lobby.errors.roomFull"],
    ["NEW_PLAYER_NOT_ALLOWED", "room.errors.newPlayerNotAllowed"],
    ["ROOM_NOT_FOUND", "lobby.errors.roomNotFound"],
  ])(
    "surfaces %s through the shared join-failure mapping and stays in the lobby",
    async (code, key) => {
      const user = userEvent.setup();
      joinRoomSpy.mockRejectedValue(new FetchError(409, code, "join rejected"));
      useRoomStore.getState().setRoomInvite(baseInvite);
      renderModal();

      await user.click(screen.getByTestId("room-invite-accept"));

      const i18n = (await import("i18next")).default;
      await waitFor(() => expect(toast.error).toHaveBeenCalledWith(i18n.t(key)));
      // No dead end: the popup is gone and no navigation happened.
      expect(useRoomStore.getState().roomInvite).toBeNull();
      expect(navigateSpy).not.toHaveBeenCalled();
    },
  );

  // HONOR_TOO_LOW takes the SPECIFIC arm of the same mapping, not the generic
  // fallback: an invite payload always carries the room's `minHonor` (Story
  // 11.5 added the field for exactly this), so `joinFailure` has the floor in
  // hand and can name it. The generic arm is for callers with no room object,
  // which is why JoinByCodeTile asserts that one instead.
  it("names the room's honor floor when an invited join is rejected for honor", async () => {
    const user = userEvent.setup();
    joinRoomSpy.mockRejectedValue(new FetchError(409, "HONOR_TOO_LOW", "join rejected"));
    useRoomStore.getState().setRoomInvite(makeRoomInvite({ ...baseInvite, minHonor: 85 }));
    renderModal();

    await user.click(screen.getByTestId("room-invite-accept"));

    const i18n = (await import("i18next")).default;
    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        i18n.t("room.errors.honorTooLow", { minHonor: 85, honor: 80 }),
      ),
    );
    expect(useRoomStore.getState().roomInvite).toBeNull();
    expect(navigateSpy).not.toHaveBeenCalled();
  });

  it("declining clears the invite without joining", async () => {
    const user = userEvent.setup();
    useRoomStore.getState().setRoomInvite(baseInvite);
    renderModal();

    await user.click(screen.getByTestId("room-invite-decline"));

    expect(joinRoomSpy).not.toHaveBeenCalled();
    expect(useRoomStore.getState().roomInvite).toBeNull();
  });

  // AC2: the popup must not outlive the grant behind it.
  it("auto-dismisses an already-expired invite", async () => {
    useRoomStore.getState().setRoomInvite({
      ...baseInvite,
      expiresAt: new Date(Date.now() - 1000).toISOString(),
    });
    renderModal();

    await waitFor(() => expect(useRoomStore.getState().roomInvite).toBeNull());
  });
});
