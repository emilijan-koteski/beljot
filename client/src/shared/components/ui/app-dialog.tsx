import type { FormEvent, PointerEventHandler, ReactNode } from "react";

import { Button, buttonVariants } from "@/shared/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/shared/components/ui/dialog";
import { cn } from "@/shared/lib/utils";

export type AppDialogTone = "default" | "danger" | "celebration";

export interface AppDialogAction {
  label: ReactNode;
  onClick?: () => void;
  href?: string;
  disabled?: boolean;
  testId?: string;
}

interface AppDialogProps {
  open: boolean;
  onClose: () => void;
  tone?: AppDialogTone;
  size?: "md" | "lg";
  icon?: ReactNode;
  art?: ReactNode;
  title: ReactNode;
  description?: ReactNode;
  error?: ReactNode;
  errorTestId?: string;
  pending?: boolean;
  primary: AppDialogAction;
  secondary?: AppDialogAction;
  onSubmit?: () => void;
  onPointerDownCapture?: PointerEventHandler<HTMLDivElement>;
  testId?: string;
  children?: ReactNode;
}

const TILE_CLASS = {
  default: "border-brass/40 bg-brass-soft text-brass-deep",
  danger: "border-danger/30 bg-danger/10 text-danger",
} as const;

/** The 44px icon tile that opens every AppDialog header; exported for the two-pane create-room sheet. */
export function AppDialogTile({
  tone = "default",
  children,
}: {
  tone?: keyof typeof TILE_CLASS;
  children: ReactNode;
}) {
  return (
    <span
      className={cn(
        "flex size-11 shrink-0 items-center justify-center rounded-lg border [&_svg]:size-5",
        TILE_CLASS[tone],
      )}
    >
      {children}
    </span>
  );
}

/**
 * The one dialog shell for the parchment pages: a 480px (or 768px `lg`) card with an icon tile, title, one line of
 * description, one error line and a footer whose primary comes first. Escape and outside clicks close it unless `pending`.
 */
export function AppDialog({
  open,
  onClose,
  tone = "default",
  size = "md",
  icon,
  art,
  title,
  description,
  error,
  errorTestId,
  pending = false,
  primary,
  secondary,
  onSubmit,
  onPointerDownCapture,
  testId,
  children,
}: AppDialogProps) {
  const celebration = tone === "celebration";

  function submit(event: FormEvent) {
    event.preventDefault();
    if (pending || primary.disabled === true) return;
    onSubmit?.();
  }

  const primaryVariant = tone === "danger" ? "danger" : "default";
  const primaryButton =
    primary.href !== undefined ? (
      <a
        href={primary.href}
        target="_blank"
        rel="noopener noreferrer"
        onClick={primary.onClick}
        className={cn(
          buttonVariants({ variant: primaryVariant, size: "cta" }),
          celebration && "w-full",
        )}
        data-testid={primary.testId}
      >
        {primary.label}
      </a>
    ) : (
      <Button
        type={onSubmit ? "submit" : "button"}
        size="cta"
        variant={primaryVariant}
        disabled={pending || primary.disabled === true}
        onClick={onSubmit ? undefined : primary.onClick}
        className={cn(celebration && "w-full")}
        data-testid={primary.testId}
      >
        {primary.label}
      </Button>
    );

  const content = (
    <>
      <div
        className={cn(
          "px-6 pt-6",
          celebration ? "flex flex-col items-center text-center" : "flex items-start gap-4",
        )}
      >
        {celebration
          ? art
          : icon && (
              <AppDialogTile tone={tone === "danger" ? "danger" : "default"}>{icon}</AppDialogTile>
            )}
        <div className={cn("min-w-0", celebration && art !== undefined && "mt-4")}>
          <DialogTitle className="text-ink text-xl leading-tight font-semibold tracking-tight">
            {title}
          </DialogTitle>
          {description !== undefined && (
            <DialogDescription className="text-ink-dim mt-1 text-sm leading-relaxed">
              {description}
            </DialogDescription>
          )}
        </div>
      </div>
      {children !== undefined && (
        <div className={cn("px-6 pt-5", celebration && "flex flex-col items-center")}>
          {children}
        </div>
      )}
      {error !== undefined && error !== null && error !== false && (
        <p role="alert" data-testid={errorTestId} className="text-danger px-6 pt-3 text-sm">
          {error}
        </p>
      )}
      {celebration ? (
        <div className="p-6">{primaryButton}</div>
      ) : (
        <div
          data-slot="app-dialog-footer"
          className="border-border bg-surface-sunken/50 mt-6 flex flex-col gap-2 border-t px-6 py-4 sm:flex-row sm:justify-end"
        >
          {primaryButton}
          {secondary && (
            <Button
              type="button"
              variant="ghost"
              size="cta"
              disabled={pending || secondary.disabled === true}
              onClick={secondary.onClick}
              data-testid={secondary.testId}
            >
              {secondary.label}
            </Button>
          )}
        </div>
      )}
    </>
  );

  return (
    <Dialog open={open} onOpenChange={(next) => !next && !pending && onClose()}>
      <DialogContent
        showCloseButton={false}
        onPointerDownCapture={onPointerDownCapture}
        data-testid={testId}
        data-tone={tone}
        className={cn(
          "bg-surface ring-brass-soft block gap-0 overflow-hidden rounded-xl p-0 shadow-xl",
          size === "lg" ? "sm:max-w-3xl" : "sm:max-w-[480px]",
        )}
      >
        {onSubmit ? <form onSubmit={submit}>{content}</form> : content}
      </DialogContent>
    </Dialog>
  );
}
