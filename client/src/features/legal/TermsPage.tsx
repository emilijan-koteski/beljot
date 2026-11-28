import { useTranslation } from "react-i18next";

import { LegalPage } from "@/features/legal/LegalPage";

/** The Terms of Service, in Macedonian for mk readers and in English for everyone else. */
export function TermsPage() {
  const { t } = useTranslation();
  return <LegalPage title={t("legal.terms.title")} doc="terms" testIdPrefix="terms" />;
}
