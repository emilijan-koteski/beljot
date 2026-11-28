import { afterEach, describe, expect, it, vi } from "vitest";

import { getLegalDocument, LegalDocumentUnavailableError } from "@/shared/api/legal";

function stubFetch(body: string, init: ResponseInit) {
  const fetchMock = vi.fn().mockResolvedValue(new Response(body, init));
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("getLegalDocument", () => {
  it("fetches the document from its same-origin path and returns the Markdown", async () => {
    const fetchMock = stubFetch("# Privacy Policy\n", {
      status: 200,
      headers: { "Content-Type": "text/markdown; charset=utf-8" },
    });

    await expect(getLegalDocument("privacy", "mk")).resolves.toBe("# Privacy Policy\n");
    expect(fetchMock).toHaveBeenCalledWith("/legal/privacy.mk.md", expect.any(Object));
  });

  it("rejects a non-OK response", async () => {
    stubFetch("Not Found", { status: 404, headers: { "Content-Type": "text/plain" } });

    await expect(getLegalDocument("terms", "en")).rejects.toBeInstanceOf(
      LegalDocumentUnavailableError,
    );
  });

  it("rejects the SPA fallback served as text/html", async () => {
    stubFetch("<!doctype html><html><body><div id=root></div></body></html>", {
      status: 200,
      headers: { "Content-Type": "text/html" },
    });

    await expect(getLegalDocument("terms", "en")).rejects.toBeInstanceOf(
      LegalDocumentUnavailableError,
    );
  });

  it.each(["<!DOCTYPE html>\n<html lang=en>", "\n  <html><head></head></html>"])(
    "rejects an HTML body even without an HTML content type: %s",
    async (body) => {
      stubFetch(body, { status: 200, headers: { "Content-Type": "application/octet-stream" } });

      await expect(getLegalDocument("privacy", "en")).rejects.toBeInstanceOf(
        LegalDocumentUnavailableError,
      );
    },
  );

  it("rejects when the network request fails", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("Failed to fetch")));

    await expect(getLegalDocument("privacy", "en")).rejects.toThrow("Failed to fetch");
  });
});
