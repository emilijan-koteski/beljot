import { ShieldCheck } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import { Switch } from "@/shared/components/ui/switch";
import {
  persistAllowNonFriendInvites,
  resolveAllowNonFriendInvites,
} from "@/shared/lib/invitePreference";
import { useAuthStore } from "@/shared/stores/authStore";

import { SettingsGroup, SettingsRow } from "./SettingsGroup";

/**
 * The Privacy group of Settings: the non-friend invite switch, saved optimistically, then links to the legal pages.
 * The switch is a private preference, so it never appears on a public profile.
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
      <div
        className="text-ink-mute flex flex-wrap gap-x-5 gap-y-2 px-4 py-3 text-xs"
        data-testid="settings-legal-links"
      >
        <Link
          to="/privacy"
          target="_blank"
          rel="noopener noreferrer"
          className="hover:text-ink underline-offset-2 hover:underline"
        >
          {t("legal.privacy.title")}
        </Link>
        <Link
          to="/terms"
          target="_blank"
          rel="noopener noreferrer"
          className="hover:text-ink underline-offset-2 hover:underline"
        >
          {t("legal.terms.title")}
        </Link>
      </div>
    </SettingsGroup>
  );
}
