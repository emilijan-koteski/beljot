import { useState } from "react";
import {
  createBrowserRouter,
  createRoutesFromElements,
  Navigate,
  Outlet,
  Route,
  RouterProvider,
} from "react-router";

import { AuthLayout } from "@/features/auth/AuthLayout";
import { ForgotPasswordPage } from "@/features/auth/ForgotPasswordPage";
import { LoginPage } from "@/features/auth/LoginPage";
import { RegisterPage } from "@/features/auth/RegisterPage";
import { ResetPasswordPage } from "@/features/auth/ResetPasswordPage";
import { LandingPage } from "@/features/landing/LandingPage";
import { LeaderboardPage } from "@/features/leaderboard/LeaderboardPage";
import { PrivacyPage } from "@/features/legal/PrivacyPage";
import { TermsPage } from "@/features/legal/TermsPage";
import { LobbyPage } from "@/features/lobby/LobbyPage";
import { MatchmakingPage } from "@/features/lobby/MatchmakingPage";
import { MatchPage } from "@/features/match/MatchPage";
import { ProfilePage } from "@/features/profile/ProfilePage";
import { PublicPlayerProfilePage } from "@/features/profile/PublicPlayerProfilePage";
import { RoomPage } from "@/features/room/RoomPage";
import { RulesPage } from "@/features/rules/RulesPage";
import { AppLayout } from "@/shared/components/AppLayout";
import { GuestRoute } from "@/shared/components/GuestRoute";
import { ProtectedRoute } from "@/shared/components/ProtectedRoute";
import { PublicContentLayout } from "@/shared/components/PublicContentLayout";
import { Toaster } from "@/shared/components/ui/sonner";
import { useAuthInit } from "@/shared/hooks/useAuth";
import { useLayoutBand } from "@/shared/hooks/useLayoutBand";
import { useTokenRefresh } from "@/shared/hooks/useTokenRefresh";
import { useVersionCheck } from "@/shared/hooks/useVersionCheck";
import { QueryProvider } from "@/shared/providers/QueryProvider";
import { useAuthStore } from "@/shared/stores/authStore";

function AuthAwareRedirect() {
  const token = useAuthStore((s) => s.token);
  return <Navigate to={token ? "/lobby" : "/"} replace />;
}

/**
 * Root layout route that runs the app-wide auth bootstrap hooks.
 * Renders nothing while the initial session refresh is in flight.
 */
function AppShell() {
  useAuthInit();
  useTokenRefresh();
  useVersionCheck();

  const isLoading = useAuthStore((s) => s.isLoading);

  if (isLoading) {
    return null;
  }

  return <Outlet />;
}

// Builds the data router that useBlocker needs; called lazily from App() because
// createBrowserRouter writes to window.history on construction.
function createAppRouter() {
  return createBrowserRouter(
    createRoutesFromElements(
      <Route element={<AppShell />}>
        <Route path="/terms" element={<TermsPage />} />
        <Route path="/privacy" element={<PrivacyPage />} />
        <Route element={<GuestRoute />}>
          <Route path="/" element={<LandingPage />} />
          <Route element={<AuthLayout />}>
            <Route path="/login" element={<LoginPage />} />
            <Route path="/register" element={<RegisterPage />} />
            <Route path="/forgot-password" element={<ForgotPasswordPage />} />
            <Route path="/reset-password" element={<ResetPasswordPage />} />
          </Route>
        </Route>
        <Route element={<PublicContentLayout />}>
          <Route path="/rules" element={<RulesPage />} />
        </Route>
        <Route element={<ProtectedRoute />}>
          <Route element={<AppLayout />}>
            <Route path="/lobby" element={<LobbyPage />} />
            <Route path="/profile" element={<ProfilePage />} />
            <Route path="/leaderboard" element={<LeaderboardPage />} />
            <Route path="/players/:id" element={<PublicPlayerProfilePage />} />
            <Route path="/rooms/:id" element={<RoomPage />} />
            <Route path="/matchmaking/:id" element={<MatchmakingPage />} />
          </Route>
          <Route path="/match/:roomId" element={<MatchPage />} />
        </Route>
        <Route path="*" element={<AuthAwareRedirect />} />
      </Route>,
    ),
  );
}

/** The app's toaster: at the top on a phone, where nothing pinned to the bottom can cover it, else bottom centre. */
function AppToaster() {
  const band = useLayoutBand();
  return <Toaster position={band === "phone" ? "top-center" : "bottom-center"} />;
}

export function App() {
  const [router] = useState(createAppRouter);
  return (
    <QueryProvider>
      <RouterProvider router={router} />
      <AppToaster />
    </QueryProvider>
  );
}
