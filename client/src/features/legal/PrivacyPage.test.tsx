import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { LegalDocument, LegalDocumentLanguage } from "@/shared/api/legal";
import { useLegalDocumentQuery } from "@/shared/hooks/queries/useLegalDocument";
import { i18n } from "@/shared/i18n/i18n";

import { PrivacyPage } from "./PrivacyPage";

vi.mock("@/shared/hooks/queries/useLegalDocument", () => ({
  useLegalDocumentQuery: vi.fn(),
}));

type QueryResult = ReturnType<typeof useLegalDocumentQuery>;

const PRIVACY_EN = `# Privacy Policy

Last updated: 28 November 2026

See [Google's Privacy Policy](https://policies.google.com/privacy) and our [Terms of Service](/terms).

| Data | Purpose |
| --- | --- |
| Email | Sign-in |
`;

const PRIVACY_MK = `# Политика за приватност

Последно ажурирано: 28 ноември 2026
`;

const SOURCES: Record<LegalDocumentLanguage, string> = { en: PRIVACY_EN, mk: PRIVACY_MK };

const mockedQuery = vi.mocked(useLegalDocumentQuery);

function loaded(_doc: LegalDocument, lang: LegalDocumentLanguage): QueryResult {
  return { isPending: false, isError: false, data: SOURCES[lang] } as unknown as QueryResult;
}

function pending(): QueryResult {
  return { isPending: true, isError: false, data: undefined } as unknown as QueryResult;
}

function failed(): QueryResult {
  return { isPending: false, isError: true, data: undefined } as unknown as QueryResult;
}

async function renderPrivacyPage(language: string) {
  await act(async () => {
    await i18n.changeLanguage(language);
  });
  return render(
    <MemoryRouter initialEntries={["/privacy"]}>
      <Routes>
        <Route path="/privacy" element={<PrivacyPage />} />
        <Route path="/terms" element={<div data-testid="terms-route" />} />
      </Routes>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  mockedQuery.mockReset();
  mockedQuery.mockImplementation(loaded);
});

afterEach(async () => {
  await act(async () => {
    await i18n.changeLanguage("en");
  });
});

describe("PrivacyPage", () => {
  it("requests and renders the Macedonian policy for the mk UI language", async () => {
    await renderPrivacyPage("mk");

    expect(mockedQuery).toHaveBeenLastCalledWith("privacy", "mk");
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Политика за приватност");
    expect(screen.getByTestId("privacy-document")).toHaveAttribute("lang", "mk");
  });

  it.each(["en", "sr", "hr"])("renders the English policy for the %s UI language", async (lng) => {
    await renderPrivacyPage(lng);

    expect(mockedQuery).toHaveBeenLastCalledWith("privacy", "en");
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Privacy Policy");
    expect(screen.getByTestId("privacy-document")).toHaveAttribute("lang", "en");
  });

  it.each([
    ["sr", "Ovaj dokument je dostupan na engleskom i makedonskom jeziku."],
    ["hr", "Ovaj je dokument dostupan na engleskom i makedonskom jeziku."],
  ])("shows the language note for the %s UI language", async (lng, note) => {
    await renderPrivacyPage(lng);

    expect(screen.getByRole("note")).toHaveTextContent(note);
  });

  it.each(["en", "mk"])("hides the language note for the %s UI language", async (lng) => {
    await renderPrivacyPage(lng);

    expect(screen.queryByTestId("legal-language-note")).not.toBeInTheDocument();
  });

  it("switches between the English and Macedonian versions without changing the app language", async () => {
    await renderPrivacyPage("en");
    const user = userEvent.setup();
    const languageSwitch = screen.getByRole("radiogroup", { name: "Document language" });

    await user.click(within(languageSwitch).getByRole("radio", { name: "МК" }));
    expect(mockedQuery).toHaveBeenLastCalledWith("privacy", "mk");
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Политика за приватност");
    expect(i18n.language).toBe("en");

    await user.click(within(languageSwitch).getByRole("radio", { name: "EN" }));
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Privacy Policy");
  });

  it("shows a loading placeholder while the document is fetched", async () => {
    mockedQuery.mockImplementation(pending);
    await renderPrivacyPage("en");

    expect(screen.getByRole("status", { name: "Loading document…" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { level: 1 })).not.toBeInTheDocument();
  });

  it("keeps the language switch working while the document loads", async () => {
    mockedQuery.mockImplementation(pending);
    await renderPrivacyPage("en");
    const user = userEvent.setup();

    await user.click(screen.getByRole("radio", { name: "МК" }));

    expect(mockedQuery).toHaveBeenLastCalledWith("privacy", "mk");
    expect(screen.getByTestId("privacy-document")).toHaveAttribute("lang", "mk");
  });

  it("shows the unavailable message with the contact email when the document fails to load", async () => {
    mockedQuery.mockImplementation(failed);
    await renderPrivacyPage("en");

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("This document is temporarily unavailable.");
    const email = within(alert).getByRole("link", { name: "emilijankoteski@pm.me" });
    expect(email).toHaveAttribute("href", "mailto:emilijankoteski@pm.me");
    expect(screen.getByRole("radiogroup", { name: "Document language" })).toBeInTheDocument();
  });

  it("recovers on switching to a version that loads after the other one failed", async () => {
    mockedQuery.mockImplementation((doc, lang) => (lang === "en" ? failed() : loaded(doc, lang)));
    await renderPrivacyPage("en");
    const user = userEvent.setup();

    expect(screen.getByRole("alert")).toBeInTheDocument();
    await user.click(screen.getByRole("radio", { name: "МК" }));

    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Политика за приватност");
  });

  it("renders the Markdown tables inside their own scroll containers", async () => {
    await renderPrivacyPage("en");

    const scrollers = screen.getAllByTestId("legal-table");
    expect(scrollers).toHaveLength(screen.getAllByRole("table").length);
    for (const scroller of scrollers) {
      expect(within(scroller).getByRole("table")).toBeInTheDocument();
    }
  });

  it("opens external links in a new tab and keeps internal links in the router", async () => {
    await renderPrivacyPage("en");
    const user = userEvent.setup();

    const external = screen.getByRole("link", { name: "Google's Privacy Policy" });
    expect(external).toHaveAttribute("target", "_blank");
    expect(external).toHaveAttribute("rel", "noopener noreferrer");

    const internal = screen.getByRole("link", { name: "Terms of Service" });
    expect(internal).not.toHaveAttribute("target");
    await user.click(internal);
    expect(screen.getByTestId("terms-route")).toBeInTheDocument();
  });

  it("renders as a page without its own back link, leaving navigation to the public shell", async () => {
    await renderPrivacyPage("en");

    expect(screen.getByTestId("privacy-page")).toBeInTheDocument();
    expect(screen.queryByTestId("privacy-back-link")).not.toBeInTheDocument();
  });
});
