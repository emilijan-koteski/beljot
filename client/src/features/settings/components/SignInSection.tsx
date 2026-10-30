import { Check, KeyRound } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { Badge } from "@/shared/components/ui/badge";
import { Button } from "@/shared/components/ui/button";
import { useForgotPasswordMutation } from "@/shared/hooks/mutations/useAuth";
import { useAuthStore } from "@/shared/stores/authStore";

import { GoogleAccountRow } from "./GoogleAccountRow";
import { SettingsGroup, SettingsRow } from "./SettingsGroup";

/** The password row: emails a link to set a new password, through the same flow as "Forgot password". */
function PasswordRow({ email }: { email: string }) {
  const { t } = useTranslation();
  const mutation = useForgotPasswordMutation();
  const sent = mutation.isSuccess;

  function send() {
    if (mutation.isPending || sent) return;
    mutation.mutate(
      { email },
      { onError: () => toast.error(t("auth.forgotPassword.errors.requestFailed")) },
    );
  }

  return (
    <SettingsRow
      testId="settings-password"
      label={t("settings.password.label")}
      hint={sent ? t("settings.password.sent", { email }) : t("auth.forgotPassword.subtitle")}
      control={
        sent ? (
          <Badge tone="accent" size="sm" icon={<Check className="size-3" />}>
            {t("settings.password.sentBadge")}
          </Badge>
        ) : (
          <Button
            variant="outline"
            size="sm"
            className="shrink-0"
            onClick={send}
            disabled={mutation.isPending}
            data-testid="settings-password-send"
          >
            {mutation.isPending ? t("settings.password.sending") : t("settings.password.send")}
          </Button>
        )
      }
    />
  );
}

/** The Sign-in group of Settings: a password reset link and the linked Google account. */
export function SignInSection() {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  if (!user) return null;

  return (
    <SettingsGroup
      id="signIn"
      title={t("settings.sections.signIn")}
      icon={<KeyRound />}
      testId="settings-sign-in"
    >
      <PasswordRow email={user.email} />
      <GoogleAccountRow userId={user.id} />
    </SettingsGroup>
  );
}
