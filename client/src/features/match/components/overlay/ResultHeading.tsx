import type { HTMLAttributes, ReactNode } from "react";

interface ResultHeadingProps extends HTMLAttributes<HTMLHeadingElement> {
  eyebrow: ReactNode;
  color: string;
  eyebrowTestId?: string;
}

/** Top of a result card: a muted small-caps eyebrow over a headline in the result's colour. */
export function ResultHeading({
  eyebrow,
  color,
  eyebrowTestId,
  style,
  children,
  ...rest
}: ResultHeadingProps) {
  return (
    <>
      <span
        className="text-2xs tracking-caps uppercase"
        style={{ color: "var(--ink-light)", opacity: 0.55 }}
        data-testid={eyebrowTestId}
      >
        {eyebrow}
      </span>
      <h2 className="text-3xl font-semibold tracking-tight" style={{ color, ...style }} {...rest}>
        {children}
      </h2>
    </>
  );
}
