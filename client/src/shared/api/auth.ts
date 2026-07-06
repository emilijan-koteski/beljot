import type { AxiosError } from "axios";

import { axiosPublic, FetchError } from "@/shared/api/axiosClient";

export interface RegisterRequest {
  email: string;
  username: string;
  password: string;
  languagePreference: string;
}

export interface RegisterResponse {
  token: string;
  id: number;
  username: string;
  email: string;
  languagePreference: string;
  walletBalance: number;
  loginStreakDays: number;
  // XP & level (Story 9.5) — echoed on auth so the top-nav banner has them on
  // load without a separate profile fetch. level is derived server-side.
  totalXp: number;
  level: number;
  createdAt: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export type LoginResponse = RegisterResponse;

export interface RefreshResponse {
  token: string;
  id: number;
  username: string;
  email: string;
  languagePreference: string;
  walletBalance: number;
  loginStreakDays: number;
  // XP & level (Story 9.5) — see RegisterResponse.
  totalXp: number;
  level: number;
  createdAt: string;
}

export interface SSOLoginRequest {
  credential: string;
}

export interface SSOLinkRequest {
  credential: string;
  password: string;
}

// SSO issues the exact same session + auth envelope as password login.
export type SSOResponse = RegisterResponse;

export interface ForgotPasswordRequest {
  email: string;
}

export interface ResetPasswordRequest {
  token: string;
  password: string;
}

export async function register(data: RegisterRequest): Promise<RegisterResponse> {
  try {
    const response = await axiosPublic.post<{ data: RegisterResponse }>("/auth/register", data);
    return response.data.data;
  } catch (e) {
    const err = e as AxiosError<{ error: { code: string; message: string } }>;
    if (err.response?.data?.error) {
      throw new FetchError(
        err.response.status,
        err.response.data.error.code,
        err.response.data.error.message,
      );
    }
    throw new FetchError(
      err.response?.status ?? 0,
      "UNKNOWN_ERROR",
      err.response?.statusText ?? "Registration failed",
    );
  }
}

export async function login(data: LoginRequest): Promise<LoginResponse> {
  try {
    const response = await axiosPublic.post<{ data: LoginResponse }>("/auth/login", data);
    return response.data.data;
  } catch (e) {
    const err = e as AxiosError<{ error: { code: string; message: string } }>;
    if (err.response?.data?.error) {
      throw new FetchError(
        err.response.status,
        err.response.data.error.code,
        err.response.data.error.message,
      );
    }
    throw new FetchError(
      err.response?.status ?? 0,
      "UNKNOWN_ERROR",
      err.response?.statusText ?? "Login failed",
    );
  }
}

// toFetchError maps an axios failure onto the standard error envelope, falling
// back to UNKNOWN_ERROR when the response carries no { error } body.
function toFetchError(e: unknown, fallbackMessage: string): FetchError {
  const err = e as AxiosError<{ error: { code: string; message: string } }>;
  if (err.response?.data?.error) {
    return new FetchError(
      err.response.status,
      err.response.data.error.code,
      err.response.data.error.message,
    );
  }
  return new FetchError(
    err.response?.status ?? 0,
    "UNKNOWN_ERROR",
    err.response?.statusText ?? fallbackMessage,
  );
}

export async function ssoLogin(provider: string, data: SSOLoginRequest): Promise<SSOResponse> {
  try {
    const response = await axiosPublic.post<{ data: SSOResponse }>(`/auth/sso/${provider}`, data);
    return response.data.data;
  } catch (e) {
    throw toFetchError(e, "Sign-in failed");
  }
}

export async function ssoLink(provider: string, data: SSOLinkRequest): Promise<SSOResponse> {
  try {
    const response = await axiosPublic.post<{ data: SSOResponse }>(
      `/auth/sso/${provider}/link`,
      data,
    );
    return response.data.data;
  } catch (e) {
    throw toFetchError(e, "Sign-in failed");
  }
}

export async function refresh(signal?: AbortSignal): Promise<RefreshResponse> {
  try {
    const response = await axiosPublic.post<{ data: RefreshResponse }>("/auth/refresh", undefined, {
      signal,
    });
    return response.data.data;
  } catch (e) {
    throw new Error(`Refresh failed: ${(e as AxiosError).response?.status ?? "unknown"}`);
  }
}

export async function forgotPassword(data: ForgotPasswordRequest): Promise<void> {
  try {
    await axiosPublic.post("/auth/forgot-password", data);
  } catch (e) {
    const err = e as AxiosError<{ error: { code: string; message: string } }>;
    if (err.response?.data?.error) {
      throw new FetchError(
        err.response.status,
        err.response.data.error.code,
        err.response.data.error.message,
      );
    }
    throw new FetchError(
      err.response?.status ?? 0,
      "UNKNOWN_ERROR",
      err.response?.statusText ?? "Request failed",
    );
  }
}

export async function resetPassword(data: ResetPasswordRequest): Promise<void> {
  try {
    await axiosPublic.post("/auth/reset-password", data);
  } catch (e) {
    const err = e as AxiosError<{ error: { code: string; message: string } }>;
    if (err.response?.data?.error) {
      throw new FetchError(
        err.response.status,
        err.response.data.error.code,
        err.response.data.error.message,
      );
    }
    throw new FetchError(
      err.response?.status ?? 0,
      "UNKNOWN_ERROR",
      err.response?.statusText ?? "Request failed",
    );
  }
}

export function logout(): void {
  axiosPublic.post("/auth/logout").catch(() => {});
}
