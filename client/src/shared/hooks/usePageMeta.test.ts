import { renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { usePageMeta } from "@/shared/hooks/usePageMeta";

describe("usePageMeta", () => {
  let meta: HTMLMetaElement;

  beforeEach(() => {
    document.title = "Home";
    meta = document.createElement("meta");
    meta.name = "description";
    meta.content = "Default description";
    document.head.appendChild(meta);
  });

  afterEach(() => {
    meta.remove();
  });

  it("sets the title with the site name and the description", () => {
    renderHook(() => usePageMeta("Rules", "How to play"));

    expect(document.title).toBe("Rules · Beljot.online");
    expect(meta.content).toBe("How to play");
  });

  it("keeps the default description when none is given", () => {
    renderHook(() => usePageMeta("Log in"));

    expect(meta.content).toBe("Default description");
  });

  it("restores the previous title and description on unmount", () => {
    const { unmount } = renderHook(() => usePageMeta("Rules", "How to play"));
    unmount();

    expect(document.title).toBe("Home");
    expect(meta.content).toBe("Default description");
  });
});
