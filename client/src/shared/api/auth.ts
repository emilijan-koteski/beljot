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
  createdAt: string;
}

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
