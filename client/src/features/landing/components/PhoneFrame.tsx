import type { ReactNode } from "react";

import { cn } from "@/shared/lib/utils";

type PhoneFrameProps = {
  className?: string;
  children: ReactNode;
};

/** A phone bezel around a real app screenshot. */
export function PhoneFrame({ className, children }: PhoneFrameProps) {
  return (
    <div
      className={cn(
        "rounded-[40px] bg-[linear-gradient(160deg,#2a2f2c,#0b0f0d)] p-2.25 shadow-felt-xl",
        className,
      )}
    >
      <div className="overflow-hidden rounded-[32px]">{children}</div>
    </div>
  );
}
