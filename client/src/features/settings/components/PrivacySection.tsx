import { ShieldCheck } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Switch } from "@/shared/components/ui/switch";
import {
  persistAllowNonFriendInvites,
  resolveAllowNonFriendInvites,
} from "@/shared/lib/invitePreference";
import { useAuthStore } from "@/shared/stores/authStore";

import { SettingsGroup, SettingsRow } from "./SettingsGroup";

/**
 * The Privacy group of Settings: whether players who are not friends can invite you, saved optimistically.
 * It is a private preference, so it never appears on a public profile.
 */
export function PrivacySection() {
  const { t } = useTranslation();
  const allowed = resolveAllowNonFriendInvites(useAuthStore((s) => s.user?.allowNonFriendInvites));

  return (
    <SettingsGroup
      id="privacy"
      title={t("settings.sections.privacy")}
      icon={<ShieldCheck />}
      testId="profile-invites"
    >
      <SettingsRow
        asLabel
        label={t("profile.invites.allowNonFriends")}
        hint={t("profile.invites.description")}
        control={
          <Switch
            checked={allowed}
            onCheckedChange={(checked) => void persistAllowNonFriendInvites(checked)}
            data-testid="profile-allow-non-friend-invites"
          />
        }
      />
    </SettingsGroup>
  );
}
