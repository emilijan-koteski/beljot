import type { ReactNode } from "react";

import { Eyebrow } from "@/shared/components/ui/eyebrow";
import { cn } from "@/shared/lib/utils";

type SectionHeadingProps = {
  eyebrow: string;
  title: ReactNode;
  body?: string;
  className?: string;
};

/** A landing section's eyebrow, display-face title and one-line lead. */
export function SectionHeading({ eyebrow, title, body, className }: SectionHeadingProps) {
  return (
    <div className={className}>
      <Eyebrow size="xl">{eyebrow}</Eyebrow>
      <h2 className="font-display text-ink mt-3.5 text-[38px] leading-[1.02] font-semibold tracking-[0.03em] uppercase md:text-[50px]">
        {title}
      </h2>
      {body && (
        <p className={cn("text-ink-dim mt-4 max-w-130 text-base leading-relaxed md:text-lg")}>
          {body}
        </p>
      )}
    </div>
  );
}
