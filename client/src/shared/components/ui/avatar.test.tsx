import { fireEvent, render, screen } from "@testing-library/react";
import { Bot } from "lucide-react";
import { describe, expect, it } from "vitest";

import { Avatar } from "./avatar";

const URL_A = "https://assets.test/avatars/aaaa/128.webp";
const URL_B = "https://assets.test/avatars/bbbb/128.webp";

// The picture is decorative (alt="", inside an aria-hidden disc), so it is
// queried by tag rather than by role.
function image(container: HTMLElement): HTMLImageElement | null {
  return container.querySelector("img");
}

describe("Avatar", () => {
  it("renders the initial when there is no avatar", () => {
    const { container } = render(<Avatar name="marko" />);
    expect(image(container)).toBeNull();
    expect(container).toHaveTextContent("M");
  });

  it("treats a null avatarUrl as no avatar", () => {
    const { container } = render(<Avatar name="ana" avatarUrl={null} />);
    expect(image(container)).toBeNull();
    expect(container).toHaveTextContent("A");
  });

  it("renders the picture with a reserved box, async decoding and lazy loading", () => {
    const { container } = render(<Avatar name="marko" avatarUrl={URL_A} size={42} />);
    const img = image(container);
    expect(img).not.toBeNull();
    expect(img).toHaveAttribute("src", URL_A);
    expect(img).toHaveAttribute("alt", "");
    expect(img).toHaveAttribute("width", "42");
    expect(img).toHaveAttribute("height", "42");
    expect(img).toHaveAttribute("decoding", "async");
    expect(img).toHaveAttribute("loading", "lazy");
    expect(container).not.toHaveTextContent("M");
  });

  it("loads eagerly only when asked to", () => {
    const { container } = render(<Avatar name="marko" avatarUrl={URL_A} size={96} eager />);
    expect(image(container)).toHaveAttribute("loading", "eager");
  });

  it("falls back to the initial when the picture fails to load", () => {
    const { container } = render(<Avatar name="marko" avatarUrl={URL_A} />);
    fireEvent.error(image(container)!);
    expect(image(container)).toBeNull();
    expect(container).toHaveTextContent("M");
  });

  it("tries a new URL again after an earlier one failed", () => {
    const { container, rerender } = render(<Avatar name="marko" avatarUrl={URL_A} />);
    fireEvent.error(image(container)!);
    expect(image(container)).toBeNull();

    rerender(<Avatar name="marko" avatarUrl={URL_B} />);
    expect(image(container)).toHaveAttribute("src", URL_B);
  });

  it("keeps the icon over any picture", () => {
    const { container } = render(<Avatar name="" avatarUrl={URL_A} icon={<Bot />} />);
    expect(image(container)).toBeNull();
    expect(screen.getByTestId("avatar-icon")).toBeInTheDocument();
  });

  it("keeps the ring and the profile halo around the picture", () => {
    const { container: ringed } = render(<Avatar name="marko" avatarUrl={URL_A} owner />);
    expect((ringed.firstChild as HTMLElement).style.border).toContain("2px solid");

    const { container: halo } = render(<Avatar name="marko" avatarUrl={URL_A} halo="profile" />);
    expect((halo.firstChild as HTMLElement).style.boxShadow).toContain("var(--brass)");
  });
});
