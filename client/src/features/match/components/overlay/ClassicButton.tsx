import { type ButtonHTMLAttributes, forwardRef } from "react";

import { MOTION } from "@/shared/lib/motion";

export type ClassicButtonVariant = "primary" | "ghost";

interface ClassicButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ClassicButtonVariant;
}

/**
 * Action button used inside ClassicPanel overlays.
 *
 *  • `primary` — brass gradient + dark engraved label (Take, Continue,
 *    Announce Belot…).
 *  • `ghost`   — translucent felt with brass border (Pass, Decline, Skip…).
 */
export const ClassicButton = forwardRef<HTMLButtonElement, ClassicButtonProps>(
  function ClassicButton(
    { variant = "ghost", type = "button", style, className = "", children, disabled, ...rest },
    ref,
  ) {
    const isPrimary = variant === "primary";

    const baseStyle: React.CSSProperties = {
      border: isPrimary ? "1px solid rgba(201,168,118,0.85)" : "1px solid rgba(201,168,118,0.4)",
      background: isPrimary
        ? "linear-gradient(180deg, var(--brass, #c9a876) 0%, var(--brass-deep, #9c7d4e) 100%)"
        : "linear-gradient(180deg, rgba(60,90,70,0.6), rgba(30,50,35,0.6))",
      color: isPrimary ? "var(--brass-ink, #2a1a08)" : "var(--ink-light, #f5f2e8)",
      fontFamily: "var(--font-body)",
      fontWeight: 600,
      letterSpacing: 0.4,
      borderRadius: 8,
      cursor: disabled ? "not-allowed" : "pointer",
      opacity: disabled ? 0.4 : 1,
      boxShadow: isPrimary
        ? "0 4px 10px rgba(201,168,118,0.4), inset 0 1px 0 rgba(255,255,255,0.3)"
        : "0 2px 6px rgba(0,0,0,0.3)",
      transition: `transform ${MOTION.BUTTON_HOVER}ms, filter ${MOTION.BUTTON_HOVER}ms`,
      ...style,
    };

    return (
      <button
        ref={ref}
        type={type}
        disabled={disabled}
        className={`px-3.5 py-2 text-[13px] sm:px-4.5 sm:py-2.5 sm:text-[14px] hover:not-disabled:brightness-110 ${className}`}
        style={baseStyle}
        {...rest}
      >
        {children}
      </button>
    );
  },
);
