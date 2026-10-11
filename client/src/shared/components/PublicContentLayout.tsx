import { Outlet } from "react-router";

import { PublicTopBar } from "@/shared/components/PublicTopBar";
import { TopBar } from "@/shared/components/TopBar";
import { WebSocketProvider } from "@/shared/providers/WebSocketProvider";
import { useAuthStore } from "@/shared/stores/authStore";

/**
 * Layout for pages reachable by everyone, such as Rules: authed visitors get the full app shell
 * inside the WebSocketProvider, logged-out visitors get the public header with sign-in CTAs.
 */
export function PublicContentLayout() {
  const token = useAuthStore((s) => s.token);
  const isLoading = useAuthStore((s) => s.isLoading);

  if (isLoading) {
    return null;
  }

  if (token) {
    return (
      <WebSocketProvider>
        <div className="min-h-screen">
          <TopBar showNav showUserMenu persistLanguage />
          <main>
            <Outlet />
          </main>
        </div>
      </WebSocketProvider>
    );
  }

  return (
    <div className="min-h-screen">
      <PublicTopBar />
      <main>
        <Outlet />
      </main>
    </div>
  );
}
