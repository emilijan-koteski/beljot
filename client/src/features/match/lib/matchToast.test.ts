import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { MOTION } from "@/shared/lib/motion";

import { MATCH_ERROR_TOAST_ID, showMatchError, useFeltToasts } from "./matchToast";

const toastMock = vi.hoisted(() => ({ error: vi.fn(), dismiss: vi.fn() }));
vi.mock("sonner", () => ({ toast: toastMock }));

describe("showMatchError", () => {
  beforeEach(() => {
    toastMock.error.mockClear();
  });

  it("raises a closable error toast under one id for the error duration", () => {
    showMatchError("That card cannot be played");

    expect(toastMock.error).toHaveBeenCalledWith("That card cannot be played", {
      id: MATCH_ERROR_TOAST_ID,
      duration: MOTION.TOAST_ERROR,
      closeButton: true,
    });
  });
});

describe("useFeltToasts", () => {
  it("marks the body while mounted and dismisses the match error on unmount", () => {
    const { unmount } = renderHook(() => useFeltToasts());
    expect(document.body).toHaveClass("felt-toasts");

    unmount();

    expect(document.body).not.toHaveClass("felt-toasts");
    expect(toastMock.dismiss).toHaveBeenCalledWith(MATCH_ERROR_TOAST_ID);
  });
});
