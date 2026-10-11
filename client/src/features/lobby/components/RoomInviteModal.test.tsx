import "@/shared/i18n/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
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
import { expectActionBeforeStatusQuo, makeRoomInvite } from "@/test-utils";

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
    cleanup();
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

  it("joins directly on a non-host invite into a public room", async () => {
    const user = userEvent.setup();
    joinRoomSpy.mockResolvedValue({ id: 7 });
    useRoomStore.getState().setRoomInvite(baseInvite);
    renderModal();

    await user.click(screen.getByTestId("room-invite-accept"));

    await waitFor(() => expect(joinRoomSpy).toHaveBeenCalledWith(7, undefined));
    expect(screen.queryByTestId("password-prompt-dialog")).not.toBeInTheDocument();
  });

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
      expect(useRoomStore.getState().roomInvite).toBeNull();
      expect(navigateSpy).not.toHaveBeenCalled();
    },
  );

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

  it("auto-dismisses an already-expired invite", async () => {
    useRoomStore.getState().setRoomInvite({
      ...baseInvite,
      expiresAt: new Date(Date.now() - 1000).toISOString(),
    });
    renderModal();

    await waitFor(() => expect(useRoomStore.getState().roomInvite).toBeNull());
  });
});

describe("RoomInviteModal footer order", () => {
  beforeEach(() => {
    useRoomStore.getState().setRoomInvite(null);
  });
  afterEach(() => {
    cleanup();
    useRoomStore.getState().setRoomInvite(null);
  });

  it("places accept before decline so the action sits left of the status quo", () => {
    useRoomStore.getState().setRoomInvite(baseInvite);
    renderModal();
    expectActionBeforeStatusQuo("room-invite-accept", "room-invite-decline");
  });
});
