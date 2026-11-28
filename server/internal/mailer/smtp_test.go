package mailer

import (
	"errors"
	"strings"
	"testing"
)

func TestRedactRecipient_MasksAddressAndKeepsChain(t *testing.T) {
	base := errors.New("550 mailbox unavailable, affected recipient(s): player@example.com")
	err := redactRecipient(base, "player@example.com")
	if strings.Contains(err.Error(), "player@example.com") {
		t.Fatalf("error still names the recipient: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "550 mailbox unavailable") {
		t.Errorf("error lost its reason: %q", err.Error())
	}
	if !errors.Is(err, base) {
		t.Error("errors.Is no longer reaches the SMTP error")
	}
}
