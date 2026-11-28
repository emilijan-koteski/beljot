import { useQuery } from "@tanstack/react-query";

import {
  getLegalDocument,
  type LegalDocument,
  type LegalDocumentLanguage,
} from "@/shared/api/legal";
import { queryKeys } from "@/shared/api/queryKeys";

const LEGAL_DOCUMENT_STALE_MS = 60 * 60 * 1000;

/** One legal document's Markdown, cached for an hour because the texts rarely change. */
export function useLegalDocumentQuery(doc: LegalDocument, lang: LegalDocumentLanguage) {
  return useQuery({
    queryKey: queryKeys.legal.document(doc, lang),
    queryFn: () => getLegalDocument(doc, lang),
    staleTime: LEGAL_DOCUMENT_STALE_MS,
    retry: 1,
  });
}
