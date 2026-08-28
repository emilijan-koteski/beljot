import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SupportDialog } from "@/shared/components/support/SupportDialog";
import { BMC_MARK_SRC, BMC_STICKER_SRC, BMC_URL } from "@/shared/components/support/supportLinks";

// jsdom has no matchMedia, so the real hook would always report "motion is
// fine" and the reduced-motion branch would never run.
let mockReducedMotion = false;
vi.mock("@/shared/hooks/useReducedMotion", () => ({
  useReducedMotion: () => mockReducedMotion,
}));

function renderDialog(open = true) {
  const onOpenChange = vi.fn();
  render(<SupportDialog open={open} onOpenChange={onOpenChange} />);
  return { onOpenChange };
}

describe("SupportDialog", () => {
  beforeEach(() => {
    mockReducedMotion = false;
  });

  it("renders nothing while closed", () => {
    renderDialog(false);
    expect(screen.queryByTestId("support-dialog")).not.toBeInTheDocument();
  });

  it("explains why support is being asked for", () => {
    renderDialog();
    expect(screen.getByTestId("support-dialog")).toBeInTheDocument();
    expect(screen.getByText("Hobby project")).toBeInTheDocument();
    expect(screen.getByText("I build Beljot.online in my free time")).toBeInTheDocument();
    // The reason has to name a real cost, not gesture at one.
    expect(screen.getByText(/come out of my own pocket/)).toBeInTheDocument();
    // ...and has to say the ask is optional, which is the whole tone of this.
    expect(screen.getByText(/helps just as much/)).toBeInTheDocument();
  });

  it("points the call to action at the canonical page in a safe new tab", () => {
    renderDialog();
    const cta = screen.getByTestId("support-cta");
    expect(cta).toHaveAttribute("href", BMC_URL);
    expect(cta).toHaveAttribute("target", "_blank");
    // Both tokens matter: noopener severs window.opener, noreferrer withholds
    // the referrer. An external link must carry them together.
    expect(cta).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("animates the mark by default, hidden from assistive tech", () => {
    renderDialog();
    const mark = screen.getByTestId("support-mark");
    expect(mark).toHaveAttribute("src", BMC_STICKER_SRC);
    // Decorative: the copy beside it already says everything the cup does.
    expect(mark).toHaveAttribute("alt", "");
    expect(mark).toHaveAttribute("aria-hidden", "true");
  });

  it("freezes the mark to a still frame under prefers-reduced-motion", () => {
    mockReducedMotion = true;
    renderDialog();
    expect(screen.getByTestId("support-mark")).toHaveAttribute("src", BMC_MARK_SRC);
  });

  it("keeps the QR collapsed until asked for", async () => {
    const user = userEvent.setup();
    renderDialog();

    const toggle = screen.getByTestId("support-qr-toggle");
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByTestId("support-qr")).not.toBeInTheDocument();

    await user.click(toggle);

    expect(toggle).toHaveAttribute("aria-expanded", "true");
    // A QR is content, not decoration — it needs a real alt, since a viewer who
    // can't see it still needs to know what the square is.
    expect(screen.getByTestId("support-qr")).toHaveAccessibleName(/QR code/i);
  });

  it("collapses the QR again on a second click", async () => {
    const user = userEvent.setup();
    renderDialog();

    const toggle = screen.getByTestId("support-qr-toggle");
    await user.click(toggle);
    await user.click(toggle);

    expect(screen.queryByTestId("support-qr")).not.toBeInTheDocument();
  });
});
