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

import { UnlinkAccountDialog } from "./UnlinkAccountDialog";

interface GoogleAccountRowProps {
  userId: number | undefined;
}

const GOOGLE = "google";

/**
 * The Google row of Settings' sign-in group: links or unlinks the Google SSO account.
 * Unlink is disabled with a hint when it would remove a passwordless account's only sign-in.
 */
export function GoogleAccountRow({ userId }: GoogleAccountRowProps) {
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

  let status: string | null = null;
  if (data) {
    status = google
      ? t("profile.linkedAccounts.linkedAs", { email: google.email })
      : t("profile.linkedAccounts.notLinked");
  }

  return (
    <div className="flex flex-col gap-3 px-4 py-3.5" data-testid="profile-linked-accounts">
      {isPending ? (
        <Skeleton className="h-10 rounded-md" testId="linked-accounts-loading" />
      ) : isError || !data ? (
        <p className="text-danger m-0 text-sm" data-testid="linked-accounts-error">
          {t("profile.linkedAccounts.loadError")}
        </p>
      ) : (
        <div className="flex flex-col gap-3" data-testid="linked-account-google">
          <div className="flex items-center gap-3">
            <span className="border-border bg-surface-elevated flex size-9 shrink-0 items-center justify-center rounded-full border">
              <img src="/google.svg" alt="" aria-hidden="true" className="size-4" />
            </span>
            <div className="flex min-w-0 flex-1 flex-col gap-0.5">
              <span className="text-ink text-sm font-semibold">
                {t("profile.linkedAccounts.google")}
              </span>
              <span
                className="text-ink-mute truncate text-xs"
                data-testid="linked-account-google-status"
              >
                {status}
              </span>
              {google && !canUnlink && (
                <span
                  className="text-ink-mute text-2xs leading-tight"
                  data-testid="unlink-google-hint"
                >
                  {t("profile.linkedAccounts.passwordlessHint")}
                </span>
              )}
            </div>
            {google && (
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="shrink-0"
                disabled={!canUnlink || unlinkMutation.isPending}
                onClick={() => setUnlinkOpen(true)}
                data-testid="unlink-google-button"
              >
                {t("profile.linkedAccounts.unlink")}
              </Button>
            )}
          </div>
          {!google && (
            <div data-testid="link-google-slot">
              <GoogleSignInButton onCredential={handleGoogleCredential} />
            </div>
          )}
        </div>
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
    </div>
  );
}
