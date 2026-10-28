import { InfoPopover } from "@/shared/components/ui/info-popover";
import { Z } from "@/shared/lib/zLayers";

type Props = {
  note: string;
  label: string;
  tone?: "light" | "dark";
};

/** Inline "!" marker whose note says what the other variant does; it opens on hover or tap. */
export function DiffMarker({ note, label, tone = "light" }: Props) {
  return (
    <InfoPopover
      variant="diff"
      tone={tone}
      text={note}
      ariaLabel={label}
      zIndex={tone === "dark" ? Z.UTIL_POPOVER : undefined}
      testId="rules-diff-marker"
      className="ml-1.5"
    />
  );
}
