import { create } from "zustand";

import { logout as logoutApi } from "@/shared/api/auth";
import { useChatStore } from "@/shared/stores/chatStore";
import { useMatchStore } from "@/shared/stores/matchStore";
import { useRankChangeStore } from "@/shared/stores/rankChangeStore";
import { useRoomStore } from "@/shared/stores/roomStore";
import type { User } from "@/shared/types/apiTypes";

interface AuthState {
  token: string | null;
  user: User | null;
  isLoading: boolean;
  setToken: (token: string | null) => void;
  setUser: (user: User) => void;
  setLoading: (loading: boolean) => void;
  logout: () => void;
}

/**
 * Wipes every session-scoped store, including notices meant for the outgoing account.
 * Runs before auth state clears so token subscribers never see stale data.
 */
function clearSessionStores() {
  useMatchStore.getState().clearGame();
  useRoomStore.getState().clearSessionNotices();
  useChatStore.getState().clearLobby();
  useChatStore.getState().clearMatch();
  useChatStore.getState().clearRoom();
  useChatStore.getState().clearWhispers();
  useRankChangeStore.getState().clear();
}

export const useAuthStore = create<AuthState>((set, get) => ({
  token: null,
  user: null,
  isLoading: true,
  setToken: (token) => set({ token }),
  setUser: (user) => set({ user }),
  setLoading: (isLoading) => set({ isLoading }),
  logout: () => {
    if (get().token) {
      logoutApi();
    }
    clearSessionStores();
    set({ token: null, user: null, isLoading: false });
  },
}));
