import { useEffect, useRef } from "react";
import { useLocation } from "react-router";

import { APP_VERSION, reloadForNewVersion } from "@/shared/lib/appVersion";

// How often a live tab compares itself against the deployed version. Checks
// also fire when a hidden tab becomes visible again and when the device comes
// back online — the two moments a long-slept tab is most likely stale.
const CHECK_INTERVAL_MS = 5 * 60 * 1000;

// Remembers which version this tab already reloaded for. If an intermediary
// cache keeps serving the old index.html even after a reload, we'd otherwise
// reload forever; sessionStorage survives reloads but stays per-tab.
const RELOADED_FOR_KEY = "beljot:version-reloaded-for";

// An active match is the one place a surprise reload hurts (table state only
// partially resyncs), so the reload is deferred until the player leaves the
// match screen. "/match/" (not "/match") so /matchmaking/:id isn't caught.
function isMatchRoute(pathname: string): boolean {
  return pathname.startsWith("/match/");
}

function reloadOncePerVersion(version: string): void {
  if (sessionStorage.getItem(RELOADED_FOR_KEY) === version) return;
  sessionStorage.setItem(RELOADED_FOR_KEY, version);
  reloadForNewVersion();
}

// useVersionCheck reloads the tab when a newer build has been deployed: it
// polls /version.json (emitted at build time with the commit SHA) and compares
// against the SHA baked into this bundle. Combined with the Caddy cache rules
// (index.html no-cache, /assets immutable) a plain reload is equivalent to a
// hard refresh. No-ops in dev, where there is no version.json and hot reload
// handles freshness. Mount once, near the app root.
export function useVersionCheck(): void {
  const { pathname } = useLocation();
  const inMatchRef = useRef(isMatchRoute(pathname));
  inMatchRef.current = isMatchRoute(pathname);
  const pendingVersionRef = useRef<string | null>(null);

  // A reload deferred during a match fires as soon as the route leaves it.
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
        // Offline or a mid-deploy blip — the next scheduled check retries.
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
