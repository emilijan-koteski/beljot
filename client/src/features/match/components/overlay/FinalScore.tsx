import type { ReactNode } from "react";

import type { TeamString } from "@/shared/types/matchTypes";

interface ScoreSide {
  label: ReactNode;
  score: number;
  color: string;
  team?: TeamString;
  testId?: string;
  scoreTestId?: string;
}

interface FinalScoreProps {
  sides: readonly [ScoreSide, ScoreSide];
  testId?: string;
  children?: ReactNode;
}

/** One team's label over its final score, both in the team's colour. */
function ScoreColumn({ side }: { side: ScoreSide }) {
  return (
    <div className="text-center" data-testid={side.testId} data-team={side.team}>
      <p className="text-xs font-semibold tracking-wider uppercase" style={{ color: side.color }}>
        {side.label}
      </p>
      <p
        className="text-5xl font-bold tracking-tight tabular-nums"
        style={{ color: side.color }}
        data-testid={side.scoreTestId}
      >
        {side.score}
      </p>
    </div>
  );
}

/** Two final scores side by side, the viewer's team first, split by a faint dot. */
export function FinalScore({ sides, testId, children }: FinalScoreProps) {
  return (
    <div className="my-2 flex items-center justify-center gap-6" data-testid={testId}>
      <ScoreColumn side={sides[0]} />
      <span
        aria-hidden
        className="text-3xl tracking-tight"
        style={{ color: "var(--ink-light)", opacity: 0.4 }}
      >
        ·
      </span>
      <ScoreColumn side={sides[1]} />
      {children}
    </div>
  );
}
