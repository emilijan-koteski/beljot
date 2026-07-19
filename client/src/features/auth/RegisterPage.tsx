import { Eye, EyeOff } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "react-router";
import { toast } from "sonner";

import { AltLink, AuthCard, Checkbox, Field } from "@/features/auth/components/AuthCard";
import { GoogleSignInButton } from "@/features/auth/components/GoogleSignInButton";
import { LinkAccountDialog } from "@/features/auth/components/LinkAccountDialog";
import { useGoogleSso } from "@/features/auth/useGoogleSso";
import { FetchError } from "@/shared/api/axiosClient";
import { Button } from "@/shared/components/ui/button";
import { Input } from "@/shared/components/ui/input";
import { useRegisterMutation } from "@/shared/hooks/mutations/useAuth";
import { normalizeLanguage } from "@/shared/i18n/i18n";
import {
  USERNAME_MAX_LENGTH,
  USERNAME_MIN_LENGTH,
  USERNAME_REGEX,
} from "@/shared/lib/usernameChange";

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

interface FieldErrors {
  email?: string;
  username?: string;
  password?: string;
  consent?: string;
}

export function RegisterPage() {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const registerMutation = useRegisterMutation();
  const { handleGoogleCredential, linkDialogProps } = useGoogleSso();

  const [email, setEmail] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [acceptedTerms, setAcceptedTerms] = useState(false);
  const [errors, setErrors] = useState<FieldErrors>({});

  function validateEmail(value: string): string | undefined {
    if (!value) return t("auth.register.errors.emailRequired");
    if (!EMAIL_REGEX.test(value)) return t("auth.register.errors.emailInvalid");
    return undefined;
  }

  function validateUsername(value: string): string | undefined {
    if (!value) return t("auth.register.errors.usernameRequired");
    if (value.length < USERNAME_MIN_LENGTH) return t("auth.register.errors.usernameTooShort");
    if (value.length > USERNAME_MAX_LENGTH) return t("auth.register.errors.usernameTooLong");
    if (!USERNAME_REGEX.test(value)) return t("auth.register.errors.usernameInvalidChars");
    return undefined;
  }

  function validatePassword(value: string): string | undefined {
    if (!value) return t("auth.register.errors.passwordRequired");
    if (value.length < 8) return t("auth.register.errors.passwordTooShort");
    if (value.length > 72) return t("auth.register.errors.passwordTooLong");
    return undefined;
  }

  function handleBlur(field: keyof FieldErrors) {
    let error: string | undefined;
    if (field === "email") error = validateEmail(email);
    if (field === "username") error = validateUsername(username);
    if (field === "password") error = validatePassword(password);

    setErrors((prev) => ({ ...prev, [field]: error }));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();

    const emailError = validateEmail(email);
    const usernameError = validateUsername(username);
    const passwordError = validatePassword(password);
    const consentError = acceptedTerms ? undefined : t("auth.register.errors.consentRequired");

    if (emailError || usernameError || passwordError || consentError) {
      setErrors({
        email: emailError,
        username: usernameError,
        password: passwordError,
        consent: consentError,
      });
      return;
    }

    setErrors({});

    try {
      await registerMutation.mutateAsync({
        email,
        username,
        password,
        // Normalize so a region-tagged i18n.language ("en-US") doesn't get
        // silently downgraded by the server to "en" — send the short code we
        // already display.
        languagePreference: normalizeLanguage(i18n.language) ?? "en",
      });
      // Replace: /lobby is the app root — back must not return to the form.
      navigate("/lobby", { replace: true });
    } catch (err) {
      if (err instanceof FetchError) {
        if (err.code === "EMAIL_TAKEN") {
          setErrors({ email: t("auth.register.errors.emailTaken") });
        } else if (err.code === "USERNAME_TAKEN") {
          setErrors({ username: t("auth.register.errors.usernameTaken") });
        } else if (err.code === "INVALID_EMAIL") {
          setErrors({ email: t("auth.register.errors.emailInvalid") });
        } else if (err.code === "PASSWORD_TOO_SHORT") {
          setErrors({ password: t("auth.register.errors.passwordTooShort") });
        } else if (err.code === "PASSWORD_TOO_LONG") {
          setErrors({ password: t("auth.register.errors.passwordTooLong") });
        } else if (err.code === "USERNAME_TOO_SHORT") {
          setErrors({ username: t("auth.register.errors.usernameTooShort") });
        } else if (err.code === "USERNAME_TOO_LONG") {
          setErrors({ username: t("auth.register.errors.usernameTooLong") });
        } else if (err.code === "USERNAME_INVALID_CHARS") {
          setErrors({ username: t("auth.register.errors.usernameInvalidChars") });
        } else {
          toast.error(t("auth.register.errors.registrationFailed"));
        }
      } else {
        toast.error(t("auth.register.errors.registrationFailed"));
      }
    }
  }

  return (
    <AuthCard
      eyebrow={t("auth.register.eyebrow")}
      title={t("auth.register.title")}
      subtitle={t("auth.register.subtitle")}
      footer={
        <AltLink
          prompt={t("auth.register.altPrompt")}
          cta={t("auth.register.altCta")}
          to="/login"
          testId="login-link"
        />
      }
    >
      <h1 data-testid="register-title" className="sr-only">
        {t("auth.register.title")}
      </h1>

      <form onSubmit={handleSubmit} className="flex flex-col gap-3.5" data-testid="register-form">
        <Field
          label={t("auth.register.emailLabel")}
          htmlFor="email"
          error={errors.email}
          errorTestId="email-error"
        >
          <Input
            id="email"
            type="email"
            className="h-10.5"
            placeholder={t("auth.register.emailPlaceholder")}
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            onBlur={() => handleBlur("email")}
            aria-invalid={!!errors.email}
            data-testid="email-input"
          />
        </Field>

        <Field
          label={t("auth.register.usernameLabel")}
          htmlFor="username"
          hint={
            <span className="tabular-nums">
              {username.length}/{USERNAME_MAX_LENGTH}
            </span>
          }
          error={errors.username}
          errorTestId="username-error"
        >
          <Input
            id="username"
            type="text"
            className="h-10.5"
            maxLength={USERNAME_MAX_LENGTH}
            placeholder={t("auth.register.usernamePlaceholder")}
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            onBlur={() => handleBlur("username")}
            aria-invalid={!!errors.username}
            data-testid="username-input"
          />
        </Field>

        <Field
          label={t("auth.register.passwordLabel")}
          htmlFor="password"
          hint={<span>min 8</span>}
          error={errors.password}
          errorTestId="password-error"
        >
          <div className="relative">
            <Input
              id="password"
              type={showPassword ? "text" : "password"}
              className="h-10.5 pr-10"
              placeholder={t("auth.register.passwordPlaceholder")}
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

        <div className="mt-1">
          <Checkbox
            checked={acceptedTerms}
            onChange={(next) => {
              setAcceptedTerms(next);
              if (next) {
                setErrors((prev) => ({ ...prev, consent: undefined }));
              }
            }}
            invalid={!!errors.consent}
            testId="consent-checkbox"
          >
            {t("auth.register.consent.prefix")}
            <Link
              to="/terms"
              target="_blank"
              rel="noopener noreferrer"
              className="text-accent border-accent/30 border-b hover:underline"
              data-testid="terms-link"
            >
              {t("auth.register.consent.termsLink")}
            </Link>
            {t("auth.register.consent.and")}
            <Link
              to="/privacy"
              target="_blank"
              rel="noopener noreferrer"
              className="text-accent border-accent/30 border-b hover:underline"
              data-testid="privacy-link"
            >
              {t("auth.register.consent.privacyLink")}
            </Link>
            {t("auth.register.consent.suffix")}
          </Checkbox>
          {errors.consent && (
            <p className="text-destructive mt-1.5 text-xs font-medium" data-testid="consent-error">
              {errors.consent}
            </p>
          )}
        </div>

        <div className="mt-2">
          <Button
            type="submit"
            size="cta"
            className="w-full"
            disabled={registerMutation.isPending || !acceptedTerms}
            data-testid="submit-button"
          >
            {registerMutation.isPending
              ? t("auth.register.submitting")
              : t("auth.register.submitButton")}
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

      {/* Google sign-up has no checkbox — this small-print carries the ToS/privacy consent. */}
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
