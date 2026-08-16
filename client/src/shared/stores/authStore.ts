import { create } from "zustand";

import { logout as logoutApi } from "@/shared/api/auth";
import { useChatStore } from "@/shared/stores/chatStore";
import { useMatchStore } from "@/shared/stores/matchStore";
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
    // Wipe session-scoped stores BEFORE clearing auth state so synchronous
    // subscribers to authStore.token transitions don't observe stale game
    // data. Single canonical logout point — both manual logout and the
    // axiosClient 401 interceptor inherit this ordering.
    useMatchStore.getState().clearGame();
    // clearSessionNotices, not reset: reset() preserves roomEjection/roomInvite
    // on purpose so they survive a mid-navigation unmount, but they belong to the
    // session that just ended and must not greet the next account in this tab.
    useRoomStore.getState().clearSessionNotices();
    useChatStore.getState().clearLobby();
    useChatStore.getState().clearMatch();
    useChatStore.getState().clearRoom();
    useChatStore.getState().clearWhispers();
    set({ token: null, user: null, isLoading: false });
  },
}));
