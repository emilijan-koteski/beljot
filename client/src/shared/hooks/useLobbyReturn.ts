import { useCallback, useEffect } from "react";
import { useNavigate } from "react-router";

const LOBBY_IDX_KEY = "beljot:lobby-idx";

/**
 * Reads react-router's history index from `window.history.state.idx`.
 * Returns null when unavailable so callers fall back to the replace path.
 */
function currentHistoryIdx(): number | null {
  const state = window.history.state as { idx?: unknown } | null | undefined;
  const idx = state?.idx;
  return typeof idx === "number" && Number.isInteger(idx) ? idx : null;
}

function storedLobbyIdx(): number | null {
  try {
    const raw = sessionStorage.getItem(LOBBY_IDX_KEY);
    if (raw === null || raw.trim() === "") return null;
    const idx = Number(raw);
    return Number.isInteger(idx) ? idx : null;
  } catch {
    return null;
  }
}

/**
 * True when the entry directly beneath the current one was recorded as the lobby,
 * so a plain `navigate(-1)` should land on it. Any missing or inconsistent value reads false.
 */
export function canPopToLobby(): boolean {
  const current = currentHistoryIdx();
  const lobby = storedLobbyIdx();
  return current !== null && lobby !== null && lobby === current - 1;
}

/**
 * Marks the current history entry as the lobby root. Call from LobbyPage so
 * later `returnToLobby()` calls can decide between popping back and replacing.
 */
export function useMarkLobbyRoot(): void {
  useEffect(() => {
    const idx = currentHistoryIdx();
    if (idx === null) return;
    try {
      sessionStorage.setItem(LOBBY_IDX_KEY, String(idx));
    } catch {
      return;
    }
  }, []);
}

let popPending = false;

/** Test-only: clears the module-level pop guard between tests. */
export function resetLobbyReturnGuardForTests(): void {
  popPending = false;
}

/**
 * Returns a stable `returnToLobby()` callback that pops back to the lobby entry when it sits
 * directly beneath, otherwise replaces with `/lobby`. Repeat calls are ignored until the pop lands.
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
