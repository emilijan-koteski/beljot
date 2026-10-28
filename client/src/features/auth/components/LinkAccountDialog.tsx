import { Link2 } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { PasswordToggle } from "@/shared/components/PasswordToggle";
import { AppDialog } from "@/shared/components/ui/app-dialog";
import { Input } from "@/shared/components/ui/input";

/** Props for LinkAccountDialog; errorKey is the i18n key of an inline error shown while the player retries. */
interface LinkAccountDialogProps {
  open: boolean;
  email: string;
  pending: boolean;
  errorKey: string | null;
  onSubmit: (password: string) => void;
  onClose: () => void;
}

/**
 * Password confirmation shown when an SSO sign-in matches an existing password account.
 * It only collects the password for the server-side link, and cannot be dismissed while pending.
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
      icon={<Link2 />}
      title={t("auth.sso.linkDialog.title")}
      description={
        <span data-testid="link-account-email">
          {email === ""
            ? t("auth.sso.linkDialog.descriptionNoEmail")
            : t("auth.sso.linkDialog.description", { email })}
        </span>
      }
      error={showError ? t(errorKey) : null}
      errorTestId="link-account-error"
      pending={pending}
      onSubmit={() => onSubmit(password)}
      primary={{
        label: pending ? t("auth.sso.linkDialog.submitting") : t("auth.sso.linkDialog.submit"),
        disabled: pending || password.length === 0,
        testId: "link-account-submit",
      }}
      secondary={{
        label: t("auth.sso.linkDialog.cancel"),
        onClick: onClose,
        testId: "link-account-cancel",
      }}
      testId="link-account-dialog"
    >
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
        <PasswordToggle
          shown={showPassword}
          onToggle={() => setShowPassword(!showPassword)}
          testId="link-account-password-toggle"
        />
      </div>
    </AppDialog>
  );
}
