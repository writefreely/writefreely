/*
 * Copyright © 2021 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import "testing"

func TestApplyBasicMarkdown(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		result string
	}{
		{"empty", "", ""},
		{"empty spaces", "  ", ""},
		{"empty tabs", "\t", ""},
		{"empty newline", "\n", ""},
		{"nums", "123", "123"},
		{"dot", ".", "."},
		{"dash", "-", "-"},
		{"plain", "Hello, World!", "Hello, World!"},
		{"multibyte", "こんにちは", `こんにちは`},
		{"bold", "**안녕하세요**", `<strong>안녕하세요</strong>`},
		{"link", "[WriteFreely](https://writefreely.org)", `<a href="https://writefreely.org" rel="nofollow">WriteFreely</a>`},
		{"date", "12. April", `12. April`},
		{"table", "| Hi | There |", `| Hi | There |`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res := applyBasicMarkdown([]byte(test.in))
			if res != test.result {
				t.Errorf("%s: wanted %s, got %s", test.name, test.result, res)
			}
		})
	}
}

func TestShortPostDescription(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "short content returned as-is",
			content: "Hello, world!",
			want:    "Hello, world!",
		},
		{
			name:    "content exactly at 140 chars not truncated",
			content: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"[:140],
			want:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"[:140],
		},
		{
			name:    "content over 140 chars is truncated with ellipsis",
			content: "This is a very long post that exceeds the maximum description length of one hundred and forty characters, so it should be truncated with an ellipsis at the end.",
			want:    "This is a very long post that exceeds the maximum description length of one hundred and forty characters, so it should be truncated with ...",
		},
		{
			name:    "newlines replaced with spaces",
			content: "Line one\nLine two\nLine three",
			want:    "Line one Line two Line three",
		},
		{
			name:    "leading and trailing whitespace trimmed",
			content: "  trimmed content  ",
			want:    "trimmed content",
		},
		{
			name:    "multibyte runes counted by rune not byte",
			content: "日本語のテキストが百四十文字以内に収まっているかどうかをテストします。これは短いテキストです。",
			want:    "日本語のテキストが百四十文字以内に収まっているかどうかをテストします。これは短いテキストです。",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortPostDescription(tt.content)
			if got != tt.want {
				t.Errorf("shortPostDescription() = %q, want %q", got, tt.want)
			}
		})
	}
}
