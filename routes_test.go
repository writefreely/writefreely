package writefreely

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func newSubdirTestRouter(t *testing.T, subdir string) *mux.Router {
	t.Helper()

	app := NewApp("testdata/config.ini")
	if err := app.LoadConfig(); err != nil {
		t.Fatalf("Could not create an app; %v", err)
	}
	app.cfg.App.Subdirectory = subdir

	router := mux.NewRouter()
	inner := mux.NewRouter()
	inner.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	MountSubdirectory(app, router, inner)
	return router
}

func TestCacheControlForStaticFiles(t *testing.T) {
	app := NewApp("testdata/config.ini")
	if err := app.LoadConfig(); err != nil {
		t.Fatalf("Could not create an app; %v", err)
	}
	router := mux.NewRouter()
	app.InitStaticRoutes(router)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/css/write.css", nil)
	router.ServeHTTP(rec, req)
	if code := rec.Result().StatusCode; code != http.StatusOK {
		t.Fatalf("Could not get /css/write.css, got HTTP status %d", code)
	}
	actual := rec.Result().Header.Get("Cache-Control")

	expectedDirectives := []string{
		"public",
		"max-age",
		"immutable",
	}
	for _, expected := range expectedDirectives {
		if !strings.Contains(actual, expected) {
			t.Errorf("Expected Cache-Control header to contain '%s', but was '%s'", expected, actual)
		}
	}
}

func TestMountSubdirectoryRedirectsMissingPrefix(t *testing.T) {
	router := newSubdirTestRouter(t, "/blog")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/hello?x=1", nil)
	router.ServeHTTP(rec, req)

	if code := rec.Result().StatusCode; code != http.StatusTemporaryRedirect {
		t.Fatalf("Expected HTTP status %d, got %d", http.StatusTemporaryRedirect, code)
	}
	if loc := rec.Result().Header.Get("Location"); loc != "/blog/hello?x=1" {
		t.Fatalf("Expected redirect location '/blog/hello?x=1', got '%s'", loc)
	}
}

func TestMountSubdirectoryServesPrefixedPath(t *testing.T) {
	router := newSubdirTestRouter(t, "/blog")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/blog/hello", nil)
	router.ServeHTTP(rec, req)

	if code := rec.Result().StatusCode; code != http.StatusOK {
		t.Fatalf("Expected HTTP status %d, got %d", http.StatusOK, code)
	}
	if body := rec.Body.String(); body != "ok" {
		t.Fatalf("Expected body 'ok', got '%s'", body)
	}
}

func discoverRouteTemplates(t *testing.T) []string {
	t.Helper()

	app := NewApp("testdata/config.ini")
	if err := app.LoadConfig(); err != nil {
		t.Fatalf("Could not create an app; %v", err)
	}
	app.cfg.App.Host = "https://example.com"
	initKeyPaths(app)
	if err := app.LoadKeys(); err != nil {
		t.Fatalf("Could not load app keys; %v", err)
	}

	router := mux.NewRouter()
	InitRoutes(app, router)

	seen := map[string]struct{}{}
	err := router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		tmpl, err := route.GetPathTemplate()
		if err != nil {
			return nil
		}
		seen[tmpl] = struct{}{}
		return nil
	})
	if err != nil {
		t.Fatalf("unable to walk routes: %v", err)
	}

	templates := make([]string, 0, len(seen))
	for tmpl := range seen {
		templates = append(templates, tmpl)
	}
	sort.Strings(templates)
	return templates
}

func sampleTemplateVarValue(inner string) string {
	name := inner
	pattern := ""
	if i := strings.Index(inner, ":"); i > -1 {
		name = inner[:i]
		pattern = inner[i+1:]
	}

	switch {
	case name == "prefix":
		return "@"
	case name == "page":
		return "1"
	case name == "lang":
		return "en"
	case name == "archive":
		return "archive"
	case name == "post" && strings.Contains(pattern, "{10}"):
		return "abcdefghij"
	case name == "post":
		return "samplepost"
	case name == "tag":
		return "testing"
	case name == "handle":
		return "alice"
	case name == "slug":
		return "sample"
	case name == "collection":
		return "sample"
	case name == "alias":
		return "sample"
	default:
		return "sample"
	}
}

