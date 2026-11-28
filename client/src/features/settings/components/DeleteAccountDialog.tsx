import { UserX } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { FetchError } from "@/shared/api/axiosClient";
import { PasswordToggle } from "@/shared/components/PasswordToggle";
import { AppDialog } from "@/shared/components/ui/app-dialog";
import { Input } from "@/shared/components/ui/input";

export type DeleteAccountConfirm = "password" | "username";

interface DeleteAccountDialogProps {
  open: boolean;
  confirm: DeleteAccountConfirm;
  username: string;
  pending: boolean;
  error: unknown;
  onSubmit: (value: string) => void;
  onClose: () => void;
}

const LOST_ITEMS = ["account", "profile", "friends", "seasons", "avatar"] as const;

/** Maps a failed deletion to the line shown under the confirmation field. */
function errorKey(error: unknown, confirm: DeleteAccountConfirm): string | null {
  if (error === null || error === undefined) return null;
  if (error instanceof FetchError && error.code === "ACCOUNT_DELETE_IN_ROOM") {
    return "settings.deleteAccount.errors.inRoom";
  }
  if (error instanceof FetchError && error.code === "ACCOUNT_DELETE_CONFIRMATION_MISMATCH") {
    return confirm === "password"
      ? "settings.deleteAccount.errors.wrongPassword"
      : "settings.deleteAccount.errors.wrongUsername";
  }
  return "settings.deleteAccount.errors.failed";
}

/**
 * Final confirmation before erasing the account: what goes, what stays, and that it cannot be undone.
 * The player confirms with the password, or by typing the username when the account has no password.
 */
export function DeleteAccountDialog({
  open,
  confirm,
  username,
  pending,
  error,
  onSubmit,
  onClose,
}: DeleteAccountDialogProps) {
  const { t } = useTranslation();
  const [value, setValue] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [errorDismissed, setErrorDismissed] = useState(false);

  useEffect(() => {
    if (open) {
      setValue("");
      setShowPassword(false);
      setErrorDismissed(false);
    }
  }, [open]);

  useEffect(() => {
    setErrorDismissed(false);
  }, [error]);

  const key = errorDismissed ? null : errorKey(error, confirm);
  const ready =
    confirm === "password" ? value.length > 0 : value.trim() === username && username !== "";

  return (
    <AppDialog
      open={open}
      onClose={onClose}
      tone="danger"
      icon={<UserX />}
      title={t("settings.deleteAccount.dialog.title")}
      description={t("settings.deleteAccount.dialog.description")}
      error={key === null ? null : t(key)}
      errorTestId="delete-account-error"
      pending={pending}
      onSubmit={() => onSubmit(value)}
      primary={{
        label: pending
          ? t("settings.deleteAccount.dialog.submitting")
          : t("settings.deleteAccount.dialog.confirm"),
        disabled: pending || !ready,
        testId: "delete-account-confirm",
      }}
      secondary={{
        label: t("settings.deleteAccount.dialog.cancel"),
        onClick: onClose,
        testId: "delete-account-cancel",
      }}
      testId="delete-account-dialog"
    >
      <div className="flex flex-col gap-4">
        <div className="flex flex-col gap-1.5 text-sm">
          <p className="text-ink m-0 font-semibold">
            {t("settings.deleteAccount.dialog.lostTitle")}
          </p>
          <ul className="text-ink-dim m-0 list-disc pl-5" data-testid="delete-account-lost">
            {LOST_ITEMS.map((item) => (
              <li key={item}>{t(`settings.deleteAccount.dialog.lost.${item}`)}</li>
            ))}
          </ul>
          <p className="text-ink-dim m-0">{t("settings.deleteAccount.dialog.kept")}</p>
          <p className="text-danger m-0 font-semibold">
            {t("settings.deleteAccount.dialog.permanent")}
          </p>
        </div>
        <label className="flex flex-col gap-1.5">
          <span className="text-ink text-sm font-semibold">
            {confirm === "password"
              ? t("settings.deleteAccount.dialog.passwordLabel")
              : t("settings.deleteAccount.dialog.usernameLabel", { username })}
          </span>
          <span className="relative">
            <Input
              type={confirm === "password" && !showPassword ? "password" : "text"}
              autoFocus
              autoComplete={confirm === "password" ? "current-password" : "off"}
              spellCheck={false}
              value={value}
              onChange={(e) => {
                setValue(e.target.value);
                setErrorDismissed(true);
              }}
              aria-invalid={key !== null}
              data-testid="delete-account-input"
              className={confirm === "password" ? "h-11 pr-10" : "h-11"}
            />
            {confirm === "password" && (
              <PasswordToggle
                shown={showPassword}
                onToggle={() => setShowPassword(!showPassword)}
                testId="delete-account-toggle"
              />
            )}
          </span>
        </label>
      </div>
    </AppDialog>
  );
}
