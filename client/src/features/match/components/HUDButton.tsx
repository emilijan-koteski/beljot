import { type ButtonHTMLAttributes, forwardRef, type ReactNode } from "react";

import { floatingPanelStyle } from "../lib/floatingPanel";

type HUDButtonVariant = "default" | "danger";

interface HUDButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  icon?: ReactNode;
  label?: ReactNode;
  variant?: HUDButtonVariant;
}

export const HUDButton = forwardRef<HTMLButtonElement, HUDButtonProps>(function HUDButton(
  { icon, label, variant = "default", className = "", style, type = "button", ...rest },
  ref,
) {
  const isLabeled = label !== undefined && label !== null;

  const baseStyle: React.CSSProperties = {
    ...floatingPanelStyle({ edge: variant === "danger" ? "danger" : "hairline" }),
    color: variant === "danger" ? "var(--danger-light)" : "var(--ink-light)",
    minWidth: 36,
    height: 36,
    borderRadius: 10,
    padding: isLabeled ? "0 14px" : 0,
    gap: 8,
    fontSize: 12,
    fontWeight: 600,
    ...style,
  };

  return (
    <button
      ref={ref}
      type={type}
      className={`inline-flex items-center justify-center transition-colors disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer hover:not-disabled:brightness-110 ${className}`}
      style={baseStyle}
      {...rest}
    >
      {icon}
      {isLabeled ? <span>{label}</span> : null}
    </button>
  );
});
