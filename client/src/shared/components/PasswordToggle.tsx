import { Eye, EyeOff } from "lucide-react";
import { useTranslation } from "react-i18next";

type PasswordToggleProps = {
  shown: boolean;
  onToggle: () => void;
  testId?: string;
};

/**
 * Eye button inside a password field that shows or hides the value, labelled in the player's language.
 * The field around it needs `relative` positioning and right padding for the icon.
 */
export function PasswordToggle({ shown, onToggle, testId }: PasswordToggleProps) {
  const { t } = useTranslation();
  return (
    <button
      type="button"
      tabIndex={-1}
      className="text-ink-mute hover:text-ink absolute top-1/2 right-2.5 -translate-y-1/2 p-1.5"
      onClick={onToggle}
      data-testid={testId}
      aria-label={shown ? t("common.hidePassword") : t("common.showPassword")}
    >
      {shown ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
    </button>
  );
}
