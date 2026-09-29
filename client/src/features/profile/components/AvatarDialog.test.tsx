import "@/shared/i18n/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { FetchError } from "@/shared/api/axiosClient";
import type { ProfileResponse } from "@/shared/api/profile";
import { queryKeys } from "@/shared/api/queryKeys";
import { i18n } from "@/shared/i18n/i18n";
import { useAuthStore } from "@/shared/stores/authStore";
import { makeUser } from "@/test-utils";

import { AvatarDialog } from "./AvatarDialog";

const mockUploadAvatar = vi.fn();
const mockRemoveAvatar = vi.fn();
vi.mock("@/shared/api/profile", () => ({
  updatePreferences: vi.fn(),
  updateUsername: vi.fn(),
  uploadAvatar: (...args: unknown[]) => mockUploadAvatar(...args),
  removeAvatar: (...args: unknown[]) => mockRemoveAvatar(...args),
}));

const mockToast = { success: vi.fn(), error: vi.fn() };
vi.mock("sonner", () => ({
  toast: {
    success: (...a: unknown[]) => mockToast.success(...a),
    error: (...a: unknown[]) => mockToast.error(...a),
  },
}));

const SMALL = "https://assets.test/avatars/new/128.webp";
const LARGE = "https://assets.test/avatars/new/256.webp";
const CURRENT = "https://assets.test/avatars/old/256.webp";

function jpeg(name = "me.jpg", bytes = 1024, type = "image/jpeg"): File {
  return new File([new Uint8Array(bytes)], name, { type });
}

type Props = Parameters<typeof AvatarDialog>[0];

function renderDialog(props: Partial<Props> = {}) {
  // Not createTestQueryClient: its gcTime of 0 would drop the seeded, unobserved
  // profile entry before the mutation could patch it.
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  client.setQueryData<Partial<ProfileResponse>>(queryKeys.profile.detail(1), {
    id: 1,
    username: "kiro",
    avatarUrl: null,
    avatarLargeUrl: null,
  });
  const onClose = vi.fn();
  render(
    <QueryClientProvider client={client}>
      <AvatarDialog open onClose={onClose} userId={1} username="kiro" avatarUrl={null} {...props} />
    </QueryClientProvider>,
  );
  return { client, onClose };
}

function input(): HTMLInputElement {
  return screen.getByTestId("avatar-dialog-input") as HTMLInputElement;
}

// jsdom implements neither. Installed for the whole file and never removed:
// the dialog's unmount cleanup revokes after each test's own hooks have run.
const createObjectURL = vi.fn(() => "blob:preview");
const revokeObjectURL = vi.fn();
URL.createObjectURL = createObjectURL;
URL.revokeObjectURL = revokeObjectURL;

beforeEach(() => {
  mockUploadAvatar.mockReset();
  mockRemoveAvatar.mockReset();
  mockToast.success.mockReset();
  mockToast.error.mockReset();
  createObjectURL.mockClear();
  revokeObjectURL.mockClear();
  useAuthStore.setState({
    token: "t",
    user: makeUser({ id: 1, username: "kiro" }),
    isLoading: false,
  });
});

