import { Check, ChevronDown, type LucideIcon } from "lucide-react";

import { controlVariants } from "@/shared/components/ui/control";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/shared/components/ui/dropdown-menu";

export interface SortOption<T extends string> {
  value: T;
  label: string;
  icon: LucideIcon;
}

interface SortMenuProps<T extends string> {
  value: T;
  options: SortOption<T>[];
  onValueChange: (next: T) => void;
  ariaLabel: string;
  labelled?: boolean;
  testId?: string;
}

/**
 * A sort control that shows the active order's own icon, with its name beside it when `labelled`.
 * It opens a menu of every order with a check on the active one.
 */
export function SortMenu<T extends string>({
  value,
  options,
  onValueChange,
  ariaLabel,
  labelled = false,
  testId = "sort-menu",
}: SortMenuProps<T>) {
  const active = options.find((o) => o.value === value) ?? options[0]!;
  const ActiveIcon = active.icon;
  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger
        aria-label={ariaLabel}
        data-testid={testId}
        data-sort={active.value}
        className={controlVariants({ shape: labelled ? "labelled" : "round" })}
      >
        <ActiveIcon aria-hidden="true" />
        {labelled && (
          <>
            {active.label}
            <ChevronDown className="text-ink-mute size-3" aria-hidden="true" />
          </>
        )}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-max min-w-44">
        {options.map(({ value: optionValue, label, icon: Icon }) => (
          <DropdownMenuItem
            key={optionValue}
            onClick={() => onValueChange(optionValue)}
            data-testid={`${testId}-${optionValue}`}
          >
            <Icon className="text-ink-dim" aria-hidden="true" />
            <span className={optionValue === value ? "font-semibold" : undefined}>{label}</span>
            {optionValue === value && <Check className="text-accent ml-auto" aria-hidden="true" />}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
