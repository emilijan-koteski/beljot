import "@/shared/i18n/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { LandingPage } from "@/features/landing/LandingPage";
import { GuestRoute } from "@/shared/components/GuestRoute";
import { BRASS_MODE_CLASS } from "@/shared/lib/themePreference";
import { useAuthStore } from "@/shared/stores/authStore";
import { useGuestThemeStore } from "@/shared/stores/themeStore";

/** Mirrors the real route tree: `/` is guest-only, wrapped in GuestRoute. */
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
    useGuestThemeStore.setState({ theme: "parchment" });
  });

  afterEach(() => {
    document.documentElement.classList.remove(BRASS_MODE_CLASS);
    window.localStorage.clear();
    window.sessionStorage.clear();
  });

  it("renders every section for a guest at /", () => {
    renderAt("/");

    expect(screen.getByTestId("landing-page")).toBeInTheDocument();
    for (const section of ["hero", "facts", "lobby", "rules", "table", "seasons", "closing"]) {
      expect(screen.getByTestId(`landing-${section}`)).toBeInTheDocument();
    }
  });

  it("points the CTAs at the auth pages", () => {
    renderAt("/");

    expect(screen.getByTestId("landing-login")).toHaveAttribute("href", "/login");
    expect(screen.getByTestId("landing-signup")).toHaveAttribute("href", "/register");
    expect(screen.getByTestId("landing-hero-cta")).toHaveAttribute("href", "/register");
    expect(screen.getByTestId("landing-cta")).toHaveAttribute("href", "/register");
  });

  it("shows both rulebooks with their own bidding", () => {
    renderAt("/");

    const rules = screen.getByTestId("landing-rules");
    expect(within(rules).getByRole("heading", { name: "Bitola" })).toBeInTheDocument();
    expect(within(rules).getByRole("heading", { name: "Croatian" })).toBeInTheDocument();
    expect(within(rules).getByText("All pass? The dealer picks trump.")).toBeInTheDocument();
  });

  it("shows the lobby screenshot in the visitor's theme", async () => {
    const user = userEvent.setup();
    renderAt("/");

    const lobby = screen.getByTestId("landing-lobby");
    const [shot] = within(lobby).getAllByRole("img", { name: "The lobby with open rooms" });
    expect(shot).toHaveAttribute("src", expect.stringContaining("lobby-phone-parchment.webp"));

    await user.click(screen.getByTestId("landing-theme-toggle"));

    expect(document.documentElement).toHaveClass(BRASS_MODE_CLASS);
    const [brassShot] = within(lobby).getAllByRole("img", { name: "The lobby with open rooms" });
    expect(brassShot).toHaveAttribute("src", expect.stringContaining("lobby-phone-brass.webp"));
  });

  it("redirects an authenticated user away from / to /lobby", () => {
    useAuthStore.setState({ token: "test-token", isLoading: false });
    renderAt("/");

    expect(screen.getByTestId("lobby-page")).toBeInTheDocument();
    expect(screen.queryByTestId("landing-page")).not.toBeInTheDocument();
  });

  it("links the footer to the public reference and legal pages", () => {
    renderAt("/");

    const footer = screen.getByRole("contentinfo");
    expect(within(footer).getByRole("link", { name: "Rules" })).toHaveAttribute("href", "/rules");
    expect(within(footer).getByRole("link", { name: "Privacy" })).toHaveAttribute(
      "href",
      "/privacy",
    );
    expect(within(footer).getByRole("link", { name: "Terms" })).toHaveAttribute("href", "/terms");
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

    await user.click(screen.getByTestId("support-footer-link"));

    expect(await screen.findByTestId("support-dialog")).toBeInTheDocument();
    expect(screen.getByTestId("support-cta")).toHaveAttribute(
      "href",
      "https://buymeacoffee.com/emilijan",
    );
  });
});
