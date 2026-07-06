import { BrowserRouter, Navigate, Route, Routes } from "react-router";

import { AuthLayout } from "@/features/auth/AuthLayout";
import { ForgotPasswordPage } from "@/features/auth/ForgotPasswordPage";
import { LoginPage } from "@/features/auth/LoginPage";
import { RegisterPage } from "@/features/auth/RegisterPage";
import { ResetPasswordPage } from "@/features/auth/ResetPasswordPage";
import { LandingPage } from "@/features/landing/LandingPage";
import { PrivacyPage } from "@/features/legal/PrivacyPage";
import { TermsPage } from "@/features/legal/TermsPage";
import { LobbyPage } from "@/features/lobby/LobbyPage";
import { MatchmakingPage } from "@/features/lobby/MatchmakingPage";
import { MatchPage } from "@/features/match/MatchPage";
import { ProfilePage } from "@/features/profile/ProfilePage";
import { RoomPage } from "@/features/room/RoomPage";
import { RulesPage } from "@/features/rules/RulesPage";
import { AppLayout } from "@/shared/components/AppLayout";
import { GuestRoute } from "@/shared/components/GuestRoute";
import { ProtectedRoute } from "@/shared/components/ProtectedRoute";
import { PublicContentLayout } from "@/shared/components/PublicContentLayout";
import { Toaster } from "@/shared/components/ui/sonner";
import { useAuthInit } from "@/shared/hooks/useAuth";
import { useTokenRefresh } from "@/shared/hooks/useTokenRefresh";
import { QueryProvider } from "@/shared/providers/QueryProvider";
import { useAuthStore } from "@/shared/stores/authStore";

function AuthAwareRedirect() {
  const token = useAuthStore((s) => s.token);
  return <Navigate to={token ? "/lobby" : "/"} replace />;
}

function AppRoutes() {
  useAuthInit();
  useTokenRefresh();

  const isLoading = useAuthStore((s) => s.isLoading);

  if (isLoading) {
    return null;
  }

  return (
    <Routes>
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
      {/* Public reference pages — reachable by guests (from the landing footer)
          and authed users alike. The layout adapts to auth state. */}
      <Route element={<PublicContentLayout />}>
        <Route path="/rules" element={<RulesPage />} />
      </Route>
      <Route element={<ProtectedRoute />}>
        <Route element={<AppLayout />}>
          <Route path="/lobby" element={<LobbyPage />} />
          <Route path="/profile" element={<ProfilePage />} />
          <Route path="/rooms/:id" element={<RoomPage />} />
          <Route path="/matchmaking/:id" element={<MatchmakingPage />} />
        </Route>
        <Route path="/match/:roomId" element={<MatchPage />} />
      </Route>
      <Route path="*" element={<AuthAwareRedirect />} />
    </Routes>
  );
}

export function App() {
  return (
    <QueryProvider>
      <BrowserRouter>
        <AppRoutes />
      </BrowserRouter>
      {/* Single app-wide sonner host. Without this mounted, every toast.*()
          call (join/settlement feedback, auth errors, etc.) is silently
          invisible — the Toaster component existed but was never rendered.
          Bottom-center keeps toasts clear of the bottom-right chat FAB. */}
      <Toaster position="bottom-center" />
    </QueryProvider>
  );
}
