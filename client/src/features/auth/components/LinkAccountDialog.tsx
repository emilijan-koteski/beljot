import { Eye, EyeOff, Link2 } from "lucide-react";
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

interface LinkAccountDialogProps {
  open: boolean;
  /** Email of the existing password account being linked — shown in the lede. */
  email: string;
  /** True while the link request is in flight (disables submit, shows pending copy). */
  pending: boolean;
  /**
   * i18n key of the error to show (e.g. "auth.sso.linkDialog.errors.wrongPassword"),
   * or null. When set, the dialog stays open so the player can retry.
   */
  errorKey: string | null;
  onSubmit: (password: string) => void;
  onClose: () => void;
}

/**
 * Password confirmation shown when an SSO sign-in matches an existing password
 * account (SSO_LINK_REQUIRED). It only collects and submits the password —
 * the authoritative check and the actual link happen server-side on
 * POST /auth/sso/:provider/link. A wrong password keeps the dialog open with
 * the inline error; nothing is linked until the password checks out.
 */
export function LinkAccountDialog({
  open,
  email,
  pending,
  errorKey,
  onSubmit,
  onClose,
}: LinkAccountDialogProps) {
  const { t } = useTranslation();
  const [password, setPassword] = useState("");
  // Hide the inline error the moment the player edits the password — a stale
  // "wrong password" shouldn't linger while they retype. Re-armed whenever the
  // parent reports a fresh error (errorKey transitions to non-null).
  const [errorDismissed, setErrorDismissed] = useState(false);
  const [showPassword, setShowPassword] = useState(false);

  // Clear any stale password whenever the dialog (re)opens.
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
    // While the link request is in flight the dialog is not dismissable —
    // Escape/backdrop dismissal arrives here as onOpenChange(false) and is
    // ignored, so the in-flight state can't be silently abandoned.
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next && !pending) onClose();
      }}
    >
      <DialogContent showCloseButton={false} data-testid="link-account-dialog">
        <form onSubmit={handleSubmit} className="grid gap-4">
          <DialogHeader>
            <div className="flex items-center gap-3">
              <div className="bg-brass-soft border-brass/40 text-brass-deep flex size-11 shrink-0 items-center justify-center rounded-xl border">
                <Link2 className="size-5" />
              </div>
              <DialogTitle>{t("auth.sso.linkDialog.title")}</DialogTitle>
            </div>
            <DialogDescription data-testid="link-account-email">
              {email === ""
                ? t("auth.sso.linkDialog.descriptionNoEmail")
                : t("auth.sso.linkDialog.description", { email })}
            </DialogDescription>
          </DialogHeader>

          <div className="relative">
            <Input
              type={showPassword ? "text" : "password"}
              autoFocus
              autoComplete="current-password"
              placeholder={t("auth.sso.linkDialog.passwordPlaceholder")}
              value={password}
              onChange={(e) => {
                setPassword(e.target.value);
                setErrorDismissed(true);
              }}
              aria-invalid={showError}
              data-testid="link-account-password-input"
              className="h-11 pr-10"
            />
            <button
              type="button"
              tabIndex={-1}
              className="text-ink-mute hover:text-ink absolute top-1/2 right-2.5 -translate-y-1/2 p-1.5"
              onClick={() => setShowPassword(!showPassword)}
              data-testid="link-account-password-toggle"
              aria-label={showPassword ? t("common.hidePassword") : t("common.showPassword")}
            >
              {showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
            </button>
          </div>

          {showError && (
            <p role="alert" data-testid="link-account-error" className="text-destructive text-sm">
              {t(errorKey)}
            </p>
          )}

          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              disabled={pending}
              onClick={onClose}
              data-testid="link-account-cancel"
            >
              {t("auth.sso.linkDialog.cancel")}
            </Button>
            <Button
              type="submit"
              disabled={pending || password.length === 0}
              data-testid="link-account-submit"
            >
              {pending ? t("auth.sso.linkDialog.submitting") : t("auth.sso.linkDialog.submit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
