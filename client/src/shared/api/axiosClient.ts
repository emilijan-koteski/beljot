import type { AxiosError, InternalAxiosRequestConfig } from "axios";
import axios from "axios";

import { useAuthStore } from "@/shared/stores/authStore";
import type { CardDeck } from "@/shared/types/matchTypes";

const BASE_URL = "/api/v1";

const REQUEST_TIMEOUT_MS = 15_000;

interface ApiError {
  code: string;
  message: string;
}

interface ApiErrorResponse {
  error: ApiError;
}

export class FetchError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "FetchError";
    this.status = status;
    this.code = code;
  }
}

let authRedirect: (() => void) | null = null;
let hasRedirected = false;

export function setAuthRedirect(fn: () => void): void {
  authRedirect = fn;
  hasRedirected = false;
}

function redirectToLogin(): void {
  if (hasRedirected) return;
  hasRedirected = true;
  if (authRedirect) {
    authRedirect();
  } else {
    window.location.href = "/";
  }
}

const authChannel: BroadcastChannel | null =
  typeof BroadcastChannel !== "undefined" ? new BroadcastChannel("beljot-auth") : null;

interface TokenBroadcast {
  type: "token";
  token: string;
}

/** Shares a refreshed token so sibling tabs adopt it instead of each rotating the refresh token. */
function broadcastToken(token: string): void {
  authChannel?.postMessage({ type: "token", token } satisfies TokenBroadcast);
}

/** Invokes onToken when a sibling tab refreshes and returns an unsubscribe function. */
export function subscribeToTokenBroadcast(onToken: (token: string) => void): () => void {
  if (!authChannel) return () => {};
  const handler = (e: MessageEvent<TokenBroadcast>) => {
    if (e.data?.type === "token" && typeof e.data.token === "string") {
      onToken(e.data.token);
    }
  };
  authChannel.addEventListener("message", handler);
  return () => authChannel.removeEventListener("message", handler);
}

let refreshPromise: Promise<string> | null = null;

/**
 * Obtains a fresh access token, deduping concurrent callers and broadcasting the result cross-tab.
 * Use this instead of calling /auth/refresh directly.
 */
export function refreshAccessToken(): Promise<string> {
  return doRefresh();
}

async function doRefresh(): Promise<string> {
  if (refreshPromise) {
    return refreshPromise;
  }

  refreshPromise = axiosPublic
    .post<{
      data: {
        token: string;
        id: number;
        username: string;
        email: string;
        languagePreference: string;
        cardDeckPreference: CardDeck;
        soundEnabled: boolean;
        musicEnabled: boolean;
        soundVolume: number;
        musicVolume: number;
        allowNonFriendInvites: boolean;
        walletBalance: number;
        loginStreakDays: number;
        totalXp: number;
        level: number;
        honorScore: number;
        honorTier: string;
        isNewPlayer: boolean;
        avatarUrl: string | null;
        createdAt: string;
      };
    }>("/auth/refresh")
    .then((res) => {
      const r = res.data.data;
      useAuthStore.getState().setToken(r.token);
      useAuthStore.getState().setUser({
        id: r.id,
        username: r.username,
        email: r.email,
        languagePreference: r.languagePreference,
        cardDeckPreference: r.cardDeckPreference,
        soundEnabled: r.soundEnabled,
        musicEnabled: r.musicEnabled,
        soundVolume: r.soundVolume,
        musicVolume: r.musicVolume,
        allowNonFriendInvites: r.allowNonFriendInvites,
        walletBalance: r.walletBalance,
        loginStreakDays: r.loginStreakDays,
        totalXp: r.totalXp,
        level: r.level,
        honorScore: r.honorScore,
        honorTier: r.honorTier,
        isNewPlayer: r.isNewPlayer,
        avatarUrl: r.avatarUrl ?? null,
        createdAt: r.createdAt,
      });
      broadcastToken(r.token);
      return r.token;
    })
    .finally(() => {
      refreshPromise = null;
    });

  return refreshPromise;
}

export const axiosPublic = axios.create({
  baseURL: BASE_URL,
  withCredentials: true,
  timeout: REQUEST_TIMEOUT_MS,
  headers: { "Content-Type": "application/json" },
});

/** Request config carrying the one-shot 401 retry flag. */
interface RetryConfig extends InternalAxiosRequestConfig {
  _isRetry?: boolean;
}

/** Axios reports an exceeded timeout or an abort as ECONNABORTED, and some platforms as ETIMEDOUT. */
function isTimeout(error: AxiosError): boolean {
  return error.code === "ECONNABORTED" || error.code === "ETIMEDOUT";
}

export const axiosClient = axios.create({
  baseURL: BASE_URL,
  withCredentials: true,
  timeout: REQUEST_TIMEOUT_MS,
  headers: { "Content-Type": "application/json" },
});

axiosClient.interceptors.request.use((config) => {
  const token = useAuthStore.getState().token;
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

axiosClient.interceptors.response.use(
  (response) => {
    if (response.status === 204 || response.headers["content-length"] === "0") {
      return undefined as unknown as typeof response;
    }
    return response.data.data;
  },
  async (error: AxiosError<ApiErrorResponse>) => {
    const config = error.config as RetryConfig | undefined;

    if (error.response?.status === 401 && config) {
      if (config._isRetry) {
        useAuthStore.getState().logout();
        redirectToLogin();
        throw new FetchError(401, "UNAUTHORIZED", "Session expired");
      }

      try {
        const newToken = await doRefresh();
        config._isRetry = true;
        config.headers.Authorization = `Bearer ${newToken}`;
        const retryResponse = await axiosClient.request(config);
        return retryResponse;
      } catch {
        useAuthStore.getState().logout();
        redirectToLogin();
        throw new FetchError(401, "UNAUTHORIZED", "Session expired");
      }
    }

    if (error.response) {
      const body = error.response.data;
      if (body?.error) {
        throw new FetchError(error.response.status, body.error.code, body.error.message);
      }
      throw new FetchError(
        error.response.status,
        "UNKNOWN_ERROR",
        error.response.statusText || "Request failed",
      );
    }

    if (isTimeout(error)) {
      throw new FetchError(0, "TIMEOUT", error.message || "Request timed out");
    }

    throw new FetchError(0, "NETWORK_ERROR", error.message || "Network error");
  },
);
