import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { toast } from "sonner";

import { reconcileLanguagePreference } from "@/features/auth/reconcileLanguage";
import { FetchError } from "@/shared/api/axiosClient";
import { useSSOLinkMutation, useSSOLoginMutation } from "@/shared/hooks/mutations/useAuth";
import { decodeCredentialEmail } from "@/shared/lib/googleIdentity";

export interface LinkDialogProps {
  open: boolean;
  email: string;
  pending: boolean;
  errorKey: string | null;
  onSubmit: (password: string) => void;
  onClose: () => void;
}

export interface UseGoogleSsoResult {
  /** Wire to GoogleSignInButton's onCredential. */
  handleGoogleCredential: (credential: string) => Promise<void>;
  /** Spread onto LinkAccountDialog. */
  linkDialogProps: LinkDialogProps;
}

/**
 * The complete Google SSO flow shared by LoginPage and RegisterPage (the
 * endpoint is one and the same — it logs in, registers, or asks to link based
 * on server state): takes the GIS credential, runs the SSO login mutation,
 * and on SSO_LINK_REQUIRED owns the link-dialog state and the follow-up link
 * mutation. Success paths reconcile the picked UI language (SSO registration
 * seeds "en" server-side) and navigate to /lobby.
 */
export function useGoogleSso(): UseGoogleSsoResult {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const ssoLoginMutation = useSSOLoginMutation();
  const ssoLinkMutation = useSSOLinkMutation();

  // The credential that hit the email collision is held until the password
  // dialog confirms or is dismissed.
  const [linkCredential, setLinkCredential] = useState<string | null>(null);
  const [linkEmail, setLinkEmail] = useState("");
  const [linkErrorKey, setLinkErrorKey] = useState<string | null>(null);
  // GIS can fire the global callback again (double-tap, or a stale button)
  // before React re-renders with isPending — a ref guards synchronously.
  const inFlightRef = useRef(false);

  async function handleGoogleCredential(credential: string): Promise<void> {
    if (inFlightRef.current || ssoLoginMutation.isPending || ssoLinkMutation.isPending) return;
    inFlightRef.current = true;
    try {
      const res = await ssoLoginMutation.mutateAsync({ provider: "google", credential });
      await reconcileLanguagePreference(res, i18n.language);
      // Replace: /lobby is the app root — back must not return to the auth page.
      navigate("/lobby", { replace: true });
    } catch (err) {
      if (err instanceof FetchError && err.code === "SSO_LINK_REQUIRED") {
        // An account with this email already exists — confirm its password
        // before linking. Nothing was linked server-side yet.
        setLinkEmail(decodeCredentialEmail(credential) ?? "");
        setLinkErrorKey(null);
        setLinkCredential(credential);
        return;
      }
      if (err instanceof FetchError && err.code === "SSO_EMAIL_UNVERIFIED") {
        toast.error(t("auth.sso.errors.emailUnverified"));
        return;
      }
      toast.error(t("auth.sso.errors.ssoFailed"));
    } finally {
      inFlightRef.current = false;
    }
  }

  async function handleLinkSubmit(password: string): Promise<void> {
    if (linkCredential === null || ssoLinkMutation.isPending) return;
    setLinkErrorKey(null);
    try {
      const res = await ssoLinkMutation.mutateAsync({
        provider: "google",
        credential: linkCredential,
        password,
      });
      setLinkCredential(null);
      await reconcileLanguagePreference(res, i18n.language);
      navigate("/lobby", { replace: true });
    } catch (err) {
      // Discriminate by code, never by status: INVALID_CREDENTIALS and
      // SSO_INVALID_CREDENTIAL are both 401s, but only the first one is
      // fixable by retyping the password.
      if (err instanceof FetchError && err.code === "INVALID_CREDENTIALS") {
        setLinkErrorKey("auth.sso.linkDialog.errors.wrongPassword");
        return;
      }
      // The held Google credential itself is dead (typically expired while
      // the dialog sat open) — no password can fix that, so close the dialog
      // and send the player back to the Google button.
      setLinkCredential(null);
      setLinkErrorKey(null);
      if (err instanceof FetchError && err.code === "SSO_INVALID_CREDENTIAL") {
        toast.error(t("auth.sso.errors.credentialExpired"));
      } else {
        toast.error(t("auth.sso.linkDialog.errors.linkFailed"));
      }
    }
  }

  function handleLinkClose(): void {
    if (ssoLinkMutation.isPending) return;
    setLinkCredential(null);
    setLinkErrorKey(null);
  }

  return {
    handleGoogleCredential,
    linkDialogProps: {
      open: linkCredential !== null,
      email: linkEmail,
      pending: ssoLinkMutation.isPending,
      errorKey: linkErrorKey,
      onSubmit: (password) => void handleLinkSubmit(password),
      onClose: handleLinkClose,
    },
  };
}