func samplePathFromTemplate(tmpl string) string {
	var out strings.Builder
	for i := 0; i < len(tmpl); {
		if tmpl[i] != '{' {
			out.WriteByte(tmpl[i])
			i++
			continue
		}

		j := i + 1
		depth := 1
		for ; j < len(tmpl) && depth > 0; j++ {
			switch tmpl[j] {
			case '{':
				depth++
			case '}':
				depth--
			}
		}
		if depth != 0 {
			out.WriteByte(tmpl[i])
			i++
			continue
		}

		inner := tmpl[i+1 : j-1]
		out.WriteString(sampleTemplateVarValue(inner))
		i = j
	}

	replaced := out.String()
	for strings.Contains(replaced, "//") {
		replaced = strings.ReplaceAll(replaced, "//", "/")
	}
	return replaced
}

func TestInitRoutesDiscoveryFindsEndpoints(t *testing.T) {
	templates := discoverRouteTemplates(t)

	if len(templates) < 40 {
		t.Fatalf("expected at least 40 route templates, got %d", len(templates))
	}

	required := map[string]bool{
		"/":          false,
		"/api/posts": false,
	}
	for _, tmpl := range templates {
		if _, ok := required[tmpl]; ok {
			required[tmpl] = true
		}
	}
	for tmpl, found := range required {
		if !found {
			t.Fatalf("expected discovered route template %q", tmpl)
		}
	}
}

func TestDiscoveredEndpointsMountUnderSubdirectory(t *testing.T) {
	templates := discoverRouteTemplates(t)

	inner := mux.NewRouter()
	for _, tmpl := range templates {
		inner.HandleFunc(tmpl, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
	}

	app := NewApp("testdata/config.ini")
	if err := app.LoadConfig(); err != nil {
		t.Fatalf("Could not create an app; %v", err)
	}
	app.cfg.App.Subdirectory = "/blog"

	router := mux.NewRouter()
	MountSubdirectory(app, router, inner)

	for _, tmpl := range templates {
		path := samplePathFromTemplate(tmpl)

		// Requests without configured subdirectory should redirect under it.
		redir := httptest.NewRecorder()
		redirReq := httptest.NewRequest("GET", path, nil)
		router.ServeHTTP(redir, redirReq)
		if redir.Result().StatusCode != http.StatusTemporaryRedirect {
			t.Fatalf("expected redirect for %q, got %d", path, redir.Result().StatusCode)
		}

		// Requests under configured subdirectory should be served by inner router.
		prefixed := app.cfg.App.PrefixPath(path)
		okRec := httptest.NewRecorder()
		okReq := httptest.NewRequest("GET", prefixed, nil)
		router.ServeHTTP(okRec, okReq)
		if okRec.Result().StatusCode == http.StatusNotFound || okRec.Result().StatusCode >= 500 {
			t.Fatalf("expected mounted route for template %q (sample %q), got %d", tmpl, prefixed, okRec.Result().StatusCode)
		}
	}
}

func TestRoutingModesRootSubdirectoryAndSubdomain(t *testing.T) {
	// Root and subdomain modes both have no subdirectory mount.
	for _, host := range []string{"https://example.com", "https://blog.example.com"} {
		app := NewApp("testdata/config.ini")
		if err := app.LoadConfig(); err != nil {
			t.Fatalf("Could not create an app; %v", err)
		}
		app.cfg.App.Host = host
		app.cfg.App.Subdirectory = ""

		router := mux.NewRouter()
		inner := mux.NewRouter()
		inner.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		MountSubdirectory(app, router, inner)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/hello", nil)
		router.ServeHTTP(rec, req)
		if rec.Result().StatusCode != http.StatusOK {
			t.Fatalf("expected root-style mount for host %q, got %d", host, rec.Result().StatusCode)
		}
	}

	// Subdirectory mode redirects missing prefix and serves prefixed routes.
	router := newSubdirTestRouter(t, "/blog")
	redir := httptest.NewRecorder()
	redirReq := httptest.NewRequest("GET", "/hello", nil)
	router.ServeHTTP(redir, redirReq)
	if redir.Result().StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("expected redirect in subdirectory mode, got %d", redir.Result().StatusCode)
	}
	if loc := redir.Result().Header.Get("Location"); loc != "/blog/hello" {
		t.Fatalf("expected /blog/hello redirect, got %q", loc)
	}

	okRec := httptest.NewRecorder()
	okReq := httptest.NewRequest("GET", "/blog/hello", nil)
	router.ServeHTTP(okRec, okReq)
	if okRec.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected served prefixed route in subdirectory mode, got %d", okRec.Result().StatusCode)
	}
}
