import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { toast } from "sonner";

import { reconcileLanguagePreference } from "@/features/auth/reconcileLanguage";
import { FetchError } from "@/shared/api/axiosClient";
import { useSSOLinkMutation, useSSOLoginMutation } from "@/shared/hooks/mutations/useAuth";
import { decodeCredentialEmail } from "@/shared/lib/googleIdentity";

interface LinkDialogProps {
  open: boolean;
  email: string;
  pending: boolean;
  errorKey: string | null;
  onSubmit: (password: string) => void;
  onClose: () => void;
}

/** Wire handleGoogleCredential to GoogleSignInButton and spread linkDialogProps onto LinkAccountDialog. */
interface UseGoogleSsoResult {
  handleGoogleCredential: (credential: string) => Promise<void>;
  linkDialogProps: LinkDialogProps;
}

/**
 * Runs the Google SSO login, register, or link flow shared by LoginPage and RegisterPage.
 * Success paths reconcile the picked UI language and navigate to /lobby.
 */
export function useGoogleSso(): UseGoogleSsoResult {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const ssoLoginMutation = useSSOLoginMutation();
  const ssoLinkMutation = useSSOLinkMutation();

  const [linkCredential, setLinkCredential] = useState<string | null>(null);
  const [linkEmail, setLinkEmail] = useState("");
  const [linkErrorKey, setLinkErrorKey] = useState<string | null>(null);
  const inFlightRef = useRef(false);

  async function handleGoogleCredential(credential: string): Promise<void> {
    if (inFlightRef.current || ssoLoginMutation.isPending || ssoLinkMutation.isPending) return;
    inFlightRef.current = true;
    try {
      const res = await ssoLoginMutation.mutateAsync({ provider: "google", credential });
      await reconcileLanguagePreference(res, i18n.language);
      navigate("/lobby", { replace: true });
    } catch (err) {
      if (err instanceof FetchError && err.code === "SSO_LINK_REQUIRED") {
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
      if (err instanceof FetchError && err.code === "INVALID_CREDENTIALS") {
        setLinkErrorKey("auth.sso.linkDialog.errors.wrongPassword");
        return;
      }
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
