import { useTranslation } from "react-i18next";

import { LegalPage } from "@/features/legal/LegalPage";

/** The Privacy Policy, in Macedonian for mk readers and in English for everyone else. */
export function PrivacyPage() {
  const { t } = useTranslation();
  return <LegalPage title={t("legal.privacy.title")} doc="privacy" testIdPrefix="privacy" />;
}
