package config

import "testing"

func TestFriendlyHost(t *testing.T) {
	tests := []struct {
		name string
		host string
		want string
	}{
		{
			name: "plain http host",
			host: "http://example.com",
			want: "example.com",
		},
		{
			name: "https host",
			host: "https://example.com",
			want: "example.com",
		},
		{
			name: "host with port",
			host: "http://example.com:8080",
			want: "example.com:8080",
		},
		{
			name: "https host with port",
			host: "https://example.com:443",
			want: "example.com:443",
		},
		{
			name: "punycode host decoded to unicode",
			host: "https://xn--nxasmq6b.com",
			want: "βόλοσ.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := AppCfg{Host: tt.host}
			got := cfg.FriendlyHost()
			if got != tt.want {
				t.Errorf("FriendlyHost() = %q, want %q (host: %q)", got, tt.want, tt.host)
			}
		})
	}
}

func TestCanCreateBlogs(t *testing.T) {
	tests := []struct {
		name          string
		maxBlogs      int
		currentlyUsed uint64
		want          bool
	}{
		{
			name:          "unlimited (MaxBlogs=0) always allowed",
			maxBlogs:      0,
			currentlyUsed: 9999,
			want:          true,
		},
		{
			name:          "negative MaxBlogs is unlimited",
			maxBlogs:      -1,
			currentlyUsed: 9999,
			want:          true,
		},
		{
			name:          "under limit",
			maxBlogs:      5,
			currentlyUsed: 4,
			want:          true,
		},
		{
			name:          "at limit",
			maxBlogs:      5,
			currentlyUsed: 5,
			want:          false,
		},
		{
			name:          "over limit",
			maxBlogs:      5,
			currentlyUsed: 10,
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := AppCfg{MaxBlogs: tt.maxBlogs}
			got := cfg.CanCreateBlogs(tt.currentlyUsed)
			if got != tt.want {
				t.Errorf("CanCreateBlogs(%d) = %v, want %v (MaxBlogs: %d)", tt.currentlyUsed, got, tt.want, tt.maxBlogs)
			}
		})
	}
}

func TestOrDefaultString(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		defaultValue string
		want         string
	}{
		{
			name:         "non-empty input returned as-is",
			input:        "hello",
			defaultValue: "world",
			want:         "hello",
		},
		{
			name:         "empty input returns default",
			input:        "",
			defaultValue: "world",
			want:         "world",
		},
		{
			name:         "both empty returns empty default",
			input:        "",
			defaultValue: "",
			want:         "",
		},
		{
			name:         "whitespace is non-empty",
			input:        " ",
			defaultValue: "fallback",
			want:         " ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OrDefaultString(tt.input, tt.defaultValue)
			if got != tt.want {
				t.Errorf("OrDefaultString(%q, %q) = %q, want %q", tt.input, tt.defaultValue, got, tt.want)
			}
		})
	}
}
