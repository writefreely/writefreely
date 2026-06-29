/*
 * Copyright © 2020-2021 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package spam

import (
	"net/http/httptest"
	"testing"
)

func TestCleanEmail(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain email unchanged (sans dots)",
			input:    "user@example.com",
			expected: "user@example.com",
		},
		{
			name:     "strips plus alias",
			input:    "user+newsletter@example.com",
			expected: "user@example.com",
		},
		{
			name:     "strips dots in local part",
			input:    "us.er@example.com",
			expected: "user@example.com",
		},
		{
			name:     "strips dots and plus alias together",
			input:    "u.s.e.r+tag@example.com",
			expected: "user@example.com",
		},
		{
			name:     "uppercased is lowercased",
			input:    "User@Example.COM",
			expected: "user@example.com",
		},
		{
			name:     "missing @ returns empty",
			input:    "notanemail",
			expected: "",
		},
		{
			name:     "empty string returns empty",
			input:    "",
			expected: "",
		},
		{
			name:     "domain with dots is preserved",
			input:    "user@mail.example.com",
			expected: "user@mail.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanEmail(tt.input)
			if got != tt.expected {
				t.Errorf("CleanEmail(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestHoneypotFieldName(t *testing.T) {
	// Reset the package-level field so tests are independent.
	honeypotField = ""

	name := HoneypotFieldName()
	if name == "" {
		t.Fatal("HoneypotFieldName() returned empty string")
	}
	// Should be a fixed-length base-62 string.
	if len(name) != 39 {
		t.Errorf("HoneypotFieldName() length = %d, want 39", len(name))
	}
	// Subsequent calls must return the same value (singleton behaviour).
	if got := HoneypotFieldName(); got != name {
		t.Errorf("HoneypotFieldName() returned different value on second call: %q vs %q", got, name)
	}
}

func TestGetIP(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{
			name:     "single IP",
			header:   "1.2.3.4",
			expected: "1.2.3.4",
		},
		{
			name:     "multiple IPs returns first",
			header:   "1.2.3.4, 5.6.7.8, 9.10.11.12",
			expected: "1.2.3.4",
		},
		{
			name:     "trims whitespace from first IP",
			header:   "  10.0.0.1  , 192.168.1.1",
			expected: "10.0.0.1",
		},
		{
			name:     "missing header returns empty",
			header:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set("X-Forwarded-For", tt.header)
			}
			got := GetIP(req)
			if got != tt.expected {
				t.Errorf("GetIP() = %q, want %q (X-Forwarded-For: %q)", got, tt.expected, tt.header)
			}
		})
	}
}
