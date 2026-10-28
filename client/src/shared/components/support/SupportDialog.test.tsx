import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SupportDialog } from "@/shared/components/support/SupportDialog";
import { BMC_MARK_SRC, BMC_STICKER_SRC, BMC_URL } from "@/shared/components/support/supportLinks";

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
    expect(
      screen.getByRole("dialog", { name: "I build Beljot.online in my free time" }),
    ).toHaveAccessibleDescription(
      "The server and domain come out of my pocket. A coffee helps, and so does playing and bringing friends.",
    );
    expect(screen.queryByText("Hobby project")).toBeNull();
  });

  it("points the call to action at the canonical page in a safe new tab", () => {
    renderDialog();
    const cta = screen.getByTestId("support-cta");
    expect(cta).toHaveAttribute("href", BMC_URL);
    expect(cta).toHaveAttribute("target", "_blank");
    expect(cta).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("animates the mark by default, hidden from assistive tech", () => {
    renderDialog();
    const mark = screen.getByTestId("support-mark");
    expect(mark).toHaveAttribute("src", BMC_STICKER_SRC);
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
