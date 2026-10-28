import { Search, X } from "lucide-react";
import type { ComponentProps } from "react";

import { cn } from "@/shared/lib/utils";

type SearchInputProps = Omit<ComponentProps<"input">, "value" | "onChange"> & {
  value: string;
  onValueChange: (value: string) => void;
  clearLabel?: string;
  clearTestId?: string;
  wrapperClassName?: string;
};

/**
 * The one search box: a magnifier, a borderless input and, when `clearLabel` is given and there is
 * text, a clear button. The whole box takes the focus ring.
 */
export function SearchInput({
  value,
  onValueChange,
  clearLabel,
  clearTestId,
  wrapperClassName,
  className,
  ...props
}: SearchInputProps) {
  return (
    <div
      data-slot="search-input"
      className={cn(
        "bg-surface-elevated border-border flex items-center gap-2 rounded-md border px-2.5 py-1.5 transition-[border-color,box-shadow] has-[input:focus-visible]:border-ring has-[input:focus-visible]:ring-3 has-[input:focus-visible]:ring-ring/70",
        wrapperClassName,
      )}
    >
      <Search className="text-ink-mute size-3.5 shrink-0" aria-hidden="true" />
      <input
        value={value}
        onChange={(e) => onValueChange(e.target.value)}
        className={cn(
          "text-ink placeholder:text-ink-mute min-w-0 flex-1 truncate bg-transparent text-sm outline-none",
          className,
        )}
        {...props}
      />
      {clearLabel !== undefined && value !== "" && (
        <button
          type="button"
          onClick={() => onValueChange("")}
          aria-label={clearLabel}
          data-testid={clearTestId}
          className="text-ink-mute hover:text-ink flex rounded p-0.5 outline-none focus-visible:ring-3 focus-visible:ring-ring/70"
        >
          <X className="size-3.5" aria-hidden="true" />
        </button>
      )}
    </div>
  );
}
