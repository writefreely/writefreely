package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempConfig(t *testing.T, appSection string) string {
	t.Helper()

	dir := t.TempDir()
	p := filepath.Join(dir, "config.ini")
	contents := "[app]\n" + appSection + "\n"
	if err := os.WriteFile(p, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return p
}

func TestLoadMovesHostPathToSubdirectory(t *testing.T) {
	f := writeTempConfig(t, "host = https://example.com/blog")

	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.App.Host != "https://example.com" {
		t.Fatalf("expected host without path; got %q", cfg.App.Host)
	}
	if got := cfg.App.SubdirectoryPath(); got != "/blog" {
		t.Fatalf("expected subdirectory /blog; got %q", got)
	}
}

func TestLoadStripsHostPathWhenMatchingSubdirectory(t *testing.T) {
	f := writeTempConfig(t, "host = https://example.com/blog\nsubdirectory = /blog")

	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.App.Host != "https://example.com" {
		t.Fatalf("expected host without path; got %q", cfg.App.Host)
	}
	if got := cfg.App.SubdirectoryPath(); got != "/blog" {
		t.Fatalf("expected subdirectory /blog; got %q", got)
	}
}

func TestLoadPrefersExplicitSubdirectoryOverHostPath(t *testing.T) {
	f := writeTempConfig(t, "host = https://example.com/blog\nsubdirectory = /site")

	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.App.Host != "https://example.com" {
		t.Fatalf("expected host without path; got %q", cfg.App.Host)
	}
	if got := cfg.App.SubdirectoryPath(); got != "/site" {
		t.Fatalf("expected explicit subdirectory /site; got %q", got)
	}
}

func TestNew_Defaults(t *testing.T) {
	cfg := New()
	if cfg.Server.Port != 8080 {
		t.Errorf("default port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.App.Host != "http://localhost:8080" {
		t.Errorf("default host = %q, want http://localhost:8080", cfg.App.Host)
	}
	if !cfg.App.SingleUser {
		t.Error("expected SingleUser to be true by default")
	}
	if cfg.Database.Type != "mysql" {
		t.Errorf("default database type = %q, want mysql", cfg.Database.Type)
	}
}

func TestUseMySQL(t *testing.T) {
	cfg := &Config{}
	cfg.UseMySQL(true)
	if cfg.Database.Type != "mysql" {
		t.Errorf("type = %q, want mysql", cfg.Database.Type)
	}
	if cfg.Database.Host != "localhost" {
		t.Errorf("host = %q, want localhost", cfg.Database.Host)
	}
	if cfg.Database.Port != 3306 {
		t.Errorf("port = %d, want 3306", cfg.Database.Port)
	}

	// fresh=false should not reset host/port
	cfg.Database.Host = "db.example.com"
	cfg.UseMySQL(false)
	if cfg.Database.Host != "db.example.com" {
		t.Errorf("host changed on fresh=false, got %q", cfg.Database.Host)
	}
}

func TestUseSQLite(t *testing.T) {
	cfg := &Config{}
	cfg.UseSQLite(true)
	if cfg.Database.Type != "sqlite3" {
		t.Errorf("type = %q, want sqlite3", cfg.Database.Type)
	}
	if cfg.Database.FileName != "writefreely.db" {
		t.Errorf("filename = %q, want writefreely.db", cfg.Database.FileName)
	}

	cfg.Database.FileName = "custom.db"
	cfg.UseSQLite(false)
	if cfg.Database.FileName != "custom.db" {
		t.Errorf("filename changed on fresh=false, got %q", cfg.Database.FileName)
	}
}

func TestIsSecureStandalone(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want bool
	}{
		{
			name: "port 443 with cert and key",
			cfg:  Config{Server: ServerCfg{Port: 443, TLSCertPath: "/cert.pem", TLSKeyPath: "/key.pem"}},
			want: true,
		},
		{
			name: "port 443 missing cert",
			cfg:  Config{Server: ServerCfg{Port: 443, TLSKeyPath: "/key.pem"}},
			want: false,
		},
		{
			name: "port 443 missing key",
			cfg:  Config{Server: ServerCfg{Port: 443, TLSCertPath: "/cert.pem"}},
			want: false,
		},
		{
			name: "port 8080 with cert and key",
			cfg:  Config{Server: ServerCfg{Port: 8080, TLSCertPath: "/cert.pem", TLSKeyPath: "/key.pem"}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.IsSecureStandalone(); got != tt.want {
				t.Errorf("IsSecureStandalone() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAbsoluteHost(t *testing.T) {
	tests := []struct {
		name   string
		host   string
		subdir string
		want   string
	}{
		{
			name: "no subdir",
			host: "https://example.com",
			want: "https://example.com",
		},
		{
			name:   "with subdir appended",
			host:   "https://example.com",
			subdir: "/blog",
			want:   "https://example.com/blog",
		},
		{
			name:   "host already includes subdir",
			host:   "https://example.com/blog",
			subdir: "/blog",
			want:   "https://example.com/blog",
		},
		{
			name: "empty host",
			host: "",
			want: "",
		},
		{
			name: "trailing slash stripped",
			host: "https://example.com/",
			want: "https://example.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := AppCfg{Host: tt.host, Subdirectory: tt.subdir}
			if got := cfg.AbsoluteHost(); got != tt.want {
				t.Errorf("AbsoluteHost() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAbsoluteURL(t *testing.T) {
	tests := []struct {
		name   string
		host   string
		subdir string
		path   string
		want   string
	}{
		{
			name: "simple path",
			host: "https://example.com",
			path: "/about",
			want: "https://example.com/about",
		},
		{
			name:   "with subdir prefix",
			host:   "https://example.com",
			subdir: "/blog",
			path:   "/about",
			want:   "https://example.com/blog/about",
		},
		{
			name: "empty host falls back to PrefixPath",
			host: "",
			path: "/about",
			want: "/about",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := AppCfg{Host: tt.host, Subdirectory: tt.subdir}
			if got := cfg.AbsoluteURL(tt.path); got != tt.want {
				t.Errorf("AbsoluteURL(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestStripSubdirectory(t *testing.T) {
	tests := []struct {
		name   string
		subdir string
		path   string
		want   string
	}{
		{
			name: "no subdir, non-empty path",
			path: "/about",
			want: "/about",
		},
		{
			name: "no subdir, empty path",
			path: "",
			want: "/",
		},
		{
			name:   "path equals subdir",
			subdir: "/blog",
			path:   "/blog",
			want:   "/",
		},
		{
			name:   "path under subdir",
			subdir: "/blog",
			path:   "/blog/about",
			want:   "/about",
		},
		{
			name:   "path unrelated to subdir",
			subdir: "/blog",
			path:   "/other",
			want:   "/other",
		},
		{
			name:   "empty path with subdir",
			subdir: "/blog",
			path:   "",
			want:   "/",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := AppCfg{Subdirectory: tt.subdir}
			if got := cfg.StripSubdirectory(tt.path); got != tt.want {
				t.Errorf("StripSubdirectory(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestPrefixPath(t *testing.T) {
	tests := []struct {
		name   string
		subdir string
		path   string
		want   string
	}{
		{
			name: "no subdir returns path as-is",
			path: "/about",
			want: "/about",
		},
		{
			name:   "path gets subdir prepended",
			subdir: "/blog",
			path:   "/about",
			want:   "/blog/about",
		},
		{
			name:   "root path with subdir",
			subdir: "/blog",
			path:   "/",
			want:   "/blog/",
		},
		{
			name:   "path already starts with subdir",
			subdir: "/blog",
			path:   "/blog/about",
			want:   "/blog/about",
		},
		{
			name:   "absolute URL is not prefixed",
			subdir: "/blog",
			path:   "https://example.com/foo",
			want:   "https://example.com/foo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := AppCfg{Subdirectory: tt.subdir}
			if got := cfg.PrefixPath(tt.path); got != tt.want {
				t.Errorf("PrefixPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestEmailCfg_Enabled(t *testing.T) {
	tests := []struct {
		name string
		cfg  EmailCfg
		want bool
	}{
		{
			name: "mailgun domain and key",
			cfg:  EmailCfg{Domain: "mg.example.com", MailgunPrivate: "key-abc"},
			want: true,
		},
		{
			name: "SMTP credentials complete",
			cfg:  EmailCfg{Username: "u", Password: "p", Host: "smtp.example.com", Port: 587},
			want: true,
		},
		{
			name: "all empty",
			cfg:  EmailCfg{},
			want: false,
		},
		{
			name: "SMTP missing port",
			cfg:  EmailCfg{Username: "u", Password: "p", Host: "smtp.example.com"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.Enabled(); got != tt.want {
				t.Errorf("Enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSignupPath(t *testing.T) {
	tests := []struct {
		name string
		cfg  AppCfg
		want string
	}{
		{
			name: "closed registration returns empty",
			cfg:  AppCfg{OpenRegistration: false},
			want: "",
		},
		{
			name: "open registration no special flags returns /",
			cfg:  AppCfg{OpenRegistration: true},
			want: "/",
		},
		{
			name: "chorus mode returns /signup",
			cfg:  AppCfg{OpenRegistration: true, Chorus: true},
			want: "/signup",
		},
		{
			name: "private mode returns /signup",
			cfg:  AppCfg{OpenRegistration: true, Private: true},
			want: "/signup",
		},
		{
			name: "non-root landing returns /signup",
			cfg:  AppCfg{OpenRegistration: true, Landing: "start"},
			want: "/signup",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.SignupPath(); got != tt.want {
				t.Errorf("SignupPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLandingPath(t *testing.T) {
	tests := []struct {
		name    string
		landing string
		want    string
	}{
		{
			name:    "empty landing defaults to /",
			landing: "",
			want:    "/",
		},
		{
			name:    "slash prefix preserved",
			landing: "/start",
			want:    "/start",
		},
		{
			name:    "no leading slash gets one added",
			landing: "start",
			want:    "/start",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := AppCfg{Landing: tt.landing}
			if got := cfg.LandingPath(); got != tt.want {
				t.Errorf("LandingPath() = %q, want %q", got, tt.want)
			}
		})
	}
}
