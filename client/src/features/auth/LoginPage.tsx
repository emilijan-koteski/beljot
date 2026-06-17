import { Eye, EyeOff } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "react-router";
import { toast } from "sonner";

import { AltLink, AuthCard, Field } from "@/features/auth/components/AuthCard";
import { FetchError } from "@/shared/api/axiosClient";
import { updatePreferences } from "@/shared/api/profile";
import { Button } from "@/shared/components/ui/button";
import { Input } from "@/shared/components/ui/input";
import { useLoginMutation } from "@/shared/hooks/mutations/useAuth";
import { normalizeLanguage } from "@/shared/i18n/i18n";
import { useAuthStore } from "@/shared/stores/authStore";

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

interface FieldErrors {
  email?: string;
  password?: string;
}

export function LoginPage() {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const loginMutation = useLoginMutation();

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
      // If the visitor picked a different language on the auth page than the
      // one stored on their profile, push the picked language to the server
      // and reconcile the auth store. Mirrors LanguageSelector's
      // optimistic-with-rollback pattern; UI language stays as picked.
      // Normalize i18n.language to the short code so region-tagged values
      // like "en-US" don't trigger a futile PATCH the server would reject.
      const picked = normalizeLanguage(i18n.language);
      if (picked && picked !== res.languagePreference) {
        try {
          await updatePreferences(res.id, { languagePreference: picked });
          const current = useAuthStore.getState().user;
          if (current?.id === res.id) {
            useAuthStore.getState().setUser({ ...current, languagePreference: picked });
          }
        } catch {
          const current = useAuthStore.getState().user;
          if (current?.id === res.id) {
            useAuthStore
              .getState()
              .setUser({ ...current, languagePreference: res.languagePreference });
          }
        }
      }
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
              aria-label={showPassword ? "Hide password" : "Show password"}
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
    </AuthCard>
  );
}
