import type { ReactNode } from "react";

import { Eyebrow } from "@/shared/components/ui/eyebrow";

type SectionHeaderProps = {
  eyebrow: string;
  title: string;
  sub?: string;
  right?: ReactNode;
};

/**
 * Mono eyebrow + display title + optional subtitle, used above the Stats and
 * Match History sections. Mirrors the lobby's section-header rhythm so the
 * profile reads as part of the same surface.
 */
export function SectionHeader({ eyebrow, title, sub, right }: SectionHeaderProps) {
  return (
    <div className="mb-3.5 flex items-end gap-4">
      <div className="min-w-0">
        <Eyebrow className="mb-1">{eyebrow}</Eyebrow>
        <h2 className="text-ink m-0 text-3xl font-semibold">{title}</h2>
        {sub && <p className="text-ink-dim m-0 mt-1 max-w-160 text-sm">{sub}</p>}
      </div>
      {right && <div className="ml-auto">{right}</div>}
    </div>
  );
}
