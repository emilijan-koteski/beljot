import { TriangleAlert } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { toast } from "sonner";

import { Button } from "@/shared/components/ui/button";
import { useDeleteAccountMutation } from "@/shared/hooks/mutations/useProfile";
import { useIdentitiesQuery } from "@/shared/hooks/queries/useIdentities";
import { useAuthStore } from "@/shared/stores/authStore";

import { DeleteAccountDialog } from "./DeleteAccountDialog";
import { SettingsGroup, SettingsRow } from "./SettingsGroup";

/**
 * The danger group of Settings: permanently deleting the account after a confirmation dialog.
 * On success the session ends locally and the player lands on the landing page.
 */
export function DeleteAccountSection() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const user = useAuthStore((s) => s.user);
  const identities = useIdentitiesQuery(user?.id);
  const mutation = useDeleteAccountMutation(user?.id ?? 0);
  const [open, setOpen] = useState(false);

  if (!user) return null;
  const username = user.username;
  const confirm = identities.data?.hasPassword === false ? "username" : "password";

  function handleSubmit(value: string) {
    if (mutation.isPending) return;
    const body = confirm === "password" ? { password: value } : { username: value.trim() };
    mutation.mutate(body, {
      onSuccess: () => {
        setOpen(false);
        navigate("/");
        toast.success(t("settings.deleteAccount.deletedToast"));
      },
    });
  }

  return (
    <SettingsGroup
      id="danger"
      title={t("settings.sections.danger")}
      icon={<TriangleAlert />}
      testId="settings-danger"
    >
      <SettingsRow
        label={t("settings.deleteAccount.label")}
        hint={t("settings.deleteAccount.hint")}
        control={
          <Button
            variant="outline"
            size="sm"
            className="text-danger hover:text-danger shrink-0"
            disabled={identities.data === undefined}
            onClick={() => {
              mutation.reset();
              setOpen(true);
            }}
            data-testid="settings-delete-account"
          >
            {t("settings.deleteAccount.open")}
          </Button>
        }
      />
      <DeleteAccountDialog
        open={open}
        confirm={confirm}
        username={username}
        pending={mutation.isPending}
        error={mutation.error}
        onSubmit={handleSubmit}
        onClose={() => setOpen(false)}
      />
    </SettingsGroup>
  );
}
