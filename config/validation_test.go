package config

import "testing"

func TestValidateDomain(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"http prefix ok", "http://example.com", false},
		{"https prefix ok", "https://example.com", false},
		{"subdomain is ok", "https://subdomain.example.com", false},
		{"https with path ok", "https://example.com/blog", false},
		{"missing scheme", "example.com", true},
		{"ftp scheme rejected", "ftp://example.com", true},
		{"empty string rejected", "", true},
		{"just slashes rejected", "//example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDomain(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDomain(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"minimum valid port", "80", false},
		{"common HTTP port", "8080", false},
		{"HTTPS port", "443", false},
		{"max valid port", "65535", false},
		{"port 79 too low", "79", true},
		{"port 0 too low", "0", true},
		{"negative port", "-1", true},
		{"port above max", "65536", true},
		{"non-numeric", "abc", true},
		{"empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePort(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePort(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateNonEmpty(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"non-empty string ok", "hello", false},
		{"single space ok", " ", false},
		{"empty string errors", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNonEmpty(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateNonEmpty(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateSubdirectory(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty string allowed", "", false},
		{"root slash allowed", "/", false},
		{"simple path ok", "/blog", false},
		{"nested path ok", "/my/blog", false},
		{"full URL rejected", "https://example.com/blog", true},
		{"with query string rejected", "/blog?page=1", true},
		{"with fragment rejected", "/blog#section", true},
		{"with space rejected", "/my blog", true},
		{"path with tab rejected", "/my\tblog", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSubdirectory(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSubdirectory(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
