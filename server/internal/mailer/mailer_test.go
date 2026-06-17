package mailer

import (
	"strings"
	"testing"
)

func TestRenderPasswordReset_LocalizedSubjectAndLink(t *testing.T) {
	cases := map[string]string{
		"en": "Reset your Beljot password",
		"sr": "Resetuj svoju Beljot lozinku",
		"mk": "Ресетирај ја твојата Beljot лозинка",
		"hr": "Resetiraj svoju Beljot lozinku",
	}
	const link = "http://localhost:5173/reset-password?token=abc123"
	for lang, wantSubject := range cases {
		subject, body := renderPasswordReset(lang, link)
		if subject != wantSubject {
			t.Errorf("lang %q: subject = %q, want %q", lang, subject, wantSubject)
		}
		if !strings.Contains(body, link) {
			t.Errorf("lang %q: body is missing the reset link", lang)
		}
		if !strings.Contains(body, `lang="`+lang+`"`) {
			t.Errorf("lang %q: body should declare matching html lang", lang)
		}
	}
}

func TestRenderPasswordReset_UnknownLangFallsBackToEn(t *testing.T) {
	subject, body := renderPasswordReset("de", "http://x/reset-password?token=abc")
	enSubject, _ := renderPasswordReset("en", "http://x/reset-password?token=abc")

	if subject != enSubject {
		t.Errorf("unknown lang subject = %q, want en fallback %q", subject, enSubject)
	}
	if !strings.Contains(body, `lang="en"`) {
		t.Errorf("unknown lang body should declare lang=en, got: %s", body)
	}
}

func TestRenderPasswordReset_EscapesLink(t *testing.T) {
	link := `http://x/reset-password?token=a&foo=bar`
	_, body := renderPasswordReset("en", link)

	if strings.Contains(body, link) {
		t.Error("raw unescaped link should not appear verbatim in the HTML body")
	}
	if !strings.Contains(body, "&amp;foo=bar") {
		t.Errorf("ampersand in link should be HTML-escaped, body: %s", body)
	}
}
