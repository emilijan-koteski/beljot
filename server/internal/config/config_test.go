package config

import "testing"

func TestStripWhitespace(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"gmail app password with spaces", "ytbk awjy liwm pyzm", "ytbkawjyliwmpyzm"},
		{"already clean", "abcd1234", "abcd1234"},
		{"tabs and newlines", "ab\tcd\nef ", "abcdef"},
		{"empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripWhitespace(tc.in); got != tc.want {
				t.Errorf("stripWhitespace(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSMTPConfigured(t *testing.T) {
	tests := []struct {
		name                 string
		host, user, password string
		want                 bool
	}{
		{"all set", "smtp.gmail.com", "u@x.com", "pw", true},
		{"missing password", "smtp.gmail.com", "u@x.com", "", false},
		{"missing host", "", "u@x.com", "pw", false},
		{"missing username", "smtp.gmail.com", "", "pw", false},
		{"all empty", "", "", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{SMTPHost: tc.host, SMTPUsername: tc.user, SMTPPassword: tc.password}
			if got := c.SMTPConfigured(); got != tc.want {
				t.Errorf("SMTPConfigured() = %v, want %v", got, tc.want)
			}
		})
	}
}
