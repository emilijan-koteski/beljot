"use client";

import {
  CircleCheckIcon,
  InfoIcon,
  Loader2Icon,
  OctagonXIcon,
  TriangleAlertIcon,
} from "lucide-react";
import { Toaster as Sonner, type ToasterProps } from "sonner";

import { Z } from "@/shared/lib/zLayers";

const Toaster = ({ ...props }: ToasterProps) => {
  return (
    <Sonner
      theme="dark"
      className="toaster group"
      icons={{
        success: <CircleCheckIcon className="size-4" />,
        info: <InfoIcon className="size-4" />,
        warning: <TriangleAlertIcon className="size-4" />,
        error: <OctagonXIcon className="size-4" />,
        loading: <Loader2Icon className="size-4 animate-spin" />,
      }}
      style={
        {
          // This project's design tokens are `--color-*` (Tailwind v4 @theme),
          // not the shadcn-default `--popover`/`--border`. The old refs resolved
          // to nothing, so the toast background was transparent and page text
          // bled through. Point them at the real (opaque) tokens.
          "--normal-bg": "var(--color-popover)",
          "--normal-text": "var(--color-popover-foreground)",
          "--normal-border": "var(--color-border)",
          "--border-radius": "var(--radius)",
          // Override sonner's hard-coded 999999999 with the project's declared
          // top layer (Z.TOAST) so stacking stays traceable to zLayers.ts.
          zIndex: Z.TOAST,
        } as React.CSSProperties
      }
      toastOptions={{
        classNames: {
          // Solid, elevated card so toasts read clearly above any page content.
          toast: "bg-popover text-popover-foreground border border-border shadow-xl",
        },
      }}
      {...props}
    />
  );
};

export { Toaster };
