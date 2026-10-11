import { Unlink } from "lucide-react";
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
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next && !pending) onClose();
      }}
    >
      <DialogContent showCloseButton={false} data-testid="unlink-account-dialog">
        <DialogHeader>
          <div className="flex items-center gap-3">
            <div className="bg-brass-soft border-brass/40 text-brass-deep flex size-11 shrink-0 items-center justify-center rounded-xl border">
              <Unlink className="size-5" />
            </div>
            <DialogTitle>
              {t("profile.linkedAccounts.unlinkDialog.title", { provider: providerLabel })}
            </DialogTitle>
          </div>
          <DialogDescription data-testid="unlink-account-email">
            {email === ""
              ? t("profile.linkedAccounts.unlinkDialog.descriptionNoEmail", {
                  provider: providerLabel,
                })
              : t("profile.linkedAccounts.unlinkDialog.description", {
                  provider: providerLabel,
                  email,
                })}
          </DialogDescription>
        </DialogHeader>

        <DialogFooter>
          <Button
            type="button"
            variant="destructive"
            disabled={pending}
            onClick={onConfirm}
            data-testid="unlink-account-confirm"
          >
            {pending
              ? t("profile.linkedAccounts.unlinkDialog.submitting")
              : t("profile.linkedAccounts.unlinkDialog.confirm")}
          </Button>
          <Button
            type="button"
            variant="ghost"
            disabled={pending}
            onClick={onClose}
            data-testid="unlink-account-cancel"
          >
            {t("profile.linkedAccounts.unlinkDialog.cancel")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
