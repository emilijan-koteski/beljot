import type { AxiosError, InternalAxiosRequestConfig } from "axios";
import axios from "axios";

import { useAuthStore } from "@/shared/stores/authStore";

const BASE_URL = "/api/v1";

// ---------------------------------------------------------------------------
// FetchError — kept for backward compatibility with all component error handling
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Auth redirect — same mechanism as the old fetchClient
// ---------------------------------------------------------------------------

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
    // Fallback when no SPA redirect is registered — the public landing is the
    // logged-out home (see useAuthInit's setAuthRedirect).
    window.location.href = "/";
  }
}

// ---------------------------------------------------------------------------
// Singleton refresh — deduplicates concurrent 401 refresh attempts
// ---------------------------------------------------------------------------

let refreshPromise: Promise<string> | null = null;

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
        walletBalance: number;
        loginStreakDays: number;
        createdAt: string;
      };
    }>("/auth/refresh")
    .then((res) => {
      const r = res.data.data;
      useAuthStore.getState().setToken(r.token);
      // Re-hydrate the full user incl. wallet fields — a 401-retry refresh that
      // dropped these would blank the header coin pill mid-session.
      useAuthStore.getState().setUser({
        id: r.id,
        username: r.username,
        email: r.email,
        languagePreference: r.languagePreference,
        walletBalance: r.walletBalance,
        loginStreakDays: r.loginStreakDays,
        createdAt: r.createdAt,
      });
      return r.token;
    })
    .finally(() => {
      refreshPromise = null;
    });

  return refreshPromise;
}

// ---------------------------------------------------------------------------
// axiosPublic — bare instance for login / refresh / logout (no interceptors)
// ---------------------------------------------------------------------------

export const axiosPublic = axios.create({
  baseURL: BASE_URL,
  withCredentials: true,
  headers: { "Content-Type": "application/json" },
});

// ---------------------------------------------------------------------------
// axiosClient — intercepted instance for all authenticated API calls
// ---------------------------------------------------------------------------

// Extend config type to carry the retry flag
interface RetryConfig extends InternalAxiosRequestConfig {
  _isRetry?: boolean;
}

export const axiosClient = axios.create({
  baseURL: BASE_URL,
  withCredentials: true,
  headers: { "Content-Type": "application/json" },
});

// Request interceptor: attach JWT Bearer token
axiosClient.interceptors.request.use((config) => {
  const token = useAuthStore.getState().token;
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Response interceptor: unwrap envelope, handle 401, throw FetchError
axiosClient.interceptors.response.use(
  // Success path — unwrap { data: T } envelope
  (response) => {
    if (response.status === 204 || response.headers["content-length"] === "0") {
      return undefined as unknown as typeof response;
    }
    return response.data.data;
  },

  // Error path — handle 401 refresh/retry and throw FetchError
  async (error: AxiosError<ApiErrorResponse>) => {
    const config = error.config as RetryConfig | undefined;

    // 401 Unauthorized — attempt token refresh and retry once
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
        // Retry the original request — the response interceptor success
        // path will unwrap the envelope for us.
        const retryResponse = await axiosClient.request(config);
        return retryResponse;
      } catch {
        useAuthStore.getState().logout();
        redirectToLogin();
        throw new FetchError(401, "UNAUTHORIZED", "Session expired");
      }
    }

    // Other HTTP errors — parse error envelope and throw FetchError
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

    // Network error or no response
    throw new FetchError(0, "NETWORK_ERROR", error.message || "Network error");
  },
);
