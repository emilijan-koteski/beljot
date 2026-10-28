import { Unlink } from "lucide-react";
import { useTranslation } from "react-i18next";

import { AppDialog } from "@/shared/components/ui/app-dialog";

interface UnlinkAccountDialogProps {
  open: boolean;
  providerLabel: string;
  email: string;
  pending: boolean;
  onConfirm: () => void;
  onClose: () => void;
}

/**
 * Confirmation shown before unlinking an SSO provider from the profile.
 * Not dismissable while the request is in flight.
 */
export function UnlinkAccountDialog({
  open,
  providerLabel,
  email,
  pending,
  onConfirm,
  onClose,
}: UnlinkAccountDialogProps) {
  const { t } = useTranslation();

  return (
    <AppDialog
      open={open}
      onClose={onClose}
      tone="danger"
      icon={<Unlink />}
      title={t("profile.linkedAccounts.unlinkDialog.title", { provider: providerLabel })}
      description={
        <span data-testid="unlink-account-email">
          {email === ""
            ? t("profile.linkedAccounts.unlinkDialog.descriptionNoEmail")
            : t("profile.linkedAccounts.unlinkDialog.description", { email })}
        </span>
      }
      pending={pending}
      primary={{
        label: pending
          ? t("profile.linkedAccounts.unlinkDialog.submitting")
          : t("profile.linkedAccounts.unlinkDialog.confirm"),
        onClick: onConfirm,
        testId: "unlink-account-confirm",
      }}
      secondary={{
        label: t("profile.linkedAccounts.unlinkDialog.cancel"),
        onClick: onClose,
        testId: "unlink-account-cancel",
      }}
      testId="unlink-account-dialog"
    />
  );
}
