import "@/shared/i18n/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
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

const audioEngine = vi.hoisted(() => ({
  playAlertSfx: vi.fn((_name: string, _options: { dedupeKey: string; signal?: AbortSignal }) =>
    Promise.resolve(),
  ),
}));
vi.mock("@/shared/audio/audioEngine", () => audioEngine);

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

describe("RoomInviteModal sound", () => {
  beforeEach(() => {
    useRoomStore.getState().setRoomInvite(null);
    useRoomStore.getState().setRoomEjection(null);
    audioEngine.playAlertSfx.mockClear();
    joinRoomSpy.mockReset();
  });
  afterEach(() => {
    cleanup();
    useRoomStore.getState().setRoomInvite(null);
    useRoomStore.getState().setRoomEjection(null);
  });

  const baseKey = `1:${baseInvite.expiresAt}`;
  const laterExpiry = new Date(Date.now() + 120_000).toISOString();

  function whistles() {
    return audioEngine.playAlertSfx.mock.calls.map(([name, options]) => ({
      name,
      dedupeKey: options.dedupeKey,
      aborted: options.signal?.aborted,
    }));
  }

  function showEjectionNotice() {
    useRoomStore.getState().setRoomEjection({
      roomId: 9,
      buyIn: 500,
      balance: 100,
      reason: "insolvent",
    });
  }

  it("stays silent without an invite", () => {
    renderModal();
    expect(audioEngine.playAlertSfx).not.toHaveBeenCalled();
  });

  it("whistles once as the popup appears, keyed to the invite's id and expiry", () => {
    useRoomStore.getState().setRoomInvite(baseInvite);
    renderModal();

    expect(screen.getByTestId("room-invite-modal")).toBeInTheDocument();
    expect(whistles()).toEqual([{ name: "invite", dedupeKey: baseKey, aborted: false }]);
  });

  it("does not whistle again when the same invite re-renders", () => {
    useRoomStore.getState().setRoomInvite(baseInvite);
    renderModal();

    act(() => {
      useRoomStore.getState().setRoomInvite({ ...baseInvite });
    });

    expect(whistles()).toEqual([{ name: "invite", dedupeKey: baseKey, aborted: false }]);
  });

  it("whistles again for a new invite, aborting the replaced one", () => {
    useRoomStore.getState().setRoomInvite(baseInvite);
    renderModal();

    act(() => {
      useRoomStore.getState().setRoomInvite({ ...baseInvite, inviteId: 2 });
    });

    expect(whistles()).toEqual([
      { name: "invite", dedupeKey: baseKey, aborted: true },
      { name: "invite", dedupeKey: `2:${baseInvite.expiresAt}`, aborted: false },
    ]);
  });

  it("whistles again for the same inviteId with a new expiry", () => {
    useRoomStore.getState().setRoomInvite(baseInvite);
    renderModal();

    act(() => {
      useRoomStore.getState().setRoomInvite({ ...baseInvite, expiresAt: laterExpiry });
    });

    expect(whistles().map((w) => w.dedupeKey)).toEqual([baseKey, `1:${laterExpiry}`]);
  });

  it("stays silent behind the ejection notice, then whistles once the popup shows", () => {
    showEjectionNotice();
    useRoomStore.getState().setRoomInvite(baseInvite);
    renderModal();

    expect(screen.queryByTestId("room-invite-modal")).not.toBeInTheDocument();
    expect(audioEngine.playAlertSfx).not.toHaveBeenCalled();

    act(() => {
      useRoomStore.getState().setRoomEjection(null);
    });

    expect(screen.getByTestId("room-invite-modal")).toBeInTheDocument();
    expect(whistles()).toEqual([{ name: "invite", dedupeKey: baseKey, aborted: false }]);
  });

  it("reuses the same key when the same invite is hidden and shown again", () => {
    useRoomStore.getState().setRoomInvite(baseInvite);
    renderModal();

    act(() => {
      showEjectionNotice();
    });
    act(() => {
      useRoomStore.getState().setRoomEjection(null);
    });

    expect(whistles()).toEqual([
      { name: "invite", dedupeKey: baseKey, aborted: true },
      { name: "invite", dedupeKey: baseKey, aborted: false },
    ]);
  });

  it("aborts the whistle when the popup unmounts", () => {
    useRoomStore.getState().setRoomInvite(baseInvite);
    const { unmount } = renderModal();

    unmount();

    expect(whistles()).toEqual([{ name: "invite", dedupeKey: baseKey, aborted: true }]);
  });

  it("aborts the whistle when the invite is declined", async () => {
    const user = userEvent.setup();
    useRoomStore.getState().setRoomInvite(baseInvite);
    renderModal();

    await user.click(screen.getByTestId("room-invite-decline"));

    expect(whistles()).toEqual([{ name: "invite", dedupeKey: baseKey, aborted: true }]);
  });

  it.each(["room-invite-accept", "room-invite-decline"])(
    "aborts the whistle on a press of %s and never replays it for that invite",
    (testId) => {
      useRoomStore.getState().setRoomInvite(baseInvite);
      renderModal();

      fireEvent.pointerDown(screen.getByTestId(testId));
      act(() => {
        showEjectionNotice();
      });
      act(() => {
        useRoomStore.getState().setRoomEjection(null);
      });

      expect(whistles()).toEqual([{ name: "invite", dedupeKey: baseKey, aborted: true }]);
    },
  );

  it("aborts the whistle once a join into a public room is in flight", async () => {
    joinRoomSpy.mockReturnValue(new Promise(() => {}));
    useRoomStore.getState().setRoomInvite(baseInvite);
    renderModal();

    fireEvent.click(screen.getByTestId("room-invite-accept"));

    await waitFor(() =>
      expect(whistles()).toEqual([{ name: "invite", dedupeKey: baseKey, aborted: true }]),
    );
    expect(joinRoomSpy).toHaveBeenCalledWith(7, undefined);
  });

  it("aborts the whistle when Accept opens the password prompt", async () => {
    useRoomStore.getState().setRoomInvite({ ...baseInvite, isPrivate: true, isHostInvite: false });
    renderModal();

    fireEvent.click(screen.getByTestId("room-invite-accept"));

    expect(await screen.findByTestId("password-prompt-dialog")).toBeInTheDocument();
    expect(whistles()).toEqual([{ name: "invite", dedupeKey: baseKey, aborted: true }]);
  });

  it("aborts the whistle for an invite already expired on arrival", async () => {
    const past = new Date(Date.now() - 1000).toISOString();
    useRoomStore.getState().setRoomInvite({ ...baseInvite, expiresAt: past });
    renderModal();

    await waitFor(() => expect(useRoomStore.getState().roomInvite).toBeNull());
    expect(whistles()).toEqual([{ name: "invite", dedupeKey: `1:${past}`, aborted: true }]);
  });

  it("aborts the whistle when the invite expires on its timer", async () => {
    const soon = new Date(Date.now() + 150).toISOString();
    useRoomStore.getState().setRoomInvite({ ...baseInvite, expiresAt: soon });
    renderModal();
    expect(whistles()).toEqual([{ name: "invite", dedupeKey: `1:${soon}`, aborted: false }]);

    await waitFor(() => expect(useRoomStore.getState().roomInvite).toBeNull());
    expect(whistles()).toEqual([{ name: "invite", dedupeKey: `1:${soon}`, aborted: true }]);
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
