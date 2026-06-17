import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { AltLink, AuthCard, Field } from "@/features/auth/components/AuthCard";
import { Button } from "@/shared/components/ui/button";
import { Input } from "@/shared/components/ui/input";
import { useForgotPasswordMutation } from "@/shared/hooks/mutations/useAuth";

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export function ForgotPasswordPage() {
  const { t } = useTranslation();
  const forgotMutation = useForgotPasswordMutation();

  const [email, setEmail] = useState("");
  const [error, setError] = useState<string | undefined>(undefined);
  const [submitted, setSubmitted] = useState(false);

  function validateEmail(value: string): string | undefined {
    if (!value) return t("auth.forgotPassword.errors.emailRequired");
    if (!EMAIL_REGEX.test(value)) return t("auth.forgotPassword.errors.emailInvalid");
    return undefined;
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();

    const emailError = validateEmail(email);
    if (emailError) {
      setError(emailError);
      return;
    }
    setError(undefined);

    try {
      await forgotMutation.mutateAsync({ email });
      // Anti-enumeration: the server returns the same generic success whether or
      // not the account exists, so we always land on the confirmation state.
      setSubmitted(true);
    } catch {
      toast.error(t("auth.forgotPassword.errors.requestFailed"));
    }
  }

  const backToLogin = (
    <AltLink
      prompt={t("auth.forgotPassword.altPrompt")}
      cta={t("auth.forgotPassword.altCta")}
      to="/login"
      testId="login-link"
    />
  );

  if (submitted) {
    return (
      <AuthCard
        eyebrow={t("auth.forgotPassword.eyebrow")}
        title={t("auth.forgotPassword.successTitle")}
        subtitle={t("auth.forgotPassword.successBody")}
        footer={backToLogin}
      >
        <p className="text-ink-dim text-[13.5px] leading-[1.55]" data-testid="forgot-success">
          {t("auth.forgotPassword.successHint")}
        </p>
      </AuthCard>
    );
  }

  return (
    <AuthCard
      eyebrow={t("auth.forgotPassword.eyebrow")}
      title={t("auth.forgotPassword.title")}
      subtitle={t("auth.forgotPassword.subtitle")}
      footer={backToLogin}
    >
      <h1 data-testid="forgot-title" className="sr-only">
        {t("auth.forgotPassword.title")}
      </h1>

      <form onSubmit={handleSubmit} className="flex flex-col gap-4" data-testid="forgot-form">
        <Field
          label={t("auth.forgotPassword.emailLabel")}
          htmlFor="email"
          error={error}
          errorTestId="email-error"
        >
          <Input
            id="email"
            type="email"
            className="h-10.5"
            placeholder={t("auth.forgotPassword.emailPlaceholder")}
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            onBlur={() => setError(validateEmail(email))}
            aria-invalid={!!error}
            autoFocus
            data-testid="email-input"
          />
        </Field>

        <div className="mt-1.5">
          <Button
            type="submit"
            size="cta"
            className="w-full"
            disabled={forgotMutation.isPending || !email}
            data-testid="submit-button"
          >
            {forgotMutation.isPending
              ? t("auth.forgotPassword.submitting")
              : t("auth.forgotPassword.submitButton")}
          </Button>
        </div>
      </form>
    </AuthCard>
  );
}
