import { Lock } from "lucide-react";
import type { CSSProperties } from "react";

import { Avatar } from "@/shared/components/ui/avatar";
import { buttonVariants } from "@/shared/components/ui/button";

type Seat = [initial: string | null, name: string, team: "a" | "b"];

type RoomCardProps = {
  title: string;
  code?: string;
  meta: string;
  ago: string;
  seats?: Seat[];
  host?: string;
  filling?: boolean;
  joinLabel: string;
  emptyLabel: string;
  width?: number;
  style?: CSSProperties;
};

const DEFAULT_SEATS: Seat[] = [
  ["K", "kiro", "a"],
  ["I", "irena", "a"],
  ["E", "emilijan", "b"],
  [null, "empty", "b"],
];

function SeatChip({ seat, emptyLabel }: { seat?: Seat; emptyLabel: string }) {
  const [ini, name, team] = seat ?? [null, "", "b"];
  const isA = team === "a";

  if (!ini) {
    return (
      <span
        className="text-ink-off flex h-7.5 items-center justify-center rounded-lg border border-dashed px-2.5 text-[11px]"
        style={{
          borderColor: isA ? "var(--team-a-edge-soft)" : "var(--team-b-edge-soft)",
          background: isA ? "var(--team-a-line)" : "var(--team-b-line)",
        }}
      >
        {emptyLabel}
      </span>
    );
  }

  return (
    <span
      className="text-ink flex h-7.5 items-center gap-1.5 rounded-lg border py-0 pr-2.5 pl-1 text-xs"
      style={{
        borderColor: isA ? "var(--team-a-edge)" : "var(--team-b-edge)",
        background: isA ? "var(--team-a-tint)" : "var(--team-b-tint)",
      }}
    >
      <span
        className="inline-flex size-4.5 items-center justify-center rounded-full text-[10px] font-bold"
        style={{
          color: "var(--brass-ink)",
          background: isA ? "var(--team-a-fill)" : "var(--team-b-fill)",
        }}
      >
        {ini}
      </span>
      {name}
    </span>
  );
}

// Static marketing room card that mirrors the real lobby RoomCard layout.
export function RoomCard({
  title,
  code = "B4LJ7T",
  meta,
  ago,
  seats = DEFAULT_SEATS,
  host = "kiro",
  filling = false,
  joinLabel,
  emptyLabel,
  width = 340,
  style,
}: RoomCardProps) {
  return (
    <div
      className="bg-surface border-border w-full overflow-hidden rounded-2xl border"
      style={{ maxWidth: width, ...style }}
    >
      <div className="px-4 pt-4 pb-3">
        <div className="font-display flex items-center gap-2 text-base font-semibold">
          <span
            className="size-2 shrink-0 rounded-full"
            style={{
              background: filling ? "var(--brass-deep)" : "var(--accent)",
              animation: "pulse-dot 1.8s ease-in-out infinite",
            }}
          />
          <span className="truncate">{title}</span>
          <span className="bg-surface-sunken border-border text-ink-dim font-mono ml-auto inline-flex items-center gap-1.5 rounded-md border px-2 py-1 text-[11px] font-semibold tracking-[1.2px]">
            <Lock className="size-2.75" />
            {code}
          </span>
        </div>
        <div className="text-ink-dim mt-1.5 flex items-center gap-1.5 text-xs">
          <span>{meta}</span>
          <span className="text-ink-off">·</span>
          <span>{ago}</span>
        </div>
      </div>

      <div className="grid grid-cols-[auto_1fr_1fr] items-center gap-1.5 px-4 pb-3">
        <span className="text-[10px] font-bold tracking-[1.2px]" style={{ color: "var(--team-a)" }}>
          A
        </span>
        <SeatChip seat={seats[0]} emptyLabel={emptyLabel} />
        <SeatChip seat={seats[1]} emptyLabel={emptyLabel} />
        <span className="text-[10px] font-bold tracking-[1.2px]" style={{ color: "var(--team-b)" }}>
          B
        </span>
        <SeatChip seat={seats[2]} emptyLabel={emptyLabel} />
        <SeatChip seat={seats[3]} emptyLabel={emptyLabel} />
      </div>

      <div
        className="border-border text-ink-dim flex items-center gap-2.5 border-t px-4 py-2.5 text-xs"
        style={{ background: "rgba(14,58,36,0.03)" }}
      >
        <Avatar name={host} size={18} />
        <span>{host}</span>
        <span className="text-ink-mute">3/4</span>
        <button
          type="button"
          tabIndex={-1}
          aria-hidden="true"
          className={buttonVariants({ size: "sm", className: "ml-auto" })}
        >
          {joinLabel} →
        </button>
      </div>
    </div>
  );
}
