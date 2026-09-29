import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { useAuthStore } from "@/shared/stores/authStore";
import { QueryWrapper } from "@/test-utils";

import { useLoginMutation } from "./useAuth";

const mockLogin = vi.fn();
vi.mock("@/shared/api/auth", () => ({
  login: (...args: unknown[]) => mockLogin(...args),
  register: vi.fn(),
  forgotPassword: vi.fn(),
  resetPassword: vi.fn(),
  ssoLogin: vi.fn(),
  ssoLink: vi.fn(),
}));

const envelope = {
  token: "tok",
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
  mockLogin.mockReset();
  useAuthStore.setState({ token: null, user: null, isLoading: false });
});

async function logIn() {
  const { result } = renderHook(() => useLoginMutation(), { wrapper: QueryWrapper });
  await act(async () => {
    await result.current.mutateAsync({ email: "kiro@example.com", password: "password123" });
  });
}

describe("useLoginMutation avatar", () => {
  it("copies the envelope's avatarUrl into the auth store", async () => {
    const url = "https://assets.test/avatars/k/128.webp";
    mockLogin.mockResolvedValue({ ...envelope, avatarUrl: url });

    await logIn();

    expect(useAuthStore.getState().user?.avatarUrl).toBe(url);
  });

  it("stores null when the envelope has no avatarUrl key", async () => {
    mockLogin.mockResolvedValue(envelope);

    await logIn();

    expect(useAuthStore.getState().user).not.toBeNull();
    expect(useAuthStore.getState().user?.avatarUrl).toBeNull();
  });
});
