package writefreely

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"testing"

	"github.com/writefreely/writefreely/config"
)

// The existing helper uses a disposable SQLite database and skips without
// -tags sqlite. No production database or remote federation server is used.
func TestBlueNoteEmptyActivityPages(t *testing.T) {
	app, router := newTemplateTestApp(t, func(cfg *config.Config) { cfg.App.SingleUser = false })
	_, coll, _ := createTemplateTestUser(t, app, "blue0a6m5c")
	for _, followerCount := range []int{0, 1} {
		if followerCount > 0 {
			res, err := app.db.Exec("INSERT INTO remoteusers (actor_id, inbox, shared_inbox) VALUES (?, ?, ?)",
				"https://remote.example/users/test", "https://remote.example/inbox", "")
			if err != nil {
				t.Fatal(err)
			}
			id, err := res.LastInsertId()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := app.db.Exec("INSERT INTO remotefollows (collection_id, remote_user_id) VALUES (?, ?)", coll.ID, id); err != nil {
				t.Fatal(err)
			}
		}
		for _, kind := range []string{"followers", "following"} {
			for _, page := range []int{1, 2} {
				t.Run(fmt.Sprintf("%s/followers=%d/page=%d", kind, followerCount, page), func(t *testing.T) {
					path := fmt.Sprintf("/api/collections/%s/%s?page=%d", coll.Alias, kind, page)
					req := httptest.NewRequest(http.MethodGet, path, nil)
					req.Header.Set("Accept", "application/activity+json")
					rec := httptest.NewRecorder()
					router.ServeHTTP(rec, req)
					if rec.Code != http.StatusOK {
						t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
					}
					var body map[string]json.RawMessage
					if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
						t.Fatal(err)
					}
					if string(body["type"]) != `"OrderedCollectionPage"` {
						t.Fatalf("unexpected page: %s", rec.Body.String())
					}
					// web-core marks orderedItems omitempty, so an empty slice
					// is omitted from the current wire format.
					var items []json.RawMessage
					if raw, exists := body["orderedItems"]; exists {
						if err := json.Unmarshal(raw, &items); err != nil {
							t.Fatal(err)
						}
					}
					if len(items) != 0 {
						t.Errorf("expected empty page, got %s", body["orderedItems"])
					}
					wantTotal := "0"
					if kind == "followers" {
						wantTotal = fmt.Sprint(followerCount)
					}
					if string(body["totalItems"]) != wantTotal {
						t.Errorf("totalItems = %s, want %s", body["totalItems"], wantTotal)
					}
					if next, exists := body["next"]; exists {
						t.Errorf("next must be omitted, got %s", next)
					}
				})
			}
		}
	}
}
