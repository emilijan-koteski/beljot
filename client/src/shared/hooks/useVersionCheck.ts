import { useEffect, useRef } from "react";
import { useLocation } from "react-router";

import { APP_VERSION, reloadForNewVersion } from "@/shared/lib/appVersion";

const CHECK_INTERVAL_MS = 5 * 60 * 1000;

const RELOADED_FOR_KEY = "beljot:version-reloaded-for";

function isMatchRoute(pathname: string): boolean {
  return pathname.startsWith("/match/");
}

function reloadOncePerVersion(version: string): void {
  if (sessionStorage.getItem(RELOADED_FOR_KEY) === version) return;
  sessionStorage.setItem(RELOADED_FOR_KEY, version);
  reloadForNewVersion();
}

/**
 * Reloads the tab when /version.json reports a build newer than this bundle, deferring the
 * reload while on a match route. No-ops in dev; mount once near the app root.
 */
export function useVersionCheck(): void {
  const { pathname } = useLocation();
  const inMatchRef = useRef(isMatchRoute(pathname));
  inMatchRef.current = isMatchRoute(pathname);
  const pendingVersionRef = useRef<string | null>(null);

  useEffect(() => {
    if (pendingVersionRef.current && !isMatchRoute(pathname)) {
      reloadOncePerVersion(pendingVersionRef.current);
    }
  }, [pathname]);

  useEffect(() => {
    if (APP_VERSION === "dev") return;

    let cancelled = false;

    const check = async () => {
      try {
        const res = await fetch("/version.json", { cache: "no-store" });
        if (!res.ok) return;
        const { version } = (await res.json()) as { version?: string };
        if (cancelled || !version || version === APP_VERSION) return;
        if (inMatchRef.current) {
          pendingVersionRef.current = version;
          return;
        }
        reloadOncePerVersion(version);
      } catch {
        return;
      }
    };

    const intervalId = setInterval(() => void check(), CHECK_INTERVAL_MS);
    const onVisibilityChange = () => {
      if (document.visibilityState === "visible") void check();
    };
    const onOnline = () => void check();
    document.addEventListener("visibilitychange", onVisibilityChange);
    window.addEventListener("online", onOnline);

    return () => {
      cancelled = true;
      clearInterval(intervalId);
      document.removeEventListener("visibilitychange", onVisibilityChange);
      window.removeEventListener("online", onOnline);
    };
  }, []);
}
