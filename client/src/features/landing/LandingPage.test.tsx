import "@/shared/i18n/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it } from "vitest";

import { LandingPage } from "@/features/landing/LandingPage";
import { GuestRoute } from "@/shared/components/GuestRoute";
import { useAuthStore } from "@/shared/stores/authStore";

// Mirror the real App route tree: `/` is public-only (wrapped in GuestRoute).
// A QueryClient is needed because the hero polls public stats via useQuery.
function renderAt(path: string) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route element={<GuestRoute />}>
            <Route path="/" element={<LandingPage />} />
          </Route>
          <Route path="/lobby" element={<div data-testid="lobby-page">Lobby</div>} />
          <Route path="/login" element={<div data-testid="login-page">Login</div>} />
          <Route path="/register" element={<div data-testid="register-page">Register</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("LandingPage", () => {
  beforeEach(() => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  it("renders the landing page for a guest at /", () => {
    renderAt("/");

    expect(screen.getByTestId("landing-page")).toBeInTheDocument();
    expect(screen.getByTestId("landing-hero-cta")).toBeInTheDocument();
  });

  it("points the CTAs at the auth pages", () => {
    renderAt("/");

    expect(screen.getByTestId("landing-login")).toHaveAttribute("href", "/login");
    expect(screen.getByTestId("landing-signup")).toHaveAttribute("href", "/register");
    expect(screen.getByTestId("landing-hero-cta")).toHaveAttribute("href", "/register");
    expect(screen.getByTestId("landing-cta")).toHaveAttribute("href", "/register");
  });

  it("redirects an authenticated user away from / to /lobby", () => {
    useAuthStore.setState({ token: "test-token", isLoading: false });
    renderAt("/");

    expect(screen.getByTestId("lobby-page")).toBeInTheDocument();
    expect(screen.queryByTestId("landing-page")).not.toBeInTheDocument();
  });

  it("links the footer to the public reference + legal pages", () => {
    renderAt("/");

    expect(screen.getByRole("link", { name: "Rules" })).toHaveAttribute("href", "/rules");
    expect(screen.getByRole("link", { name: "Privacy" })).toHaveAttribute("href", "/privacy");
    expect(screen.getByRole("link", { name: "Terms" })).toHaveAttribute("href", "/terms");
  });

  it("opens a contact dialog with email, LinkedIn and a beta disclaimer", async () => {
    const user = userEvent.setup();
    renderAt("/");

    await user.click(screen.getByTestId("landing-contact"));

    expect(await screen.findByTestId("contact-email")).toHaveAttribute(
      "href",
      "mailto:emilijankoteski@pm.me",
    );
    expect(screen.getByTestId("contact-linkedin")).toHaveAttribute(
      "href",
      "https://www.linkedin.com/in/emilijan-koteski/",
    );
    expect(screen.getByText(/beta build/i)).toBeInTheDocument();
  });

  it("opens the support dialog from the footer", async () => {
    const user = userEvent.setup();
    renderAt("/");

    // A button, not a link — it opens the in-app explainer rather than
    // navigating, so the Rules/Privacy/Terms link queries above stay unaffected.
    await user.click(screen.getByTestId("support-footer-link"));

    expect(await screen.findByTestId("support-dialog")).toBeInTheDocument();
    expect(screen.getByTestId("support-cta")).toHaveAttribute(
      "href",
      "https://buymeacoffee.com/emilijan",
    );
  });
});
