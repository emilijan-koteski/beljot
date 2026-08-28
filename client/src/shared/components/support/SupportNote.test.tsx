import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { BMC_MARK_SRC } from "@/shared/components/support/supportLinks";
import { SupportNote } from "@/shared/components/support/SupportNote";

describe("SupportNote", () => {
  // Both placements render the same `support.note`; `variant` only changes the
  // type scale. Asserted per variant so a future re-split shows up here.
  it.each(["auth", "lobby"] as const)("renders the same wording in the %s note", (variant) => {
    render(<SupportNote variant={variant} />);
    expect(screen.getByTestId("support-note").textContent).toContain(
      "A hobby project — you can always buy me a coffee.",
    );
    expect(screen.getByTestId("support-note-link")).toHaveTextContent("buy me a coffee");
  });

  it("closes the sentence with a decorative cup, never the animation", () => {
    render(<SupportNote variant="auth" />);

    const icon = screen.getByTestId("support-note-icon");
    // The STILL frame: a looping GIF in a footer line is the nagging this
    // feature is built to avoid.
    expect(icon).toHaveAttribute("src", BMC_MARK_SRC);
    expect(icon).toHaveAttribute("alt", "");
    expect(icon).toHaveAttribute("aria-hidden", "true");
  });

  it("stays quiet until the link is used", () => {
    render(<SupportNote variant="lobby" />);
    expect(screen.queryByTestId("support-dialog")).not.toBeInTheDocument();
  });

  it("opens the explainer dialog from the link", async () => {
    const user = userEvent.setup();
    render(<SupportNote variant="auth" />);

    await user.click(screen.getByTestId("support-note-link"));

    expect(await screen.findByTestId("support-dialog")).toBeInTheDocument();
    expect(screen.getByTestId("support-cta")).toBeInTheDocument();
  });
});
