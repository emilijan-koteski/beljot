import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate } from "react-router";

import { refresh } from "@/shared/api/auth";
import { setAuthRedirect } from "@/shared/api/axiosClient";
import { useAuthStore } from "@/shared/stores/authStore";

const GUEST_PATHS = ["/login", "/register"];

export function useAuthInit(): void {
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const { i18n } = useTranslation();

  useEffect(() => {
    // Auth failures (logout, expired session, failed refresh) land on the
    // public landing page — the canonical logged-out home, with its own
    // prominent "Log in" CTA. Pointing this at "/" (rather than "/login")
    // also means a deliberate logout isn't raced to /login by an in-flight
    // authed request 401-ing as the lobby unmounts.
    setAuthRedirect(() => navigate("/"));
  }, [navigate]);

  useEffect(() => {
    const token = useAuthStore.getState().token;
    if (token) {
      useAuthStore.getState().setLoading(false);
      return;
    }

    // Guest pages (login/register) never need a refresh attempt —
    // there is no session to restore.
    if (GUEST_PATHS.includes(pathname)) {
      useAuthStore.getState().setLoading(false);
      return;
    }

    const abortController = new AbortController();

    refresh(abortController.signal)
      .then(async (res) => {
        if (abortController.signal.aborted) return;
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
        // Await the language switch so the first paint after `setLoading(false)`
        // already renders in the saved locale — prevents an English flash on
        // bootstrap when the user's preference is mk/hr/sr (AC #4 of story 10.1).
        await i18n.changeLanguage(res.languagePreference);
      })
      .catch(() => {
        if (abortController.signal.aborted) return;
        // Clear local state only — don't call logout() which fires an API
        // request to /api/v1/auth/logout when no session exists yet.
        useAuthStore.getState().setToken(null);
      })
      .finally(() => {
        if (abortController.signal.aborted) return;
        useAuthStore.getState().setLoading(false);
      });

    return () => {
      abortController.abort();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
}
