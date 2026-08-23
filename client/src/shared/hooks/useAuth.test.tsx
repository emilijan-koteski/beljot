import { render, waitFor } from "@testing-library/react";
import { StrictMode } from "react";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useAuthStore } from "@/shared/stores/authStore";

const refreshAccessToken = vi.fn();
const changeLanguage = vi.fn(() => Promise.resolve(undefined));

// Only the refresh entry point is stubbed; setAuthRedirect and the axios
// instance stay real so the hook is exercised as it ships.
vi.mock("@/shared/api/axiosClient", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/shared/api/axiosClient")>()),
  refreshAccessToken: () => refreshAccessToken(),
}));

vi.mock("react-i18next", async (importOriginal) => ({
  ...(await importOriginal<typeof import("react-i18next")>()),
  useTranslation: () => ({ t: (k: string) => k, i18n: { changeLanguage } }),
}));

const { useAuthInit } = await import("@/shared/hooks/useAuth");

function Probe() {
  useAuthInit();
  return null;
}

function renderAt(path: string, { strict = false } = {}) {
  const tree = (
    <MemoryRouter initialEntries={[path]}>
      <Probe />
    </MemoryRouter>
  );
  return render(strict ? <StrictMode>{tree}</StrictMode> : tree);
}

const USER = {
  id: 1,
  username: "alice",
  email: "a@b.c",
  languagePreference: "mk",
  cardDeckPreference: "french" as const,
  walletBalance: 0,
  loginStreakDays: 0,
  totalXp: 0,
  level: 1,
  honorScore: 0,
  honorTier: "new",
  isNewPlayer: true,
  createdAt: "2026-01-01T00:00:00Z",
};

const restoreSucceeds = () =>
  refreshAccessToken.mockImplementation(() => {
    useAuthStore.getState().setToken("fresh-token");
    useAuthStore.getState().setUser(USER);
    return Promise.resolve("fresh-token");
  });

describe("useAuthInit", () => {
  beforeEach(() => {
    refreshAccessToken.mockReset();
    changeLanguage.mockReset();
    changeLanguage.mockResolvedValue(undefined);
    useAuthStore.setState({ token: null, user: null, isLoading: true });
  });

  afterEach(() => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  it("restores the session through the coordinated refresh singleton", async () => {
    restoreSucceeds();

    renderAt("/lobby");

    await waitFor(() => expect(useAuthStore.getState().isLoading).toBe(false));
    expect(refreshAccessToken).toHaveBeenCalledTimes(1);
    expect(useAuthStore.getState().token).toBe("fresh-token");
  });

  it("applies the saved locale before it stops loading", async () => {
    restoreSucceeds();

    renderAt("/lobby");

    await waitFor(() => expect(useAuthStore.getState().isLoading).toBe(false));
    expect(changeLanguage).toHaveBeenCalledWith("mk");
  });

  // The regression this file exists for. Restoring via a second, uncoordinated
  // caller rotated the refresh-token family twice; the browser then kept the
  // superseded cookie and the session died at the NEXT refresh, minutes later
  // and far from the cause. A remount must never trigger a second rotation.
  it("does not refresh twice when the effect is mounted twice (StrictMode)", async () => {
    restoreSucceeds();

    renderAt("/lobby", { strict: true });

    await waitFor(() => expect(useAuthStore.getState().isLoading).toBe(false));
    expect(refreshAccessToken).toHaveBeenCalledTimes(1);
  });

  // A locale switch is post-restore cosmetics. It once shared a `.catch` with
  // the restore itself, so an i18n failure cleared a perfectly valid session.
  it("keeps the session when only the language switch fails", async () => {
    restoreSucceeds();
    changeLanguage.mockRejectedValue(new Error("locale load failed"));

    renderAt("/lobby");

    await waitFor(() => expect(useAuthStore.getState().isLoading).toBe(false));
    expect(useAuthStore.getState().token).toBe("fresh-token");
  });

  it("clears the token and stops loading when there is no session to restore", async () => {
    refreshAccessToken.mockRejectedValue(new Error("no session"));

    renderAt("/lobby");

    await waitFor(() => expect(useAuthStore.getState().isLoading).toBe(false));
    expect(useAuthStore.getState().token).toBeNull();
  });

  it("skips the restore entirely on guest pages", async () => {
    renderAt("/login");

    await waitFor(() => expect(useAuthStore.getState().isLoading).toBe(false));
    expect(refreshAccessToken).not.toHaveBeenCalled();
  });

  it("skips the restore when a token is already in memory", async () => {
    useAuthStore.setState({ token: "already-here", isLoading: true });

    renderAt("/lobby");

    await waitFor(() => expect(useAuthStore.getState().isLoading).toBe(false));
    expect(refreshAccessToken).not.toHaveBeenCalled();
  });
});
