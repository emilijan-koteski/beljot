import { QueryClientProvider } from "@tanstack/react-query";
import { act, render } from "@testing-library/react";
import type { ComponentProps } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useLevelUpStore } from "@/shared/stores/levelUpStore";
import { useRankChangeStore } from "@/shared/stores/rankChangeStore";
import { createTestQueryClient } from "@/test-utils";

import type { RankChangeDialog } from "./RankChangeDialog";
import { RankChangeGate } from "./RankChangeGate";

// The popup's EXIT animation cannot be observed in jsdom (it unmounts at once),
// so this file checks the gate's side of the contract directly: the props it
// hands the dialog on the render where `open` turns false.
type DialogProps = ComponentProps<typeof RankChangeDialog>;
const renders: DialogProps[] = [];
vi.mock("./RankChangeDialog", () => ({
  RankChangeDialog: (props: DialogProps) => {
    renders.push(props);
    return null;
  },
}));

vi.mock("@/shared/api/season", () => ({
  getCurrentSeason: () => new Promise(() => {}),
}));

describe("RankChangeGate while the dialog closes", () => {
  beforeEach(() => {
    renders.length = 0;
    useRankChangeStore.getState().clear();
    useLevelUpStore.getState().clear();
  });

  it("keeps handing the dialog the demotion it was showing, never the promoted fallback", () => {
    useRankChangeStore.getState().setPending({
      direction: "demoted",
      rankTier: "silver",
      rankDivision: 3,
      spChange: -20,
      newSeasonSp: 590,
    });
    render(
      <QueryClientProvider client={createTestQueryClient()}>
        <RankChangeGate />
      </QueryClientProvider>,
    );

    act(() => useRankChangeStore.getState().clear());

    const closing = renders.at(-1);
    if (!closing) throw new Error("the gate never rendered the dialog");
    expect(closing.open).toBe(false);
    expect(closing.direction).toBe("demoted");
    expect(closing.tier).toBe("silver");
    expect(closing.division).toBe(3);
    expect(closing.spChange).toBe(-20);
  });
});
