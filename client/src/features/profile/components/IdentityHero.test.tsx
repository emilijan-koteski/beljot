import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { IdentityHero } from "./IdentityHero";

type HeroProps = Parameters<typeof IdentityHero>[0];

function renderHero(overrides: Partial<HeroProps> = {}) {
  const props: HeroProps = {
    username: "kiro",
    createdAt: "2026-01-01T00:00:00Z",
    games: 10,
    wins: 6,
    losses: 4,
    capots: 1,
    walletBalance: 5000,
    loginStreakDays: 0,
    level: 3,
    xpIntoLevel: 150,
    xpForNextLevel: 350,
    winRate: 60,
    ...overrides,
  };
  return render(<IdentityHero {...props} />);
}

describe("IdentityHero XP (Story 9.5)", () => {
  it("renders the level label and XP progress bar from the profile", () => {
    renderHero({ level: 3, xpIntoLevel: 150, xpForNextLevel: 350 });

    expect(screen.getByTestId("profile-level")).toHaveTextContent("Level 3");
    expect(screen.getByTestId("profile-xp")).toHaveTextContent("150 / 350 XP");
    // 150 / 350 → round(42.857) = 43%.
    expect(screen.getByTestId("profile-xp-bar")).toHaveAttribute("aria-valuenow", "43");
  });

  it("renders Level 0 at an empty bar for a fresh profile", () => {
    renderHero({ level: 0, xpIntoLevel: 0, xpForNextLevel: 50 });

    expect(screen.getByTestId("profile-level")).toHaveTextContent("Level 0");
    expect(screen.getByTestId("profile-xp-bar")).toHaveAttribute("aria-valuenow", "0");
  });
});
