import { Check } from "lucide-react";
import * as React from "react";
import { Link } from "react-router";

import { Field } from "@/shared/components/ui/field";
import { SuitRule } from "@/shared/components/ui/suit-rule";
import { cn } from "@/shared/lib/utils";

export { Field };

type AuthCardProps = {
  eyebrow: string;
  title: string;
  subtitle: string;
  children: React.ReactNode;
  footer?: React.ReactNode;
};

/**
 * 440px parchment card with a brass top hairline, eyebrow, title, subtitle,
 * and suit-pip divider before the form content.
 */
export function AuthCard({ eyebrow, title, subtitle, children, footer }: AuthCardProps) {
  return (
    <div
      className="bg-surface relative w-full max-w-125 rounded-[22px] border border-border px-10 pt-10 pb-8 shadow-[0_18px_44px_-28px_rgba(14,58,36,0.32),0_2px_0_rgba(255,255,255,0.6)_inset]"
      data-testid="auth-card"
    >
      <div
        aria-hidden="true"
        className="absolute -top-px right-7 left-7 h-0.5 bg-[linear-gradient(90deg,transparent,var(--brass)_50%,transparent)] opacity-70"
      />

      <div className="text-brass-deep mb-3 font-mono text-[11.5px] font-semibold tracking-[2.4px] uppercase">
        {eyebrow}
      </div>

      <h1 className="font-display text-ink m-0 text-[34px] leading-[1.1] font-bold tracking-[-0.8px]">
        {title}
      </h1>

      <p className="text-ink-dim mt-2.5 mb-5.5 max-w-95 text-[14.5px] leading-[1.55]">{subtitle}</p>

      <SuitRule />

      {children}

      {footer && <div className="mt-5.5">{footer}</div>}
    </div>
  );
}

type AltLinkProps = {
  prompt: string;
  cta: string;
  to: string;
  testId?: string;
};

/** Bottom prompt and accent link to the alternate auth page. */
export function AltLink({ prompt, cta, to, testId }: AltLinkProps) {
  return (
    <div className="text-ink-dim text-center text-[13.5px]">
      {prompt}{" "}
      <Link
        to={to}
        className="text-accent border-accent/30 inline-block border-b pb-px font-semibold hover:underline"
        data-testid={testId}
      >
        {cta}
      </Link>
    </div>
  );
}

type CheckboxProps = {
  checked: boolean;
  onChange: (next: boolean) => void;
  invalid?: boolean;
  testId?: string;
  children: React.ReactNode;
};

/**
 * Parchment-styled checkbox whose label wraps the whole row, so clicking the text also toggles.
 * The real input sits transparently over the visual box.
 */
export function Checkbox({ checked, onChange, invalid, testId, children }: CheckboxProps) {
  return (
    <label
      className={cn(
        "text-ink-dim flex cursor-pointer items-start gap-2.5 text-[13px] leading-normal",
      )}
    >
      <span className="relative mt-px inline-block size-4.5 shrink-0">
        <input
          type="checkbox"
          checked={checked}
          onChange={(e) => onChange(e.target.checked)}
          aria-invalid={invalid || undefined}
          data-testid={testId}
          className="peer absolute inset-0 size-full cursor-pointer opacity-0"
        />
        <span
          aria-hidden="true"
          className={cn(
            "pointer-events-none absolute inset-0 inline-flex items-center justify-center rounded-[5px] border transition-[background-color,border-color,box-shadow] peer-focus-visible:ring-3 peer-focus-visible:ring-ring/70",
            checked
              ? "bg-accent border-accent text-accent-ink"
              : invalid
                ? "bg-surface-elevated border-danger"
                : "bg-surface-elevated border-brass",
          )}
        >
          {checked && <Check className="size-3" strokeWidth={3} />}
        </span>
      </span>
      <span>{children}</span>
    </label>
  );
}
