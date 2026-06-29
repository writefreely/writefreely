package writefreely

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/writeas/web-core/activitystreams"
)

var actorTestTable = []struct {
	Name string
	Resp []byte
}{
	{
		"Context as a string",
		[]byte(`{"@context":"https://www.w3.org/ns/activitystreams"}`),
	},
	{
		"Context as a list",
		[]byte(`{"@context":["one string", "two strings"]}`),
	},
}

func TestUnmarshalActor(t *testing.T) {
	for _, tc := range actorTestTable {
		actor := activitystreams.Person{}
		err := unmarshalActor(tc.Resp, &actor)
		if err != nil {
			t.Errorf("%s failed with error %s", tc.Name, err)
		}
	}
}

func TestParsePostIDFromURLDraftPathWithSubdirectory(t *testing.T) {
	app := NewApp("testdata/config.ini")
	if err := app.LoadConfig(); err != nil {
		t.Fatalf("Could not create an app; %v", err)
	}
	app.cfg.App.Subdirectory = "/blog"

	u, err := url.Parse("https://example.com/blog/api/posts/abc123")
	if err != nil {
		t.Fatalf("could not parse URL: %v", err)
	}

	postID, err := parsePostIDFromURL(app, u)
	if err != nil {
		t.Fatalf("expected parse to succeed, got error: %v", err)
	}
	if postID != "abc123" {
		t.Fatalf("expected postID abc123, got %q", postID)
	}
}

func TestParsePostIDFromURLDraftPathAtRoot(t *testing.T) {
	app := NewApp("testdata/config.ini")
	if err := app.LoadConfig(); err != nil {
		t.Fatalf("Could not create an app; %v", err)
	}
	app.cfg.App.Subdirectory = ""

	u, err := url.Parse("https://example.com/api/posts/xyz789")
	if err != nil {
		t.Fatalf("could not parse URL: %v", err)
	}

	postID, err := parsePostIDFromURL(app, u)
	if err != nil {
		t.Fatalf("expected parse to succeed, got error: %v", err)
	}
	if postID != "xyz789" {
		t.Fatalf("expected postID xyz789, got %q", postID)
	}
}

func TestHostMetaWebfingerTemplateUsesAbsoluteURL(t *testing.T) {
	app := NewApp("testdata/config.ini")
	if err := app.LoadConfig(); err != nil {
		t.Fatalf("Could not create an app; %v", err)
	}
	app.cfg.App.Host = "https://example.com"
	app.cfg.App.Subdirectory = "/blog"

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/blog/.well-known/host-meta", nil)
	if err := handleViewHostMeta(app, rec, req); err != nil {
		t.Fatalf("host-meta handler failed: %v", err)
	}

	body := rec.Body.String()
	expected := "https://example.com/blog/.well-known/webfinger?resource={uri}"
	if !strings.Contains(body, expected) {
		t.Fatalf("expected host-meta template URL %q in response body, got %q", expected, body)
	}
}

func TestNodeInfoConfigBaseURLUsesAbsoluteHost(t *testing.T) {
	app := NewApp("testdata/config.ini")
	if err := app.LoadConfig(); err != nil {
		t.Fatalf("Could not create an app; %v", err)
	}
	app.cfg.App.Host = "https://example.com"
	app.cfg.App.Subdirectory = "/blog"
	app.cfg.App.SingleUser = false

	nc := nodeInfoConfig(nil, app.cfg)
	if nc.BaseURL != "https://example.com/blog" {
		t.Fatalf("expected base URL with subdirectory, got %q", nc.BaseURL)
	}
}
