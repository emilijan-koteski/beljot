import type { ReactNode } from "react";

import type { Card } from "@/shared/types/matchTypes";

import { CARD_SIZES } from "../../lib/cardFace";
import { PlayingCard } from "../PlayingCard";

const CARD_WIDTH = CARD_SIZES.sm.width;
const FAN_STEP = Math.round(CARD_WIDTH * 0.6);
const FAN_MAX_CARDS_AT_FULL_STEP = 5;
const FAN_MAX_WIDTH = CARD_WIDTH + FAN_STEP * (FAN_MAX_CARDS_AT_FULL_STEP - 1);

interface MeldRowProps {
  label: ReactNode;
  cards: readonly Card[];
  value: number;
  byline?: ReactNode;
  testId?: string;
}

/** Horizontal offset between fanned cards, tightened for long runs so the fan never outgrows five cards. */
function fanStep(count: number): number {
  if (count <= FAN_MAX_CARDS_AT_FULL_STEP) return FAN_STEP;
  return (FAN_MAX_WIDTH - CARD_WIDTH) / (count - 1);
}

/** One declared meld: its cards fanned, its name with an optional byline, and the points it adds. */
export function MeldRow({ label, cards, value, byline, testId }: MeldRowProps) {
  const overlap = CARD_WIDTH - fanStep(cards.length);
  return (
    <div
      className="flex flex-wrap items-center gap-x-4 gap-y-2 rounded-lg px-3.5 py-3"
      style={{ background: "var(--shade-20)", border: "1px solid var(--brass-25)" }}
      data-testid={testId}
    >
      <div className="flex shrink-0">
        {cards.map((card, i) => (
          <div
            key={`${card.rank}${card.suit}`}
            style={{ marginLeft: i === 0 ? 0 : -overlap, zIndex: i }}
          >
            <PlayingCard card={card} state="default" size="sm" withTransition={false} />
          </div>
        ))}
      </div>
      <div className="flex min-w-0 flex-1 basis-36 items-center gap-4">
        <div className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span
            className="truncate text-base font-semibold"
            style={{ color: "var(--ink-light)" }}
            data-testid={testId && `${testId}-label`}
          >
            {label}
          </span>
          {byline !== undefined && (
            <span className="text-xs" style={{ color: "var(--ink-light)", opacity: 0.7 }}>
              {byline}
            </span>
          )}
        </div>
        <span
          className="shrink-0 text-2xl font-bold tabular-nums"
          style={{ color: "var(--ink-light)" }}
          data-testid={testId && `${testId}-value`}
        >
          +{value}
        </span>
      </div>
    </div>
  );
}
