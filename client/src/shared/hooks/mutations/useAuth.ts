import { useMutation } from "@tanstack/react-query";

import type {
  ForgotPasswordRequest,
  LoginRequest,
  RegisterRequest,
  ResetPasswordRequest,
} from "@/shared/api/auth";
import {
  forgotPassword,
  login,
  register,
  resetPassword,
  ssoLink,
  ssoLogin,
} from "@/shared/api/auth";
import { useAuthStore } from "@/shared/stores/authStore";

function setAuthState(res: {
  token: string;
  id: number;
  username: string;
  email: string;
  languagePreference: string;
  walletBalance: number;
  loginStreakDays: number;
  totalXp: number;
  level: number;
  createdAt: string;
}) {
  useAuthStore.getState().setToken(res.token);
  useAuthStore.getState().setUser({
    id: res.id,
    username: res.username,
    email: res.email,
    languagePreference: res.languagePreference,
    walletBalance: res.walletBalance,
    loginStreakDays: res.loginStreakDays,
    totalXp: res.totalXp,
    level: res.level,
    createdAt: res.createdAt,
  });
}

export function useLoginMutation() {
  return useMutation({
    mutationFn: (data: LoginRequest) => login(data),
    onSuccess: setAuthState,
  });
}

export function useRegisterMutation() {
  return useMutation({
    mutationFn: (data: RegisterRequest) => register(data),
    onSuccess: setAuthState,
  });
}

export interface SSOLoginVariables {
  provider: string;
  credential: string;
}

export interface SSOLinkVariables {
  provider: string;
  credential: string;
  password: string;
}

// SSO mutations mirror login/register: on success the server envelope is the
// same auth payload, so the same setAuthState hydrates token + user.
export function useSSOLoginMutation() {
  return useMutation({
    mutationFn: ({ provider, credential }: SSOLoginVariables) => ssoLogin(provider, { credential }),
    onSuccess: setAuthState,
  });
}

export function useSSOLinkMutation() {
  return useMutation({
    mutationFn: ({ provider, credential, password }: SSOLinkVariables) =>
      ssoLink(provider, { credential, password }),
    onSuccess: setAuthState,
  });
}

// Password-reset mutations intentionally do not touch auth state — the flow is
// unauthenticated and ends by sending the user to /login.
export function useForgotPasswordMutation() {
  return useMutation({
    mutationFn: (data: ForgotPasswordRequest) => forgotPassword(data),
  });
}

export function useResetPasswordMutation() {
  return useMutation({
    mutationFn: (data: ResetPasswordRequest) => resetPassword(data),
  });
}
