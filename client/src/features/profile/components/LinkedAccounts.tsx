import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { GoogleSignInButton } from "@/features/auth/components/GoogleSignInButton";
import { FetchError } from "@/shared/api/axiosClient";
import { Button } from "@/shared/components/ui/button";
import { Skeleton } from "@/shared/components/ui/skeleton";
import {
  useLinkIdentityMutation,
  useUnlinkIdentityMutation,
} from "@/shared/hooks/mutations/useIdentities";
import { useIdentitiesQuery } from "@/shared/hooks/queries/useIdentities";

import { SidePanel } from "./SidePanel";
import { UnlinkAccountDialog } from "./UnlinkAccountDialog";

interface LinkedAccountsProps {
  userId: number | undefined;
}

const GOOGLE = "google";

/**
 * Profile sidebar panel that links or unlinks the Google SSO account.
 * Unlink is disabled with a hint when it would remove a passwordless account's only sign-in.
 */
export function LinkedAccounts({ userId }: LinkedAccountsProps) {
  const { t } = useTranslation();
  const { data, isPending, isError } = useIdentitiesQuery(userId);
  const linkMutation = useLinkIdentityMutation(userId ?? 0);
  const unlinkMutation = useUnlinkIdentityMutation(userId ?? 0);
  const [unlinkOpen, setUnlinkOpen] = useState(false);
  const linkInFlightSyncGuard = useRef(false);

  const google = data?.identities.find((i) => i.provider === GOOGLE);
  const canUnlink = data ? data.hasPassword || data.identities.length > 1 : false;

  function handleGoogleCredential(credential: string) {
    if (userId === undefined || linkInFlightSyncGuard.current || linkMutation.isPending) return;
    linkInFlightSyncGuard.current = true;
    linkMutation.mutate(
      { provider: GOOGLE, credential },
      {
        onSuccess: () => toast.success(t("profile.linkedAccounts.linkedToast")),
        onError: (err) => {
          if (err instanceof FetchError && err.code === "SSO_IDENTITY_IN_USE") {
            toast.error(t("profile.linkedAccounts.errors.alreadyLinked"));
          } else if (err instanceof FetchError && err.code === "SSO_EMAIL_UNVERIFIED") {
            toast.error(t("profile.linkedAccounts.errors.emailUnverified"));
          } else if (err instanceof FetchError && err.code === "SSO_INVALID_CREDENTIAL") {
            toast.error(t("profile.linkedAccounts.errors.credentialExpired"));
          } else {
            toast.error(t("profile.linkedAccounts.errors.linkFailed"));
          }
        },
        onSettled: () => {
          linkInFlightSyncGuard.current = false;
        },
      },
    );
  }

  function handleUnlinkConfirm() {
    if (unlinkMutation.isPending) return;
    unlinkMutation.mutate(GOOGLE, {
      onSuccess: () => {
        setUnlinkOpen(false);
        toast.success(t("profile.linkedAccounts.unlinkedToast"));
      },
      onError: (err) => {
        setUnlinkOpen(false);
        if (err instanceof FetchError && err.code === "SSO_CANNOT_UNLINK_LAST") {
          toast.error(t("profile.linkedAccounts.errors.cannotUnlinkLast"));
        } else {
          toast.error(t("profile.linkedAccounts.errors.unlinkFailed"));
        }
      },
    });
  }

  return (
    <SidePanel
      eyebrow={t("profile.linkedAccounts.eyebrow")}
      title={t("profile.linkedAccounts.title")}
      testId="profile-linked-accounts"
    >
      {isPending ? (
        <Skeleton className="h-14 rounded-md" testId="linked-accounts-loading" />
      ) : isError || !data ? (
        <p className="text-danger text-sm" data-testid="linked-accounts-error">
          {t("profile.linkedAccounts.loadError")}
        </p>
      ) : (
        <>
          <p className="text-ink-mute mb-2.5 text-xs">{t("profile.linkedAccounts.description")}</p>
          <div
            className="bg-surface-elevated border-border rounded-md border px-3 py-2.5"
            data-testid="linked-account-google"
          >
            {google ? (
              <div className="flex items-center justify-between gap-2.5">
                <div className="min-w-0">
                  <div className="text-ink flex items-center gap-2 text-sm font-medium">
                    <img src="/google.svg" alt="" aria-hidden="true" className="size-4 shrink-0" />
                    {t("profile.linkedAccounts.google")}
                  </div>
                  <div
                    className="text-ink-mute mt-0.5 truncate text-xs"
                    data-testid="linked-account-google-status"
                  >
                    {t("profile.linkedAccounts.linkedAs", { email: google.email })}
                  </div>
                </div>
                <div className="flex shrink-0 flex-col items-end gap-1">
                  <Button
                    type="button"
                    variant="destructive"
                    size="sm"
                    disabled={!canUnlink || unlinkMutation.isPending}
                    onClick={() => setUnlinkOpen(true)}
                    data-testid="unlink-google-button"
                  >
                    {t("profile.linkedAccounts.unlink")}
                  </Button>
                  {!canUnlink && (
                    <span
                      className="text-ink-mute max-w-40 text-right text-2xs leading-tight"
                      data-testid="unlink-google-hint"
                    >
                      {t("profile.linkedAccounts.passwordlessHint")}
                    </span>
                  )}
                </div>
              </div>
            ) : (
              <div className="flex flex-col gap-2.5">
                <div>
                  <div className="text-ink flex items-center gap-2 text-sm font-medium">
                    <img src="/google.svg" alt="" aria-hidden="true" className="size-4 shrink-0" />
                    {t("profile.linkedAccounts.google")}
                  </div>
                  <div
                    className="text-ink-mute mt-0.5 text-xs"
                    data-testid="linked-account-google-status"
                  >
                    {t("profile.linkedAccounts.notLinked")}
                  </div>
                </div>
                <div data-testid="link-google-slot">
                  <GoogleSignInButton onCredential={handleGoogleCredential} />
                </div>
              </div>
            )}
          </div>
        </>
      )}

      {google && (
        <UnlinkAccountDialog
          open={unlinkOpen}
          providerLabel={t("profile.linkedAccounts.google")}
          email={google.email}
          pending={unlinkMutation.isPending}
          onConfirm={handleUnlinkConfirm}
          onClose={() => {
            if (!unlinkMutation.isPending) setUnlinkOpen(false);
          }}
        />
      )}
    </SidePanel>
  );
}
