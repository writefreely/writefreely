package writefreely

import (
	"net/url"
	"testing"

	"github.com/writefreely/writefreely/spam"
)

func TestSanitizeSubscriptionFormRemovesSpamFields(t *testing.T) {
	honeypot := spam.HoneypotFieldName()
	form := url.Values{
		"email":         {"reader@example.com"},
		"web":           {"1"},
		honeypot:        {""},
		"fake_password": {"bot-value"},
	}

	clean := sanitizeSubscriptionForm(form)

	if clean.Get("email") != "reader@example.com" {
		t.Fatalf("expected email to be preserved")
	}
	if clean.Get("web") != "1" {
		t.Fatalf("expected web to be preserved")
	}
	if _, ok := clean[honeypot]; ok {
		t.Fatalf("expected honeypot field to be removed")
	}
	if _, ok := clean["fake_password"]; ok {
		t.Fatalf("expected fake_password field to be removed")
	}
	if _, ok := form[honeypot]; !ok {
		t.Fatalf("expected original form honeypot field to remain")
	}
	if _, ok := form["fake_password"]; !ok {
		t.Fatalf("expected original form fake_password field to remain")
	}
}

func TestSubscriptionFormDecodeAfterSanitize(t *testing.T) {
	honeypot := spam.HoneypotFieldName()
	form := url.Values{
		"email":         {"reader@example.com"},
		"web":           {"1"},
		honeypot:        {""},
		"fake_password": {"bot-value"},
	}

	app := &App{}
	app.InitDecoder()

	ss := SubmittedSubscription{}
	if err := app.formDecoder.Decode(&ss, sanitizeSubscriptionForm(form)); err != nil {
		t.Fatalf("expected decode to succeed, got %v", err)
	}
	if ss.Email != "reader@example.com" {
		t.Fatalf("expected decoded email, got %q", ss.Email)
	}
	if !ss.Web {
		t.Fatalf("expected decoded web flag to be true")
	}
}
