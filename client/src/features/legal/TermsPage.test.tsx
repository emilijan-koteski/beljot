import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { LegalDocument, LegalDocumentLanguage } from "@/shared/api/legal";
import { useLegalDocumentQuery } from "@/shared/hooks/queries/useLegalDocument";
import { i18n } from "@/shared/i18n/i18n";

import { TermsPage } from "./TermsPage";

vi.mock("@/shared/hooks/queries/useLegalDocument", () => ({
  useLegalDocumentQuery: vi.fn(),
}));

type QueryResult = ReturnType<typeof useLegalDocumentQuery>;

const TERMS_EN = `# Terms of Service

Last updated: 28 November 2026

Read the [Privacy Policy](/privacy) or write to [emilijankoteski@pm.me](mailto:emilijankoteski@pm.me).
`;

const TERMS_MK = `# Услови за користење

Последно ажурирано: 28 ноември 2026
`;

const SOURCES: Record<LegalDocumentLanguage, string> = { en: TERMS_EN, mk: TERMS_MK };

const mockedQuery = vi.mocked(useLegalDocumentQuery);

function loaded(_doc: LegalDocument, lang: LegalDocumentLanguage): QueryResult {
  return { isPending: false, isError: false, data: SOURCES[lang] } as unknown as QueryResult;
}

function failed(): QueryResult {
  return { isPending: false, isError: true, data: undefined } as unknown as QueryResult;
}

async function renderTermsPage(language: string) {
  await act(async () => {
    await i18n.changeLanguage(language);
  });
  return render(
    <MemoryRouter initialEntries={["/terms"]}>
      <Routes>
        <Route path="/terms" element={<TermsPage />} />
        <Route path="/privacy" element={<div data-testid="privacy-route" />} />
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

describe("TermsPage", () => {
  it("requests and renders the Macedonian terms for the mk UI language", async () => {
    await renderTermsPage("mk");

    expect(mockedQuery).toHaveBeenLastCalledWith("terms", "mk");
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Услови за користење");
    expect(screen.getByTestId("terms-document")).toHaveAttribute("lang", "mk");
  });

  it.each(["en", "sr", "hr"])("renders the English terms for the %s UI language", async (lng) => {
    await renderTermsPage(lng);

    expect(mockedQuery).toHaveBeenLastCalledWith("terms", "en");
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Terms of Service");
    expect(screen.getByTestId("terms-document")).toHaveAttribute("lang", "en");
  });

  it.each([
    ["sr", "Ovaj dokument je dostupan na engleskom i makedonskom jeziku."],
    ["hr", "Ovaj je dokument dostupan na engleskom i makedonskom jeziku."],
  ])("shows the language note for the %s UI language", async (lng, note) => {
    await renderTermsPage(lng);

    expect(screen.getByRole("note")).toHaveTextContent(note);
  });

  it("switches between the Macedonian and English versions without changing the app language", async () => {
    await renderTermsPage("mk");
    const user = userEvent.setup();
    const languageSwitch = screen.getByRole("radiogroup", { name: "Јазик на документот" });

    await user.click(within(languageSwitch).getByRole("radio", { name: "EN" }));
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Terms of Service");
    expect(i18n.language).toBe("mk");

    await user.click(within(languageSwitch).getByRole("radio", { name: "МК" }));
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Услови за користење");
  });

  it("shows the unavailable message in the UI language when the terms fail to load", async () => {
    mockedQuery.mockImplementation(failed);
    await renderTermsPage("mk");

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Документот моментално не е достапен.");
    expect(within(alert).getByRole("link", { name: "emilijankoteski@pm.me" })).toHaveAttribute(
      "href",
      "mailto:emilijankoteski@pm.me",
    );
  });

  it("routes the internal Privacy Policy link through the router", async () => {
    await renderTermsPage("en");
    const user = userEvent.setup();

    const internal = screen.getByRole("link", { name: "Privacy Policy" });
    expect(internal).toHaveAttribute("href", "/privacy");
    expect(internal).not.toHaveAttribute("target");
    await user.click(internal);
    expect(screen.getByTestId("privacy-route")).toBeInTheDocument();
  });

  it("keeps email links as plain mailto links", async () => {
    await renderTermsPage("en");

    const email = screen.getByRole("link", { name: "emilijankoteski@pm.me" });
    expect(email).toHaveAttribute("href", "mailto:emilijankoteski@pm.me");
    expect(email).not.toHaveAttribute("target");
  });
});
