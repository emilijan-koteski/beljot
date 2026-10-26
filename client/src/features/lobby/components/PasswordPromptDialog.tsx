import { Lock } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { PasswordToggle } from "@/shared/components/PasswordToggle";
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
  roomName: string;
  pending: boolean;
  errorKey: string | null;
  onSubmit: (password: string) => void;
  onClose: () => void;
}

/**
 * Password prompt shown before joining a private room; the server does the real check.
 * Stays open showing the parent-supplied errorKey so the player can retry.
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
  const [errorDismissed, setErrorDismissed] = useState(false);
  const [showPassword, setShowPassword] = useState(false);

  useEffect(() => {
    if (open) {
      setPassword("");
      setErrorDismissed(false);
      setShowPassword(false);
    }
  }, [open]);

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
            <PasswordToggle
              shown={showPassword}
              onToggle={() => setShowPassword(!showPassword)}
              testId="password-prompt-toggle"
            />
          </div>

          {showError && (
            <p role="alert" data-testid="password-prompt-error" className="text-danger text-sm">
              {t(errorKey)}
            </p>
          )}

          <DialogFooter>
            <Button
              type="submit"
              disabled={pending || password.length === 0}
              data-testid="password-prompt-submit"
            >
              {pending ? t("room.passwordPrompt.submitting") : t("room.passwordPrompt.submit")}
            </Button>
            <Button
              type="button"
              variant="ghost"
              onClick={onClose}
              data-testid="password-prompt-cancel"
            >
              {t("room.passwordPrompt.cancel")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