describe("AvatarDialog", () => {
  it("offers only a file picker restricted to JPEG, PNG and WebP", () => {
    renderDialog();
    expect(input()).toHaveAttribute("type", "file");
    expect(input()).toHaveAttribute("accept", "image/jpeg,image/png,image/webp");
    expect(screen.getByTestId("avatar-dialog-upload")).toBeDisabled();
    // No avatar yet, so there is nothing to remove.
    expect(screen.queryByTestId("avatar-dialog-remove")).not.toBeInTheDocument();
  });

  it("rejects a file over 2 MiB inline without sending anything", async () => {
    const user = userEvent.setup();
    renderDialog();

    // The polite live region is mounted, empty, before there is anything to say.
    expect(screen.getByTestId("avatar-dialog-live")).toBeEmptyDOMElement();

    await user.upload(input(), jpeg("huge.jpg", 2_097_153));

    expect(screen.getByTestId("avatar-dialog-live")).toContainElement(
      screen.getByTestId("avatar-dialog-error"),
    );
    expect(screen.getByTestId("avatar-dialog-error")).toHaveTextContent(
      i18n.t("profile.avatar.errors.tooLarge"),
    );
    expect(screen.getByTestId("avatar-dialog-upload")).toBeDisabled();
    expect(createObjectURL).not.toHaveBeenCalled();
    expect(mockUploadAvatar).not.toHaveBeenCalled();
  });

  it("accepts a file of exactly 2 MiB", async () => {
    const user = userEvent.setup();
    renderDialog();

    await user.upload(input(), jpeg("edge.jpg", 2_097_152));

    expect(screen.queryByTestId("avatar-dialog-error")).not.toBeInTheDocument();
    expect(screen.getByTestId("avatar-dialog-upload")).toBeEnabled();
  });

  it("rejects a known-wrong type inline", async () => {
    // applyAccept off: the picker filter is not the only line of defence.
    const user = userEvent.setup({ applyAccept: false });
    renderDialog();

    await user.upload(input(), jpeg("anim.gif", 1024, "image/gif"));

    expect(screen.getByTestId("avatar-dialog-error")).toHaveTextContent(
      i18n.t("profile.avatar.errors.unsupported"),
    );
    expect(screen.getByTestId("avatar-dialog-upload")).toBeDisabled();
  });

  it("previews, uploads with progress, and adopts both URLs on success", async () => {
    let resolve!: (v: { avatarUrl: string; avatarLargeUrl: string }) => void;
    let reportProgress!: (fraction: number) => void;
    mockUploadAvatar.mockImplementation(
      (_id: number, _file: File, onProgress: (f: number) => void) => {
        reportProgress = onProgress;
        return new Promise((r) => {
          resolve = r;
        });
      },
    );
    const user = userEvent.setup();
    const { client, onClose } = renderDialog();
    const file = jpeg();

    await user.upload(input(), file);
    expect(screen.getByTestId("avatar-dialog-preview")).toHaveAttribute("src", "blob:preview");

    await user.click(screen.getByTestId("avatar-dialog-upload"));
    expect(mockUploadAvatar).toHaveBeenCalledWith(1, file, expect.any(Function));

    // In flight: progress shows, nothing can be pressed.
    reportProgress(0.5);
    await waitFor(() =>
      expect(screen.getByTestId("avatar-dialog-progress")).toHaveAttribute("aria-valuenow", "50"),
    );
    expect(screen.getByTestId("avatar-dialog-cancel")).toBeDisabled();
    expect(screen.getByTestId("avatar-dialog-upload")).toBeDisabled();

    resolve({ avatarUrl: SMALL, avatarLargeUrl: LARGE });

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(mockToast.success).toHaveBeenCalledWith(i18n.t("profile.avatar.success"));
    expect(useAuthStore.getState().user?.avatarUrl).toBe(SMALL);
    const cached = client.getQueryData<ProfileResponse>(queryKeys.profile.detail(1));
    expect(cached?.avatarUrl).toBe(SMALL);
    expect(cached?.avatarLargeUrl).toBe(LARGE);
    // The preview's object URL is released once the dialog lets go of it.
    await waitFor(() => expect(revokeObjectURL).toHaveBeenCalledWith("blob:preview"));
  });

  it("cannot be dismissed with Escape while an upload is in flight", async () => {
    mockUploadAvatar.mockReturnValue(new Promise(() => {}));
    const user = userEvent.setup();
    const { onClose } = renderDialog();

    await user.upload(input(), jpeg());
    await user.click(screen.getByTestId("avatar-dialog-upload"));
    await user.keyboard("{Escape}");

    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByTestId("avatar-dialog")).toBeInTheDocument();
  });

  it.each([
    [new FetchError(400, "AVATAR_TOO_SMALL", ""), "profile.avatar.errors.tooSmall"],
    [new FetchError(413, "AVATAR_TOO_LARGE", ""), "profile.avatar.errors.tooLarge"],
    [new FetchError(413, "AVATAR_DIMENSIONS_TOO_LARGE", ""), "profile.avatar.errors.tooLarge"],
    [new FetchError(415, "AVATAR_UNSUPPORTED_TYPE", ""), "profile.avatar.errors.unsupported"],
    [new FetchError(429, "AVATAR_UPLOAD_RATE_LIMITED", ""), "profile.avatar.errors.rateLimited"],
    [new FetchError(503, "AVATAR_BUSY", ""), "profile.avatar.errors.busy"],
    [new FetchError(503, "AVATAR_STORAGE_UNAVAILABLE", ""), "profile.avatar.errors.unavailable"],
    [new FetchError(500, "INTERNAL_ERROR", ""), "profile.avatar.errors.generic"],
  ])("toasts a failed upload (%s) with its message and stays open", async (err, key) => {
    mockUploadAvatar.mockRejectedValue(err);
    const user = userEvent.setup();
    const { onClose } = renderDialog();

    await user.upload(input(), jpeg());
    await user.click(screen.getByTestId("avatar-dialog-upload"));

    await waitFor(() => expect(mockToast.error).toHaveBeenCalledWith(i18n.t(key)));
    expect(onClose).not.toHaveBeenCalled();
    expect(useAuthStore.getState().user?.avatarUrl).toBeNull();
    // The picked file survives, so a retry is one click.
    expect(screen.getByTestId("avatar-dialog-upload")).toBeEnabled();
  });

  it("toasts a failed removal and keeps the avatar", async () => {
    mockRemoveAvatar.mockRejectedValue(new FetchError(503, "AVATAR_STORAGE_UNAVAILABLE", ""));
    useAuthStore.setState({ user: makeUser({ id: 1, avatarUrl: SMALL }) });
    const user = userEvent.setup();
    const { onClose } = renderDialog({ avatarUrl: CURRENT });

    await user.click(screen.getByTestId("avatar-dialog-remove"));

    await waitFor(() =>
      expect(mockToast.error).toHaveBeenCalledWith(i18n.t("profile.avatar.errors.unavailable")),
    );
    expect(onClose).not.toHaveBeenCalled();
    expect(useAuthStore.getState().user?.avatarUrl).toBe(SMALL);
  });

  it("removes an existing avatar and clears it everywhere", async () => {
    mockRemoveAvatar.mockResolvedValue(undefined);
    useAuthStore.setState({ user: makeUser({ id: 1, avatarUrl: SMALL }) });
    const user = userEvent.setup();
    const { client, onClose } = renderDialog({ avatarUrl: CURRENT });

    await user.click(screen.getByTestId("avatar-dialog-remove"));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(mockRemoveAvatar).toHaveBeenCalledWith(1);
    expect(mockToast.success).toHaveBeenCalledWith(i18n.t("profile.avatar.removed"));
    expect(useAuthStore.getState().user?.avatarUrl).toBeNull();
    const cached = client.getQueryData<ProfileResponse>(queryKeys.profile.detail(1));
    expect(cached?.avatarUrl).toBeNull();
    expect(cached?.avatarLargeUrl).toBeNull();
  });

  it("shows the current avatar until a new file is picked", () => {
    renderDialog({ avatarUrl: CURRENT });
    const img = screen.getByTestId("avatar-dialog").querySelector("img");
    expect(img).toHaveAttribute("src", CURRENT);
    expect(img).toHaveAttribute("loading", "eager");
  });
});
