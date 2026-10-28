import { describe, expect, it } from "vitest";

import { buttonVariants } from "./button";

describe("buttonVariants", () => {
  it("lets an outline variant's border replace the base transparent border", () => {
    const classes = buttonVariants({ variant: "outline", size: "cta" }).split(" ");
    expect(classes).toContain("border-border");
    expect(classes).not.toContain("border-transparent");
  });

  it("keeps the base transparent border when the variant sets none", () => {
    expect(buttonVariants().split(" ")).toContain("border-transparent");
  });
});
