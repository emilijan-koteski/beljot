import { Eye, EyeOff } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "react-router";
import { toast } from "sonner";

import { AltLink, AuthCard, Field } from "@/features/auth/components/AuthCard";
import { GoogleSignInButton } from "@/features/auth/components/GoogleSignInButton";
import { LinkAccountDialog } from "@/features/auth/components/LinkAccountDialog";
import { reconcileLanguagePreference } from "@/features/auth/reconcileLanguage";
import { useGoogleSso } from "@/features/auth/useGoogleSso";
import { FetchError } from "@/shared/api/axiosClient";
import { Button } from "@/shared/components/ui/button";
import { Input } from "@/shared/components/ui/input";
import { useLoginMutation } from "@/shared/hooks/mutations/useAuth";

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

interface FieldErrors {
  email?: string;
  password?: string;
}

export function LoginPage() {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const loginMutation = useLoginMutation();
  const { handleGoogleCredential, linkDialogProps } = useGoogleSso();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [errors, setErrors] = useState<FieldErrors>({});
  const [formError, setFormError] = useState<string | null>(null);

  function validateEmail(value: string): string | undefined {
    if (!value) return t("auth.login.errors.emailRequired");
    if (!EMAIL_REGEX.test(value)) return t("auth.login.errors.emailInvalid");
    return undefined;
  }

  function validatePassword(value: string): string | undefined {
    if (!value) return t("auth.login.errors.passwordRequired");
    return undefined;
  }

  function handleBlur(field: keyof FieldErrors) {
    let error: string | undefined;
    if (field === "email") error = validateEmail(email);
    if (field === "password") error = validatePassword(password);

    setErrors((prev) => ({ ...prev, [field]: error }));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();

    const emailError = validateEmail(email);
    const passwordError = validatePassword(password);

    if (emailError || passwordError) {
      setErrors({ email: emailError, password: passwordError });
      return;
    }

    setErrors({});
    setFormError(null);

    try {
      const res = await loginMutation.mutateAsync({ email, password });
      await reconcileLanguagePreference(res, i18n.language);
      navigate("/lobby");
    } catch (err) {
      if (err instanceof FetchError) {
        if (err.status === 401) {
          setFormError(t("auth.login.errors.invalidCredentials"));
        } else {
          toast.error(t("auth.login.errors.loginFailed"));
        }
      } else {
        toast.error(t("auth.login.errors.loginFailed"));
      }
    }
  }

  return (
    <AuthCard
      eyebrow={t("auth.login.eyebrow")}
      title={t("auth.login.title")}
      subtitle={t("auth.login.subtitle")}
      footer={
        <AltLink
          prompt={t("auth.login.altPrompt")}
          cta={t("auth.login.altCta")}
          to="/register"
          testId="register-link"
        />
      }
    >
      <h1 data-testid="login-title" className="sr-only">
        {t("auth.login.title")}
      </h1>

      <form onSubmit={handleSubmit} className="flex flex-col gap-4" data-testid="login-form">
        <Field
          label={t("auth.login.emailLabel")}
          htmlFor="email"
          error={errors.email}
          errorTestId="email-error"
        >
          <Input
            id="email"
            type="email"
            className="h-10.5"
            placeholder={t("auth.login.emailPlaceholder")}
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            onBlur={() => handleBlur("email")}
            aria-invalid={!!errors.email}
            autoFocus
            data-testid="email-input"
          />
        </Field>

        <Field
          label={t("auth.login.passwordLabel")}
          htmlFor="password"
          hint={
            <Link
              to="/forgot-password"
              className="text-brass-deep/90 hover:text-accent border-b border-dotted border-current pb-px"
              data-testid="forgot-password-link"
            >
              {t("auth.login.forgotLink")}
            </Link>
          }
          error={errors.password}
          errorTestId="password-error"
        >
          <div className="relative">
            <Input
              id="password"
              type={showPassword ? "text" : "password"}
              className="h-10.5 pr-10"
              placeholder={t("auth.login.passwordPlaceholder")}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              onBlur={() => handleBlur("password")}
              aria-invalid={!!errors.password}
              data-testid="password-input"
            />
            <button
              type="button"
              tabIndex={-1}
              className="text-ink-mute hover:text-ink absolute top-1/2 right-2.5 -translate-y-1/2 p-1.5"
              onClick={() => setShowPassword(!showPassword)}
              data-testid="password-toggle"
              aria-label={showPassword ? t("common.hidePassword") : t("common.showPassword")}
            >
              {showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
            </button>
          </div>
        </Field>

        {formError && (
          <div
            className="text-destructive rounded-md border border-destructive/30 bg-destructive/6 px-3 py-2 text-xs font-medium"
            data-testid="form-error"
          >
            {formError}
          </div>
        )}

        <div className="mt-1.5">
          <Button
            type="submit"
            size="cta"
            className="w-full"
            disabled={loginMutation.isPending || !email || !password}
            data-testid="submit-button"
          >
            {loginMutation.isPending ? t("auth.login.submitting") : t("auth.login.submitButton")}
          </Button>
        </div>
      </form>

      <div className="mt-4.5 mb-3.5 flex items-center gap-3" data-testid="sso-divider">
        <div className="bg-border h-px flex-1" />
        <span className="text-ink-mute text-[11.5px] font-medium tracking-[1.6px] uppercase">
          {t("auth.sso.divider")}
        </span>
        <div className="bg-border h-px flex-1" />
      </div>

      <GoogleSignInButton onCredential={handleGoogleCredential} />

      {/* The Google button on /login can REGISTER a brand-new account, so the
          same ToS/privacy small-print as on the register page applies here. */}
      <p
        className="text-ink-mute mt-3 text-center text-xs leading-normal"
        data-testid="sso-consent-note"
      >
        {t("auth.sso.consent.prefix")}
        <Link
          to="/terms"
          target="_blank"
          rel="noopener noreferrer"
          className="text-accent border-accent/30 border-b hover:underline"
          data-testid="sso-terms-link"
        >
          {t("auth.sso.consent.termsLink")}
        </Link>
        {t("auth.sso.consent.and")}
        <Link
          to="/privacy"
          target="_blank"
          rel="noopener noreferrer"
          className="text-accent border-accent/30 border-b hover:underline"
          data-testid="sso-privacy-link"
        >
          {t("auth.sso.consent.privacyLink")}
        </Link>
        {t("auth.sso.consent.suffix")}
      </p>

      <LinkAccountDialog {...linkDialogProps} />
    </AuthCard>
  );
}
