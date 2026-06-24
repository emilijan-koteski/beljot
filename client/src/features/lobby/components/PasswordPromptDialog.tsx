import { Eye, EyeOff, Lock } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "@/shared/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/shared/components/ui/dialog";
import { Input } from "@/shared/components/ui/input";

interface PasswordPromptDialogProps {
  open: boolean;
  /** Name of the private room being entered — shown in the prompt description. */
  roomName: string;
  /** True while a join attempt is in flight (disables submit, shows pending copy). */
  pending: boolean;
  /**
   * i18n key of the error to show (e.g. "room.errors.wrongPassword"), or null.
   * When set, the dialog stays open so the player can retry (Story 9.6, AC4).
   */
  errorKey: string | null;
  onSubmit: (password: string) => void;
  onClose: () => void;
}

/**
 * Story 9.6: password gate shown before a private-room join from either entry
 * point (the lobby card click and the join-by-code tile). It only collects and
 * submits the password — the authoritative check happens server-side on
 * POST /rooms/:id/join. A wrong/missing password keeps the dialog open with the
 * error (the parent controls errorKey); never navigates away on failure.
 */
export function PasswordPromptDialog({
  open,
  roomName,
  pending,
  errorKey,
  onSubmit,
  onClose,
}: PasswordPromptDialogProps) {
  const { t } = useTranslation();
  const [password, setPassword] = useState("");
  // Hide the inline error the moment the player edits the password — a stale
  // "wrong password" shouldn't linger while they retype. Re-armed whenever the
  // parent reports a fresh error (errorKey transitions to non-null).
  const [errorDismissed, setErrorDismissed] = useState(false);
  const [showPassword, setShowPassword] = useState(false);

  // Clear any stale password whenever the dialog (re)opens for a room.
  useEffect(() => {
    if (open) {
      setPassword("");
      setErrorDismissed(false);
      setShowPassword(false);
    }
  }, [open]);

  // A fresh error from the parent re-shows the message even after a prior edit.
  useEffect(() => {
    if (errorKey !== null) setErrorDismissed(false);
  }, [errorKey]);

  const showError = errorKey !== null && !errorDismissed;

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (pending || password.length === 0) return;
    onSubmit(password);
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent showCloseButton={false} data-testid="password-prompt-dialog">
        <form onSubmit={handleSubmit} className="grid gap-4">
          <DialogHeader>
            <div className="flex items-center gap-3">
              <div className="bg-brass-soft border-brass/40 text-brass-deep flex size-11 shrink-0 items-center justify-center rounded-xl border">
                <Lock className="size-5" />
              </div>
              <DialogTitle>{t("room.passwordPrompt.title")}</DialogTitle>
            </div>
            <DialogDescription>
              {t("room.passwordPrompt.description", { name: roomName })}
            </DialogDescription>
          </DialogHeader>

          <div className="relative">
            <Input
              type={showPassword ? "text" : "password"}
              autoFocus
              autoComplete="current-password"
              placeholder={t("room.passwordPrompt.placeholder")}
              value={password}
              onChange={(e) => {
                setPassword(e.target.value);
                setErrorDismissed(true);
              }}
              aria-invalid={showError}
              data-testid="password-prompt-input"
              className="h-11 pr-10"
            />
            <button
              type="button"
              tabIndex={-1}
              className="text-ink-mute hover:text-ink absolute top-1/2 right-2.5 -translate-y-1/2 p-1.5"
              onClick={() => setShowPassword(!showPassword)}
              data-testid="password-prompt-toggle"
              aria-label={showPassword ? "Hide password" : "Show password"}
            >
              {showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
            </button>
          </div>

          {showError && (
            <p
              role="alert"
              data-testid="password-prompt-error"
              className="text-destructive text-sm"
            >
              {t(errorKey)}
            </p>
          )}

          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              onClick={onClose}
              data-testid="password-prompt-cancel"
            >
              {t("room.passwordPrompt.cancel")}
            </Button>
            <Button
              type="submit"
              disabled={pending || password.length === 0}
              data-testid="password-prompt-submit"
            >
              {pending ? t("room.passwordPrompt.submitting") : t("room.passwordPrompt.submit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
