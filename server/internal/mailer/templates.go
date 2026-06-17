package mailer

import (
	"fmt"
	"html"
)

// passwordResetText holds the localized copy for the reset email. Kept as plain
// strings (not Go templates) so the wording is trivial to edit later without
// touching the send path or the HTML assembly.
type passwordResetText struct {
	subject  string
	greeting string
	intro    string
	button   string
	fallback string // leads into the raw-link line for clients that strip the button
	expiry   string
	ignore   string
}

// passwordResetTexts is keyed by the short language code. en is the fallback for
// any unknown/empty language. mk is intentionally all-Cyrillic; the "Beljot"
// brand stays Latin to match the rest of the product copy.
var passwordResetTexts = map[string]passwordResetText{
	"en": {
		subject:  "Reset your Beljot password",
		greeting: "Hello,",
		intro:    "We received a request to reset your Beljot password. Click the button below to set a new one.",
		button:   "Reset password",
		fallback: "Or paste this link into your browser:",
		expiry:   "This link expires in 1 hour.",
		ignore:   "If you didn't request a password reset, you can safely ignore this email.",
	},
	"sr": {
		subject:  "Resetuj svoju Beljot lozinku",
		greeting: "Zdravo,",
		intro:    "Primili smo zahtev za reset tvoje Beljot lozinke. Klikni na dugme ispod da postaviš novu.",
		button:   "Resetuj lozinku",
		fallback: "Ili nalepi ovaj link u svoj pregledač:",
		expiry:   "Ovaj link ističe za 1 sat.",
		ignore:   "Ako nisi tražio reset lozinke, slobodno ignoriši ovaj imejl.",
	},
	"mk": {
		subject:  "Ресетирај ја твојата Beljot лозинка",
		greeting: "Здраво,",
		intro:    "Добивме барање за ресетирање на твојата Beljot лозинка. Кликни на копчето подолу за да поставиш нова.",
		button:   "Ресетирај лозинка",
		fallback: "Или залепи го овој линк во прелистувачот:",
		expiry:   "Овој линк истекува за 1 час.",
		ignore:   "Ако не си барал ресетирање на лозинката, слободно игнорирај ја оваа порака.",
	},
	"hr": {
		subject:  "Resetiraj svoju Beljot lozinku",
		greeting: "Pozdrav,",
		intro:    "Zaprimili smo zahtjev za resetiranje tvoje Beljot lozinke. Klikni na gumb ispod kako bi postavio novu.",
		button:   "Resetiraj lozinku",
		fallback: "Ili zalijepi ovaj link u svoj preglednik:",
		expiry:   "Ovaj link istječe za 1 sat.",
		ignore:   "Ako nisi zatražio resetiranje lozinke, slobodno zanemari ovaj e-mail.",
	},
}

func passwordResetTextFor(lang string) passwordResetText {
	if t, ok := passwordResetTexts[lang]; ok {
		return t
	}
	return passwordResetTexts["en"]
}

// renderPasswordReset returns the localized subject and a deliberately minimal
// HTML body. The link is HTML-escaped and appears both as a button and as a
// raw, copy-pasteable fallback line.
func renderPasswordReset(lang, resetLink string) (subject, body string) {
	t := passwordResetTextFor(lang)
	safeLink := html.EscapeString(resetLink)
	safeLang := html.EscapeString(lang)
	if _, ok := passwordResetTexts[lang]; !ok {
		safeLang = "en"
	}

	body = fmt.Sprintf(`<!DOCTYPE html>
<html lang="%s">
<body style="font-family: Arial, sans-serif; color: #1a1a1a; line-height: 1.5;">
<p>%s</p>
<p>%s</p>
<p><a href="%s" style="display:inline-block;padding:10px 18px;background:#2f6b3c;color:#ffffff;text-decoration:none;border-radius:6px;">%s</a></p>
<p>%s<br><a href="%s">%s</a></p>
<p style="color:#888888;font-size:12px;">%s<br>%s</p>
</body>
</html>`,
		safeLang,
		t.greeting,
		t.intro,
		safeLink, t.button,
		t.fallback, safeLink, safeLink,
		t.expiry, t.ignore,
	)

	return t.subject, body
}
