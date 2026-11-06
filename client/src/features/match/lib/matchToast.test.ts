import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { MOTION } from "@/shared/lib/motion";

import {
  clearMissedTurnWarningOnMove,
  MATCH_ERROR_TOAST_ID,
  MISSED_TURN_WARNING_TOAST_ID,
  showMatchError,
  showMissedTurnWarning,
  useFeltToasts,
} from "./matchToast";

const toastMock = vi.hoisted(() => ({ error: vi.fn(), warning: vi.fn(), dismiss: vi.fn() }));
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

describe("missed-turn warning", () => {
  beforeEach(() => {
    toastMock.warning.mockClear();
    toastMock.dismiss.mockClear();
  });

  it("stays up under one id until it is cleared", () => {
    showMissedTurnWarning("Miss your next turn and you abandon the match");

    expect(toastMock.warning).toHaveBeenCalledWith(
      "Miss your next turn and you abandon the match",
      { id: MISSED_TURN_WARNING_TOAST_ID, duration: Infinity, closeButton: true },
    );
  });

  it.each(["action:play_card", "action:pass_trump", "action:pick_trump", "action:decline_belot"])(
    "clears when the player sends %s",
    (actionType) => {
      clearMissedTurnWarningOnMove(actionType);
      expect(toastMock.dismiss).toHaveBeenCalledWith(MISSED_TURN_WARNING_TOAST_ID);
    },
  );

  it.each(["action:pause", "action:continue", "action:emote", "action:surrender_request"])(
    "stays when the player sends %s, which is not a move",
    (actionType) => {
      clearMissedTurnWarningOnMove(actionType);
      expect(toastMock.dismiss).not.toHaveBeenCalled();
    },
  );
});

describe("useFeltToasts", () => {
  it("marks the body while mounted and dismisses the match toasts on unmount", () => {
    const { unmount } = renderHook(() => useFeltToasts());
    expect(document.body).toHaveClass("felt-toasts");

    unmount();

    expect(document.body).not.toHaveClass("felt-toasts");
    expect(toastMock.dismiss).toHaveBeenCalledWith(MATCH_ERROR_TOAST_ID);
    expect(toastMock.dismiss).toHaveBeenCalledWith(MISSED_TURN_WARNING_TOAST_ID);
  });
});
