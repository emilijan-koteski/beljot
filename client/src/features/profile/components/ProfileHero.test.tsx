import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ProfileHero } from "./ProfileHero";

type HeroProps = Parameters<typeof ProfileHero>[0];

function renderHero(overrides: Partial<HeroProps> = {}) {
  const props: HeroProps = {
    username: "kiro",
    createdAt: "2026-01-01T00:00:00Z",
    level: 3,
    xpIntoLevel: 150,
    xpForNextLevel: 350,
    layout: "wide",
    ...overrides,
  };
  return render(<ProfileHero {...props} />);
}

describe("ProfileHero", () => {
  it("renders the name, member-since line, level label and XP bar", () => {
    renderHero();
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("kiro");
    expect(screen.getByTestId("profile-member-since")).toHaveTextContent("Member since");
    expect(screen.getByTestId("profile-level")).toHaveTextContent("Level 3");
    expect(screen.getByTestId("profile-xp")).toHaveTextContent("150 / 350 XP");
    expect(screen.getByTestId("profile-xp-bar")).toHaveAttribute("aria-valuenow", "43");
  });

  it("renders Level 0 at an empty bar for a fresh profile", () => {
    renderHero({ level: 0, xpIntoLevel: 0, xpForNextLevel: 50 });
    expect(screen.getByTestId("profile-level")).toHaveTextContent("Level 0");
    expect(screen.getByTestId("profile-xp-bar")).toHaveAttribute("aria-valuenow", "0");
  });

  it("shows the season rank and honor chips", () => {
    renderHero({
      seasonRank: {
        seasonName: "2026 Q4",
        tier: "gold",
        division: 2,
        sp: 680,
        position: 3,
        players: 40,
        gamesPlayed: 12,
        gamesWon: 7,
      },
      honor: { score: 96, tier: "exemplary", isNewPlayer: false },
    });
    expect(screen.getByTestId("profile-hero-rank")).toHaveTextContent("Gold 2");
    expect(screen.getByTestId("profile-hero-honor")).toHaveTextContent("96");
    expect(screen.getByTestId("profile-hero-honor")).toHaveTextContent("Exemplary");
  });

  it("names a newcomer's honor chip instead of showing a score", () => {
    renderHero({ honor: { score: 80, tier: "fair", isNewPlayer: true } });
    expect(screen.getByTestId("profile-hero-honor")).toHaveTextContent("New player");
  });

  it("shows the coins and a streak of two days or more when given them", () => {
    const { container } = renderHero({ walletBalance: 5000, loginStreakDays: 3 });
    expect(container.querySelector('[aria-label^="Coins"]')).not.toBeNull();
    expect(container.querySelector('[aria-label^="Day streak"]')).toHaveTextContent("3");
  });

  it("leaves the coins and streak out when they are not given (someone else's profile)", () => {
    const { container } = renderHero();
    expect(container.querySelector('[aria-label^="Coins"]')).toBeNull();
    expect(container.querySelector('[aria-label^="Day streak"]')).toBeNull();
  });

  it("draws the large avatar eagerly", () => {
    const url = "https://assets.test/avatars/kiro/256.webp";
    renderHero({ avatarUrl: url });
    const img = screen.getByTestId("profile-identity-hero").querySelector("img");
    expect(img).toHaveAttribute("src", url);
    expect(img).toHaveAttribute("loading", "eager");
  });

  it("renders the page's action", () => {
    renderHero({ action: <button type="button">Settings</button>, layout: "phone" });
    expect(screen.getByRole("button", { name: "Settings" })).toBeInTheDocument();
  });
});
