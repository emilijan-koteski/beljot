import { type ButtonHTMLAttributes, type CSSProperties, forwardRef } from "react";

import { MOTION } from "@/shared/lib/motion";

import { BUTTON_RADIUS_PX, DISABLED_OPACITY } from "../../lib/buttonShape";

type ClassicButtonVariant = "primary" | "ghost" | "danger";

interface ClassicButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ClassicButtonVariant;
}

const VARIANT_STYLE: Record<ClassicButtonVariant, CSSProperties> = {
  primary: {
    border: "1px solid var(--brass-85)",
    background: "linear-gradient(180deg, var(--brass) 0%, var(--brass-deep) 100%)",
    color: "var(--brass-ink)",
    boxShadow:
      "0 8px 24px -12px color-mix(in srgb, var(--brass) 70%, transparent), inset 0 1px 0 rgba(255,255,255,0.25)",
  },
  ghost: {
    border: "1px solid var(--brass-40)",
    background: "linear-gradient(180deg, rgba(60,90,70,0.6), rgba(30,50,35,0.6))",
    color: "var(--ink-light)",
    boxShadow: "var(--shadow-felt-sm)",
  },
  danger: {
    border: "1px solid color-mix(in srgb, var(--danger-light) 70%, transparent)",
    background: "linear-gradient(180deg, var(--danger-light) 0%, var(--danger) 100%)",
    color: "var(--ink-light)",
    boxShadow:
      "0 8px 24px -12px color-mix(in srgb, var(--danger-light) 70%, transparent), inset 0 1px 0 rgba(255,255,255,0.25)",
  },
};

/**
 * Action button used inside ClassicPanel overlays: a brass `primary`, a translucent felt `ghost`,
 * or a red `danger` for the one action that cannot be taken back.
 */
export const ClassicButton = forwardRef<HTMLButtonElement, ClassicButtonProps>(
  function ClassicButton(
    { variant = "ghost", type = "button", style, className = "", children, disabled, ...rest },
    ref,
  ) {
    const baseStyle: CSSProperties = {
      ...VARIANT_STYLE[variant],
      fontWeight: 600,
      borderRadius: BUTTON_RADIUS_PX,
      cursor: disabled ? "not-allowed" : "pointer",
      opacity: disabled ? DISABLED_OPACITY : 1,
      transition: `transform ${MOTION.BUTTON_HOVER}ms, filter ${MOTION.BUTTON_HOVER}ms`,
      ...style,
    };

    return (
      <button
        ref={ref}
        type={type}
        disabled={disabled}
        className={`px-3.5 py-2 text-sm sm:px-4.5 sm:py-2.5 sm:text-sm hover:not-disabled:brightness-110 ${className}`}
        style={baseStyle}
        {...rest}
      >
        {children}
      </button>
    );
  },
);
