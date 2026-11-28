import { useState } from "react";
import { useTranslation } from "react-i18next";

import {
  LEGAL_LANGUAGES,
  type LegalLanguage,
  legalLanguageFor,
  needsLanguageNote,
} from "@/features/legal/legalLanguage";
import { LegalMarkdown } from "@/features/legal/LegalMarkdown";
import type { LegalDocument } from "@/shared/api/legal";
import { Eyebrow } from "@/shared/components/ui/eyebrow";
import { Segmented } from "@/shared/components/ui/segmented";
import { Skeleton } from "@/shared/components/ui/skeleton";
import { useLegalDocumentQuery } from "@/shared/hooks/queries/useLegalDocument";
import { usePageMeta } from "@/shared/hooks/usePageMeta";

const LANGUAGE_LABELS: Record<LegalLanguage, string> = { en: "EN", mk: "МК" };

const CONTACT_EMAIL = "emilijankoteski@pm.me";

const SKELETON_LINE_WIDTHS = ["w-2/3", "w-full", "w-full", "w-5/6", "w-full", "w-3/4"];

type Props = {
  title: string;
  doc: LegalDocument;
  testIdPrefix: string;
};

/** Placeholder lines shown while a legal document loads. */
function LegalDocumentSkeleton() {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      aria-label={t("legal.loading")}
      className="flex flex-col gap-3"
      data-testid="legal-loading"
    >
      {SKELETON_LINE_WIDTHS.map((width, i) => (
        <Skeleton key={i} className={i === 0 ? `h-8 ${width}` : `h-4 ${width}`} />
      ))}
    </div>
  );
}

/** Tells the reader the document could not be loaded and how to reach the operator instead. */
function LegalDocumentUnavailable() {
  const { t } = useTranslation();
  return (
    <div role="alert" className="flex flex-col gap-2" data-testid="legal-unavailable">
      <p className="text-ink m-0 font-semibold">{t("legal.unavailable.title")}</p>
      <p className="text-ink-dim m-0 leading-relaxed">
        {t("legal.unavailable.contact")}{" "}
        <a
          href={`mailto:${CONTACT_EMAIL}`}
          className="text-accent decoration-accent/40 hover:decoration-accent font-medium underline underline-offset-2 [overflow-wrap:anywhere]"
        >
          {CONTACT_EMAIL}
        </a>
      </p>
    </div>
  );
}

/**
 * Full-width public page for a legal document that exists in English and Macedonian. It opens in Macedonian
 * for mk readers and in English for everyone else, a local switch shows the other version, and the text is fetched at runtime.
 */
export function LegalPage({ title, doc, testIdPrefix }: Props) {
  const { t, i18n } = useTranslation();
  usePageMeta(title);
  const uiLanguage = i18n.resolvedLanguage ?? i18n.language;
  const [chosen, setChosen] = useState<LegalLanguage | null>(null);
  const docLanguage = chosen ?? legalLanguageFor(uiLanguage);
  const docQuery = useLegalDocumentQuery(doc, docLanguage);

  return (
    <div
      className="mx-auto flex w-full max-w-[72ch] flex-col gap-4 px-4 pt-5 pb-16 text-[0.9375rem] md:px-6 md:pt-8"
      data-testid={`${testIdPrefix}-page`}
    >
      <div className="flex items-center justify-between gap-3">
        <Eyebrow>{t("legal.eyebrow")}</Eyebrow>
        <Segmented<LegalLanguage>
          value={docLanguage}
          onValueChange={setChosen}
          options={LEGAL_LANGUAGES.map((l) => ({ value: l, label: LANGUAGE_LABELS[l] }))}
          ariaLabel={t("legal.switchLabel")}
          testId="legal-language-switch"
          dense
          className="w-28 shrink-0"
        />
      </div>

      {needsLanguageNote(uiLanguage) && (
        <p
          role="note"
          className="bg-brass-soft text-ink border-brass/40 m-0 rounded-lg border px-4 py-3 text-sm leading-snug"
          data-testid="legal-language-note"
        >
          {t("legal.languageNote")}
        </p>
      )}

      <article lang={docLanguage} className="break-words" data-testid={`${testIdPrefix}-document`}>
        {docQuery.isPending ? (
          <LegalDocumentSkeleton />
        ) : docQuery.isError ? (
          <LegalDocumentUnavailable />
        ) : (
          <LegalMarkdown source={docQuery.data} titleTestId={`${testIdPrefix}-title`} />
        )}
      </article>
    </div>
  );
}
