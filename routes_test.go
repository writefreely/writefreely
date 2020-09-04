package writefreely

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func TestCacheControlForStaticFiles(t *testing.T) {
	app := NewApp("testdata/config.ini")
	if err := app.LoadConfig(); err != nil {
		t.Fatalf("Could not create an app; %v", err)
	}
	router := mux.NewRouter()
	app.InitStaticRoutes(router)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/style.css", nil)
	router.ServeHTTP(rec, req)
	if code := rec.Result().StatusCode; code != http.StatusOK {
		t.Fatalf("Could not get /style.css, got HTTP status %d", code)
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
