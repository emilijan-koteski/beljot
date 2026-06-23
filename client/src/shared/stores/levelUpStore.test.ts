import { beforeEach, describe, expect, it } from "vitest";

import { useLevelUpStore } from "./levelUpStore";

describe("useLevelUpStore", () => {
  beforeEach(() => useLevelUpStore.getState().clear());

  it("starts with no pending level-up", () => {
    expect(useLevelUpStore.getState().pending).toBeNull();
  });

  it("stores a pending level-up and clears it", () => {
    useLevelUpStore.getState().setPending({ newLevel: 4, newTotalXp: 820, xpEarned: 120 });
    expect(useLevelUpStore.getState().pending).toEqual({
      newLevel: 4,
      newTotalXp: 820,
      xpEarned: 120,
    });

    useLevelUpStore.getState().clear();
    expect(useLevelUpStore.getState().pending).toBeNull();
  });
});
