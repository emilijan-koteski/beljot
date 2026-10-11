import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it } from "vitest";

import { PublicContentLayout } from "@/shared/components/PublicContentLayout";
import { useAuthStore } from "@/shared/stores/authStore";

function renderLayout() {
  return render(
    <MemoryRouter initialEntries={["/rules"]}>
      <Routes>
        <Route element={<PublicContentLayout />}>
          <Route path="/rules" element={<div data-testid="content">Rules content</div>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
}

describe("PublicContentLayout (guest)", () => {
  beforeEach(() => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  it("renders the public header with sign-in CTAs and the routed content", () => {
    renderLayout();

    expect(screen.getByTestId("public-nav")).toBeInTheDocument();
    expect(screen.getByTestId("public-login")).toHaveAttribute("href", "/login");
    expect(screen.getByTestId("public-signup")).toHaveAttribute("href", "/register");
    expect(screen.getByTestId("public-logo")).toHaveAttribute("href", "/");
    expect(screen.getByTestId("content")).toBeInTheDocument();
    expect(screen.queryByTestId("app-nav")).not.toBeInTheDocument();
  });
});
