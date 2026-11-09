/*
 * Copyright © 2026 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package mailer

import (
	netmail "net/mail"
	"testing"

	mail "github.com/xhit/go-simple-mail/v2"
)

func TestFormatAddress(t *testing.T) {
	cases := []struct {
		name, address string
	}{
		{"Le blog", "le-blog@example.com"},
		{"A, B, C et D", "sender@example.net"},
		{"", "noreply@example.com"},
	}
	for _, c := range cases {
		got := FormatAddress(c.name, c.address)
		addr, err := netmail.ParseAddress(got)
		if err != nil {
			t.Fatalf("FormatAddress(%q, %q) = %q, which failed to parse: %v", c.name, c.address, got, err)
		}
		if addr.Address != c.address {
			t.Errorf("FormatAddress(%q, %q) = %q, parsed address = %q, want %q", c.name, c.address, got, addr.Address, c.address)
		}
	}
}

func TestNewMessageDoesNotAddPhantomRecipients(t *testing.T) {
	m := &Mailer{smtp: mail.NewSMTPClient()}
	msg, err := m.NewMessage("Test <from@example.com>", "Subject", "body", "<to@example.com>")
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}
	if len(msg.smtpMsg.recipients) != 1 {
		t.Fatalf("expected 1 recipient, got %d: %+v", len(msg.smtpMsg.recipients), msg.smtpMsg.recipients)
	}
	if msg.smtpMsg.recipients[0].email != "<to@example.com>" {
		t.Errorf("unexpected recipient: %+v", msg.smtpMsg.recipients[0])
	}
}
