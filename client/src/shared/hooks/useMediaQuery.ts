import { useEffect, useState } from "react";

/**
 * Subscribes to a CSS media query and returns its live match state. Reactive —
 * resizing across the breakpoint updates consumers on the next render. SSR/test
 * safe (returns false when `window`/`matchMedia` is unavailable).
 *
 * Used for layout that can't be expressed with Tailwind breakpoints alone —
 * e.g. the in-match seats, whose pixel geometry is driven from JS rather than
 * class names.
 */
export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState<boolean>(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
      return false;
    }
    return window.matchMedia(query).matches;
  });

  useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
      return;
    }
    const mql = window.matchMedia(query);
    const listener = (e: MediaQueryListEvent) => setMatches(e.matches);
    setMatches(mql.matches);
    mql.addEventListener("change", listener);
    return () => mql.removeEventListener("change", listener);
  }, [query]);

  return matches;
}
