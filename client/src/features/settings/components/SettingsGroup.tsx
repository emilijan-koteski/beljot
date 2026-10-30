import { cloneElement, isValidElement, type ReactNode, useId } from "react";

import { cn } from "@/shared/lib/utils";

export type SettingsSectionId = "account" | "signIn" | "game" | "sound" | "privacy";

type SettingsGroupProps = {
  id: SettingsSectionId;
  title: string;
  icon: ReactNode;
  testId?: string;
  children: ReactNode;
};

/** A titled group of settings: a small label over one card whose rows are split by hairlines. */
export function SettingsGroup({ id, title, icon, testId, children }: SettingsGroupProps) {
  return (
    <section
      id={`settings-${id}`}
      aria-labelledby={`settings-${id}-title`}
      className="flex scroll-mt-20 flex-col gap-2"
      data-testid={testId}
    >
      <h2
        id={`settings-${id}-title`}
        className="text-ink-mute m-0 flex items-center gap-1.5 px-1 font-mono text-2xs font-semibold tracking-caps uppercase [&>svg]:size-3.5"
      >
        {icon}
        {title}
      </h2>
      <div className="bg-surface border-border divide-border flex flex-col divide-y rounded-xl border shadow-xs">
        {children}
      </div>
    </section>
  );
}

type SettingsRowProps = {
  label: ReactNode;
  hint?: ReactNode;
  control?: ReactNode;
  children?: ReactNode;
  stack?: boolean;
  asLabel?: boolean;
  testId?: string;
};

/**
 * One setting: its name and a short hint on the left, the control on the right, and optional detail underneath.
 * `asLabel` makes the top line a `<label>` that toggles its switch, naming it by the label and describing it by the hint.
 */
export function SettingsRow({
  label,
  hint,
  control,
  children,
  stack = false,
  asLabel = false,
  testId,
}: SettingsRowProps) {
  const labelId = useId();
  const hintId = useId();
  const Top = asLabel ? "label" : "div";
  const named =
    asLabel && isValidElement<Record<string, unknown>>(control)
      ? cloneElement(control, {
          "aria-labelledby": labelId,
          "aria-describedby": hint !== undefined ? hintId : undefined,
        })
      : control;
  return (
    <div className="flex flex-col gap-3 px-4 py-3.5" data-testid={testId}>
      <Top
        className={cn(
          "flex gap-3",
          stack ? "flex-col" : "items-center justify-between",
          asLabel && "cursor-pointer",
        )}
      >
        <span className="flex min-w-0 flex-col gap-0.5">
          <span id={labelId} className="text-ink text-sm font-semibold">
            {label}
          </span>
          {hint !== undefined && (
            <span id={hintId} className="text-ink-mute text-xs">
              {hint}
            </span>
          )}
        </span>
        {named}
      </Top>
      {children}
    </div>
  );
}
