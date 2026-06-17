import { Eye, EyeOff } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate, useSearchParams } from "react-router";
import { toast } from "sonner";

import { AltLink, AuthCard, Field } from "@/features/auth/components/AuthCard";
import { FetchError } from "@/shared/api/axiosClient";
import { Button } from "@/shared/components/ui/button";
import { Input } from "@/shared/components/ui/input";
import { useResetPasswordMutation } from "@/shared/hooks/mutations/useAuth";

interface FieldErrors {
  password?: string;
  confirm?: string;
}

export function ResetPasswordPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const resetMutation = useResetPasswordMutation();

  const token = searchParams.get("token") ?? "";

  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [errors, setErrors] = useState<FieldErrors>({});
  // Flipped when the server rejects the token (invalid/expired/used), or when
  // there is no token in the URL at all.
  const [tokenInvalid, setTokenInvalid] = useState(!token);

  function validatePassword(value: string): string | undefined {
    if (!value) return t("auth.resetPassword.errors.passwordRequired");
    if (value.length < 8) return t("auth.resetPassword.errors.passwordTooShort");
    if (value.length > 72) return t("auth.resetPassword.errors.passwordTooLong");
    return undefined;
  }

  function validateConfirm(value: string): string | undefined {
    if (value !== password) return t("auth.resetPassword.errors.passwordsMismatch");
    return undefined;
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();

    const passwordError = validatePassword(password);
    const confirmError = validateConfirm(confirm);
    if (passwordError || confirmError) {
      setErrors({ password: passwordError, confirm: confirmError });
      return;
    }
    setErrors({});

    try {
      await resetMutation.mutateAsync({ token, password });
      toast.success(t("auth.resetPassword.success"));
      navigate("/login");
    } catch (err) {
      if (err instanceof FetchError) {
        if (err.code === "INVALID_RESET_TOKEN") {
          setTokenInvalid(true);
        } else if (err.code === "PASSWORD_TOO_SHORT") {
          setErrors({ password: t("auth.resetPassword.errors.passwordTooShort") });
        } else if (err.code === "PASSWORD_TOO_LONG") {
          setErrors({ password: t("auth.resetPassword.errors.passwordTooLong") });
        } else {
          toast.error(t("auth.resetPassword.errors.resetFailed"));
        }
      } else {
        toast.error(t("auth.resetPassword.errors.resetFailed"));
      }
    }
  }

  const backToLogin = (
    <AltLink
      prompt={t("auth.resetPassword.altPrompt")}
      cta={t("auth.resetPassword.altCta")}
      to="/login"
      testId="login-link"
    />
  );

  if (tokenInvalid) {
    return (
      <AuthCard
        eyebrow={t("auth.resetPassword.eyebrow")}
        title={t("auth.resetPassword.invalidTitle")}
        subtitle={t("auth.resetPassword.invalidBody")}
        footer={backToLogin}
      >
        <Link
          to="/forgot-password"
          className="text-accent border-accent/30 inline-block border-b pb-px text-[13.5px] font-semibold hover:underline"
          data-testid="request-new-link"
        >
          {t("auth.resetPassword.requestNewLink")}
        </Link>
      </AuthCard>
    );
  }

  return (
    <AuthCard
      eyebrow={t("auth.resetPassword.eyebrow")}
      title={t("auth.resetPassword.title")}
      subtitle={t("auth.resetPassword.subtitle")}
      footer={backToLogin}
    >
      <h1 data-testid="reset-title" className="sr-only">
        {t("auth.resetPassword.title")}
      </h1>

      <form onSubmit={handleSubmit} className="flex flex-col gap-4" data-testid="reset-form">
        <Field
          label={t("auth.resetPassword.passwordLabel")}
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
              placeholder={t("auth.resetPassword.passwordPlaceholder")}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              onBlur={() =>
                setErrors((prev) => ({ ...prev, password: validatePassword(password) }))
              }
              aria-invalid={!!errors.password}
              autoFocus
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

        <Field
          label={t("auth.resetPassword.confirmLabel")}
          htmlFor="confirm"
          error={errors.confirm}
          errorTestId="confirm-error"
        >
          <Input
            id="confirm"
            type={showPassword ? "text" : "password"}
            className="h-10.5"
            placeholder={t("auth.resetPassword.confirmPlaceholder")}
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            onBlur={() => setErrors((prev) => ({ ...prev, confirm: validateConfirm(confirm) }))}
            aria-invalid={!!errors.confirm}
            data-testid="confirm-input"
          />
        </Field>

        <div className="mt-1.5">
          <Button
            type="submit"
            size="cta"
            className="w-full"
            disabled={resetMutation.isPending || !password || !confirm}
            data-testid="submit-button"
          >
            {resetMutation.isPending
              ? t("auth.resetPassword.submitting")
              : t("auth.resetPassword.submitButton")}
          </Button>
        </div>
      </form>
    </AuthCard>
  );
}
