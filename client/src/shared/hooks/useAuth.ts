import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate } from "react-router";

import { refreshAccessToken, setAuthRedirect } from "@/shared/api/axiosClient";
import { useAuthStore } from "@/shared/stores/authStore";

const GUEST_PATHS = ["/login", "/register"];
const LOGGED_OUT_HOME = "/";

/**
 * Restores the session on mount via the coordinated refreshAccessToken singleton, never a direct refresh call.
 * Refresh tokens rotate, so a second uncoordinated restore can leave the browser holding a superseded cookie.
 */
export function useAuthInit(): void {
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const { i18n } = useTranslation();

  useEffect(() => {
    setAuthRedirect(() => navigate(LOGGED_OUT_HOME));
  }, [navigate]);

  useEffect(() => {
    const token = useAuthStore.getState().token;
    if (token) {
      useAuthStore.getState().setLoading(false);
      return;
    }

    if (GUEST_PATHS.includes(pathname)) {
      useAuthStore.getState().setLoading(false);
      return;
    }

    let cancelled = false;

    const applySavedLocaleBeforeFirstPaint = async () => {
      if (cancelled) return;
      const lang = useAuthStore.getState().user?.languagePreference;
      const ignoreLocaleFailureSessionIsRestored = () => undefined;
      if (lang) await i18n.changeLanguage(lang).catch(ignoreLocaleFailureSessionIsRestored);
    };

    const clearLocalTokenWithoutLogoutRequest = () => {
      if (cancelled) return;
      useAuthStore.getState().setToken(null);
    };

    refreshAccessToken()
      .then(applySavedLocaleBeforeFirstPaint, clearLocalTokenWithoutLogoutRequest)
      .finally(() => {
        if (cancelled) return;
        useAuthStore.getState().setLoading(false);
      });

    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
}
