export type LegalDocument = "privacy" | "terms";

export type LegalDocumentLanguage = "en" | "mk";

const HTML_BODY_PATTERN = /^\s*<(!doctype|html)/i;

/** Thrown when a legal document is missing, unreadable, or answered by the app shell instead of Markdown. */
export class LegalDocumentUnavailableError extends Error {
  constructor(path: string, reason: string) {
    super(`Legal document ${path} is unavailable: ${reason}`);
    this.name = "LegalDocumentUnavailableError";
  }
}

/** The same-origin path a legal document is served from, outside the API. */
export function legalDocumentPath(doc: LegalDocument, lang: LegalDocumentLanguage): string {
  return `/legal/${doc}.${lang}.md`;
}

/** Fetches a legal document's Markdown, rejecting the SPA fallback page that unknown paths return. */
export async function getLegalDocument(
  doc: LegalDocument,
  lang: LegalDocumentLanguage,
): Promise<string> {
  const path = legalDocumentPath(doc, lang);
  const response = await fetch(path, { headers: { Accept: "text/markdown, text/plain" } });
  if (!response.ok) {
    throw new LegalDocumentUnavailableError(path, `status ${response.status}`);
  }
  const contentType = response.headers.get("content-type") ?? "";
  if (contentType.toLowerCase().includes("text/html")) {
    throw new LegalDocumentUnavailableError(path, "served as HTML");
  }
  const body = await response.text();
  if (HTML_BODY_PATTERN.test(body)) {
    throw new LegalDocumentUnavailableError(path, "body is HTML");
  }
  return body;
}
