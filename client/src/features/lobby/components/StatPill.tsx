import type { ReactNode } from "react";

import { Badge } from "@/shared/components/ui/badge";

type Props = {
  icon: ReactNode;
  label: string;
  value: string | number;
  tone?: "neutral" | "accent";
  testId?: string;
};

/** A lobby headline figure as an informative tag: icon, label and a bold value. */
export function StatPill({ icon, label, value, tone = "neutral", testId }: Props) {
  return (
    <Badge tone={tone} icon={icon} data-testid={testId}>
      <span className="font-medium">{label}</span>
      <span className="tabular-nums font-bold">{value}</span>
    </Badge>
  );
}
