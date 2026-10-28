import { useTranslation } from "react-i18next";

import { SurrenderDialog } from "./SurrenderDialog";

interface SurrenderPromptProps {
  onAccept: () => void;
  onDecline: () => void;
}

/**
 * The partner's accept-or-decline vote on a surrender. It carries no timer because the server
 * gives the vote no deadline.
 */
export function SurrenderPrompt({ onAccept, onDecline }: SurrenderPromptProps) {
  const { t } = useTranslation();

  return (
    <SurrenderDialog
      titleId="surrender-prompt-title"
      title={t("match.surrender.prompt.title")}
      body={t("match.surrender.prompt.body")}
      cost={t("match.surrender.prompt.consequence")}
      confirmLabel={t("match.surrender.prompt.accept")}
      cancelLabel={t("match.surrender.prompt.decline")}
      onConfirm={onAccept}
      onCancel={onDecline}
      testIds={{
        overlay: "surrender-prompt",
        confirm: "surrender-prompt-accept",
        cancel: "surrender-prompt-decline",
      }}
    />
  );
}
