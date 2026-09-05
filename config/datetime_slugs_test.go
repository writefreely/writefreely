package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDatetimeSlugConfiguration(t *testing.T) {
	if c := New(); c.App.DatetimeSlugs || c.App.DatetimeSlugTimezone != "UTC" {
		t.Fatal("incorrect defaults")
	}
	for _, tc := range []struct {
		name, settings, zone string
		enabled, invalid     bool
	}{
		{"missing", "", "UTC", false, false},
		{"empty", "datetime_slugs = true\ndatetime_slug_timezone =", "UTC", true, false},
		{"Tokyo", "datetime_slugs = true\ndatetime_slug_timezone = Asia/Tokyo", "Asia/Tokyo", true, false},
		{"invalid enabled", "datetime_slugs = true\ndatetime_slug_timezone = Invalid/Zone", "", true, true},
		{"local rejected", "datetime_slugs = true\ndatetime_slug_timezone = Local", "", true, true},
		{"invalid disabled", "datetime_slugs = false\ndatetime_slug_timezone = Invalid/Zone", "Invalid/Zone", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.ini")
			if err := os.WriteFile(path, []byte("[app]\nhost = https://example.com\n"+tc.settings+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			if tc.invalid {
				if err == nil || !strings.Contains(err.Error(), "datetime_slug_timezone") {
					t.Fatalf("expected timezone error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.App.DatetimeSlugs != tc.enabled || cfg.App.DatetimeSlugTimezone != tc.zone {
				t.Fatalf("unexpected config: %+v", cfg.App)
			}
			if err := Save(cfg, path); err != nil {
				t.Fatal(err)
			}
			roundtrip, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if roundtrip.App.DatetimeSlugs != tc.enabled || roundtrip.App.DatetimeSlugTimezone != tc.zone {
				t.Fatal("settings lost on round trip")
			}
		})
	}
}

func TestDatetimeSlugTimezone(t *testing.T) {
	for _, tc := range []struct{ zone, created, want string }{
		{"", "2026-09-05T15:30:00Z", "20260905153000"},
		{"UTC", "2026-09-06T00:30:00+09:00", "20260905153000"},
		{"Asia/Tokyo", "2026-09-05T15:30:00Z", "20260906003000"},
		{"America/New_York", "2026-07-01T12:00:00Z", "20260701080000"},
		{"America/New_York", "2026-01-01T12:00:00Z", "20260101070000"},
	} {
		t.Run(tc.zone+tc.created, func(t *testing.T) {
			created, err := time.Parse(time.RFC3339, tc.created)
			if err != nil {
				t.Fatal(err)
			}
			cfg := AppCfg{DatetimeSlugs: true, DatetimeSlugTimezone: tc.zone}
			for _, offset := range []int{-7 * 3600, 9 * 3600} {
				got, err := cfg.DatetimeSlug(created.In(time.FixedZone("source", offset)))
				if err != nil || got != tc.want {
					t.Fatalf("got %q, %v; want %q", got, err, tc.want)
				}
			}
		})
	}
	cfg := AppCfg{DatetimeSlugTimezone: "Invalid/Zone"}
	if got, err := cfg.DatetimeSlug(time.Now()); got != "" || err != nil {
		t.Fatalf("disabled feature: %q %v", got, err)
	}
}
