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
