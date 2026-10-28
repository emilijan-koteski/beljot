import { Lock } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { PasswordToggle } from "@/shared/components/PasswordToggle";
import { AppDialog } from "@/shared/components/ui/app-dialog";
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

  return (
    <AppDialog
      open={open}
      onClose={onClose}
      icon={<Lock />}
      title={t("room.passwordPrompt.title")}
      description={t("room.passwordPrompt.description", { name: roomName })}
      error={showError ? t(errorKey) : null}
      errorTestId="password-prompt-error"
      pending={pending}
      onSubmit={() => onSubmit(password)}
      primary={{
        label: pending ? t("room.passwordPrompt.submitting") : t("room.passwordPrompt.submit"),
        disabled: pending || password.length === 0,
        testId: "password-prompt-submit",
      }}
      secondary={{
        label: t("room.passwordPrompt.cancel"),
        onClick: onClose,
        testId: "password-prompt-cancel",
      }}
      testId="password-prompt-dialog"
    >
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
    </AppDialog>
  );
}
