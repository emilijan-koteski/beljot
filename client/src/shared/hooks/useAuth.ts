import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate } from "react-router";

import { refreshAccessToken, setAuthRedirect } from "@/shared/api/axiosClient";
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

    let cancelled = false;

    // Restore through the COORDINATED singleton, never POST /auth/refresh
    // directly. Refresh tokens ROTATE, so two uncoordinated restores burn two
    // generations of the family and the browser keeps whichever Set-Cookie
    // lands last — which can be the superseded one. Nothing looks wrong until
    // the next refresh presents that dead token, gets a 401, and the server
    // clears the cookie, taking the session with it. A cancelled request does
    // NOT prevent this: the server has already rotated by the time the client
    // aborts. The singleton instead collapses concurrent restores (StrictMode's
    // double mount, a 401 landing mid-restore, a second tab) into one request,
    // sets token + user, and broadcasts the result to sibling tabs.
    refreshAccessToken()
      .then(
        async () => {
          if (cancelled) return;
          // Await the language switch so the first paint after `setLoading(false)`
          // already renders in the saved locale — prevents an English flash on
          // bootstrap when the user's preference is mk/hr/sr (AC #4 of story 10.1).
          //
          // Swallowed deliberately: the session is ALREADY restored here, so a
          // locale switch that fails must not fall through to the rejection arm
          // and clear the token. Worst case the user reads English.
          const lang = useAuthStore.getState().user?.languagePreference;
          if (lang) await i18n.changeLanguage(lang).catch(() => undefined);
        },
        // Rejection arm rather than a trailing .catch(), so it only ever sees a
        // FAILED RESTORE — never an error thrown by the success work above.
        () => {
          if (cancelled) return;
          // Clear local state only — don't call logout() which fires an API
          // request to /api/v1/auth/logout when no session exists yet.
          useAuthStore.getState().setToken(null);
        },
      )
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
