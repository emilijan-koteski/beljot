import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { MatchHudMenu } from "./MatchHudMenu";

function renderMenu(overrides: Partial<Parameters<typeof MatchHudMenu>[0]> = {}) {
  const props = {
    showTurnActions: true,
    showPause: true,
    pauseUsed: false,
    canPause: true,
    canSurrender: true,
    audioOn: true,
    showEmotes: false,
    onPause: vi.fn(),
    onSurrender: vi.fn(),
    onRules: vi.fn(),
    onSettings: vi.fn(),
    onToggleMute: vi.fn(),
    onSendEmote: vi.fn(),
    ...overrides,
  };
  render(<MatchHudMenu {...props} />);
  return props;
}

describe("MatchHudMenu", () => {
  it("opens from its toggle and reports the expanded state", async () => {
    const user = userEvent.setup();
    renderMenu();

    const toggle = screen.getByTestId("hud-menu-button");
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    await user.click(toggle);

    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByTestId("hud-menu")).toBeInTheDocument();
  });

  it("runs an action and closes", async () => {
    const user = userEvent.setup();
    const props = renderMenu();

    await user.click(screen.getByTestId("hud-menu-button"));
    await user.click(screen.getByTestId("hud-menu-rules"));

    expect(props.onRules).toHaveBeenCalledTimes(1);
    expect(screen.queryByTestId("hud-menu")).not.toBeInTheDocument();
  });

  it("closes on Escape and returns focus to the toggle", async () => {
    const user = userEvent.setup();
    renderMenu();

    await user.click(screen.getByTestId("hud-menu-button"));
    await user.keyboard("{Escape}");

    expect(screen.queryByTestId("hud-menu")).not.toBeInTheDocument();
    expect(screen.getByTestId("hud-menu-button")).toHaveFocus();
  });

  it("leaves out the turn actions outside a turn phase", async () => {
    const user = userEvent.setup();
    renderMenu({ showTurnActions: false });

    await user.click(screen.getByTestId("hud-menu-button"));

    expect(screen.queryByTestId("hud-menu-pause")).not.toBeInTheDocument();
    expect(screen.queryByTestId("hud-menu-surrender")).not.toBeInTheDocument();
    expect(screen.getByTestId("hud-menu-settings")).toBeInTheDocument();
  });

  it("labels the mute row by what it will do", async () => {
    const user = userEvent.setup();
    renderMenu({ audioOn: false });

    await user.click(screen.getByTestId("hud-menu-button"));

    expect(screen.getByTestId("hud-menu-mute")).toHaveTextContent("Unmute");
  });
});
