import { useCallback, useEffect } from "react";
import { useNavigate } from "react-router";

// sessionStorage key holding the history index of the lobby entry. Session-
// scoped on purpose: history indexes are only meaningful within one tab's
// history stack, and sessionStorage shares exactly that lifetime (it survives
// reloads, where react-router restores the same idx from history.state).
const LOBBY_IDX_KEY = "beljot:lobby-idx";

/**
 * Reads react-router's history index from `window.history.state.idx`. The
 * data router maintains it on every push/replace/pop (semi-internal but
 * stable). Returns null when unavailable (memory routers in tests, a fresh
 * jsdom, non-numeric garbage) so callers fall back to the replace path.
 */
function currentHistoryIdx(): number | null {
  const state = window.history.state as { idx?: unknown } | null | undefined;
  const idx = state?.idx;
  return typeof idx === "number" && Number.isInteger(idx) ? idx : null;
}

function storedLobbyIdx(): number | null {
  try {
    const raw = sessionStorage.getItem(LOBBY_IDX_KEY);
    // Explicit empty-string check: Number("") is 0, which would read as a
    // valid lobby at index 0.
    if (raw === null || raw.trim() === "") return null;
    const idx = Number(raw);
    return Number.isInteger(idx) ? idx : null;
  } catch {
    return null;
  }
}

/**
 * True when the entry directly beneath the current one was recorded as the
 * lobby (its index equals current − 1) — i.e. a plain `navigate(-1)` pop is
 * expected to land on the live lobby entry. This is a heuristic, not a
 * proof: the recorded index is refreshed on every lobby mount but never
 * invalidated, so exotic below-root traversal can leave it stale. Any
 * missing or inconsistent value reads false, so returns degrade to the
 * replace fallback (at worst a duplicate lobby entry, never a dead match
 * entry).
 */
export function canPopToLobby(): boolean {
  const current = currentHistoryIdx();
  const lobby = storedLobbyIdx();
  return current !== null && lobby !== null && lobby === current - 1;
}

/**
 * Marks the current history entry as the lobby root. Call from LobbyPage so
 * every later `returnToLobby()` can decide between popping back to this
 * entry and replacing. Re-runs on every lobby mount (push, pop or replace
 * arrival), keeping the recorded index in sync with the live entry.
 */
export function useMarkLobbyRoot(): void {
  useEffect(() => {
    const idx = currentHistoryIdx();
    if (idx === null) return;
    try {
      sessionStorage.setItem(LOBBY_IDX_KEY, String(idx));
    } catch {
      // Storage unavailable → canPopToLobby() stays false and every return
      // takes the replace fallback. Benign.
    }
  }, []);
}

// Re-entrancy guard: a pop-based return is "in flight" until its popstate
// lands. Unlike the old push-to-lobby, `navigate(-1)` is NOT idempotent — a
// double-tap on a leave button (or a concurrent effect) firing two pops would
// step past the lobby root and back out of the app. Replace-based returns
// are idempotent and need no guard.
let popPending = false;

/** Test-only: clears the module-level pop guard between tests. */
export function resetLobbyReturnGuardForTests(): void {
  popPending = false;
}

/**
 * Returns a stable `returnToLobby()` callback: pops back to the lobby entry
 * when it sits directly beneath the current one (keeping `/lobby` the app
 * root with nothing stacked above), otherwise replaces the current entry
 * with `/lobby` (deep links, fresh reconnects — no lobby beneath).
 */
export function useLobbyReturn(): () => void {
  const navigate = useNavigate();
  return useCallback(() => {
    if (popPending) return;
    if (canPopToLobby()) {
      popPending = true;
      window.addEventListener(
        "popstate",
        () => {
          popPending = false;
        },
        { once: true },
      );
      void navigate(-1);
    } else {
      void navigate("/lobby", { replace: true });
    }
  }, [navigate]);
}
