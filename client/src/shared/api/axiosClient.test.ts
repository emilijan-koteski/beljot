import { afterEach, describe, expect, it, vi } from "vitest";

import { useAuthStore } from "@/shared/stores/authStore";

import { axiosClient, axiosPublic, FetchError, refreshAccessToken } from "./axiosClient";

describe("axiosClient request deadline", () => {
  // A missing timeout means a hung connection never settles the promise, which
  // strands every pending-guarded confirm dialog (remove-friend,
  // unlink-account, remove-bot) with all dismissal paths disabled. Both
  // instances must carry one — axiosPublic handles login/refresh/logout, which
  // are exactly the calls a user cannot navigate away from.
  it.each([
    ["axiosClient", axiosClient],
    ["axiosPublic", axiosPublic],
  ])("%s sends a finite timeout with every request", (_name, instance) => {
    expect(instance.defaults.timeout).toBeGreaterThan(0);
    expect(Number.isFinite(instance.defaults.timeout)).toBe(true);
  });

  it("reports a timed-out request as TIMEOUT, distinct from a network failure", async () => {
    // Axios surfaces an exceeded `timeout` as ECONNABORTED with no response.
    // The interceptor must map that to its own code so callers can say "took
    // too long" rather than "you are offline".
    const timeoutError = Object.assign(new Error("timeout of 15000ms exceeded"), {
      isAxiosError: true,
      code: "ECONNABORTED",
      config: {},
      response: undefined,
    });

    const handler = axiosClient.interceptors.response as unknown as {
      handlers: { rejected: (e: unknown) => Promise<unknown> }[];
    };
    const rejected = handler.handlers[0]!.rejected;

    await expect(rejected(timeoutError)).rejects.toMatchObject({
      name: "FetchError",
      status: 0,
      code: "TIMEOUT",
    });
  });

  it("still reports a genuine network failure as NETWORK_ERROR", async () => {
    const networkError = Object.assign(new Error("Network Error"), {
      isAxiosError: true,
      code: "ERR_NETWORK",
      config: {},
      response: undefined,
    });

    const handler = axiosClient.interceptors.response as unknown as {
      handlers: { rejected: (e: unknown) => Promise<unknown> }[];
    };
    const rejected = handler.handlers[0]!.rejected;

    await expect(rejected(networkError)).rejects.toBeInstanceOf(FetchError);
    await expect(rejected(networkError)).rejects.toMatchObject({
      status: 0,
      code: "NETWORK_ERROR",
    });
  });
});

describe("refreshAccessToken re-hydrates the avatar", () => {
  const envelope = {
    token: "fresh",
    id: 1,
    username: "kiro",
    email: "kiro@example.com",
    languagePreference: "en",
    cardDeckPreference: "french",
    soundEnabled: true,
    musicEnabled: true,
    soundVolume: 70,
    musicVolume: 70,
    walletBalance: 5000,
    loginStreakDays: 0,
    totalXp: 0,
    level: 0,
    honorScore: 80,
    honorTier: "fair",
    isNewPlayer: false,
    createdAt: "2026-01-01T00:00:00Z",
  };

  afterEach(() => {
    vi.restoreAllMocks();
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  it("copies the envelope's avatarUrl into the auth store", async () => {
    const url = "https://assets.test/avatars/k/128.webp";
    vi.spyOn(axiosPublic, "post").mockResolvedValue({
      data: { data: { ...envelope, avatarUrl: url } },
    });

    await refreshAccessToken();

    expect(useAuthStore.getState().user?.avatarUrl).toBe(url);
  });

  it("stores null when the envelope has no avatarUrl key", async () => {
    vi.spyOn(axiosPublic, "post").mockResolvedValue({ data: { data: envelope } });

    await refreshAccessToken();

    expect(useAuthStore.getState().user).not.toBeNull();
    expect(useAuthStore.getState().user?.avatarUrl).toBeNull();
  });
});
