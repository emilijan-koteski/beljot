import { Switch as SwitchPrimitive } from "@base-ui/react/switch";

import { cn } from "@/shared/lib/utils";

/**
 * On/off switch on the Base UI primitive, with `role="switch"` and a hidden form checkbox.
 * Wrap it in a `<label>` with its text so the text names it and toggles it on click.
 */
function Switch({ className, ...props }: SwitchPrimitive.Root.Props) {
  return (
    <SwitchPrimitive.Root
      data-slot="switch"
      className={cn(
        "peer group/switch relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full border p-0.5 outline-none transition-colors",
        "focus-visible:ring-ring/70 focus-visible:ring-3",
        "data-checked:border-accent data-checked:bg-accent",
        "data-unchecked:border-border data-unchecked:bg-surface-sunken",
        "data-disabled:cursor-not-allowed data-disabled:opacity-50",
        className,
      )}
      {...props}
    >
      <SwitchPrimitive.Thumb
        data-slot="switch-thumb"
        className={cn(
          "pointer-events-none block size-3.5 rounded-full shadow-sm motion-safe:transition-transform",
          "data-checked:bg-accent-ink data-checked:translate-x-4",
          "data-unchecked:bg-ink-mute data-unchecked:translate-x-0",
        )}
      />
    </SwitchPrimitive.Root>
  );
}

export { Switch };
