package writefreely

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/writefreely/writefreely/config"
)

func TestBlueNoteISO8601UTC(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
	}{
		{"UTC", "2026-09-05T12:34:56Z", "2026-09-05T12:34:56Z"},
		{"positive offset crosses year", "2026-01-01T00:30:45+09:00", "2025-12-31T15:30:45Z"},
		{"negative offset crosses day", "2026-09-05T23:30:45-07:00", "2026-09-06T06:30:45Z"},
		{"fractional seconds", "2026-09-05T12:34:56.123456+05:30", "2026-09-05T07:04:56Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			created, err := time.Parse(time.RFC3339Nano, tc.input)
			if err != nil {
				t.Fatal(err)
			}
			post := &Post{Created: created}
			raw := &RawPost{Created: created, Updated: created}
			for name, serialize := range map[string]func() string{
				"Post.Created8601":    post.Created8601,
				"RawPost.Created8601": raw.Created8601,
				"RawPost.Updated8601": raw.Updated8601,
			} {
				t.Run(name, func(t *testing.T) {
					if got := serialize(); got != tc.want {
						t.Fatalf("got %q, want %q", got, tc.want)
					}
				})
			}
			if post.Created != created || raw.Created != created || raw.Updated != created {
				t.Fatal("serialization changed the stored timestamp")
			}
		})
	}
	if got := (&RawPost{}).Updated8601(); got != "" {
		t.Fatalf("unset Updated8601 = %q, want empty", got)
	}
}

func TestBlueNoteActivityArticleSummary(t *testing.T) {
	cfg := config.New()
	cfg.App.Host = "https://example.com"
	cfg.App.NotesOnly = false
	app := &App{cfg: cfg}
	for _, tc := range []struct {
		name, content, want string
	}{
		{"markdown", "**Hello** & [world](https://example.org).\n\nSecond paragraph.", "Hello & world. [...]"},
		{"HTML and entities", "<strong>日本語</strong> &amp; &#39;quotes&#39;.\n\nSecond paragraph.", "日本語 & 'quotes'. [...]"},
		{"explicit excerpt", "**Before**<!--more-->Hidden\n\nSecond paragraph.", "Before [...]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			post := &PublicPost{
				Post:       &Post{ID: "testpost01", Content: tc.content, Created: time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)},
				Collection: &CollectionObj{Collection: Collection{Alias: "blue0a6m5c", hostName: cfg.App.Host}},
			}
			obj := post.ActivityObject(app)
			data, err := json.Marshal(obj)
			if err != nil {
				t.Fatal(err)
			}
			var wire struct {
				Type    string  `json:"type"`
				Summary *string `json:"summary"`
				Content string  `json:"content"`
				Preview struct {
					Content string `json:"content"`
				} `json:"preview"`
			}
			if err := json.Unmarshal(data, &wire); err != nil {
				t.Fatal(err)
			}
			if wire.Type != "Article" || wire.Summary == nil {
				t.Fatalf("expected Article with summary: %s", data)
			}
			if *wire.Summary != tc.want {
				t.Errorf("summary = %q, want %q", *wire.Summary, tc.want)
			}
			if !strings.Contains(wire.Preview.Content, "<strong>") || !strings.Contains(wire.Content, "<strong>") {
				t.Errorf("body and preview must retain HTML: %s", data)
			}
		})
	}
}
