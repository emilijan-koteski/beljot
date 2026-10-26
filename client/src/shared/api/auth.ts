import type { AxiosError } from "axios";

import { axiosPublic, FetchError } from "@/shared/api/axiosClient";
import type { CardDeck } from "@/shared/types/matchTypes";

export interface RegisterRequest {
  email: string;
  username: string;
  password: string;
  languagePreference: string;
}

interface RegisterResponse {
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
}

export interface LoginRequest {
  email: string;
  password: string;
}

type LoginResponse = RegisterResponse;

interface SSOLoginRequest {
  credential: string;
}

interface SSOLinkRequest {
  credential: string;
  password: string;
}

/** SSO issues the same session and auth envelope as password login. */
type SSOResponse = RegisterResponse;

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

/** Maps an axios failure onto FetchError, falling back to UNKNOWN_ERROR without an { error } body. */
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
