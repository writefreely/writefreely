package writefreely

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

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

func TestBlueNoteClaimDatetimeSlug(t *testing.T) {
	app, _ := newTemplateTestApp(t, func(cfg *config.Config) { cfg.App.SingleUser = false })
	for _, alias := range []string{"blue0a6m5c", "otherblog"} {
		user, coll, direct := createTemplateTestUser(t, app, alias)
		// Current scope: direct creation still uses the title-based slug.
		if direct.Slug.String != "hello-world" {
			t.Fatalf("direct post slug = %q", direct.Slug.String)
		}
		for _, viaClaim := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/claim=%t", alias, viaClaim), func(t *testing.T) {
				title, content, created := "Draft Title", "Draft body", "2001-02-03T04:05:06Z"
				draft, err := app.db.CreatePost(user.ID, 0, &SubmittedPost{Title: &title, Content: &content, Created: &created})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if _, err := app.db.Exec("DELETE FROM posts WHERE id = ?", draft.ID); err != nil {
						t.Error(err)
					}
				})
				requests := []ClaimPostRequest{{AnonymousAuthPost: &AnonymousAuthPost{ID: draft.ID}}}
				target := alias
				if viaClaim {
					target = ""
					requests[0].CollectionAlias = alias
				}
				before := time.Now().Truncate(time.Second)
				results, err := app.db.ClaimPosts(app.cfg, user.ID, target, &requests)
				after := time.Now()
				if err != nil {
					t.Fatal(err)
				}
				if results == nil || len(*results) != 1 {
					t.Fatalf("unexpected claim results: %#v", results)
				}
				result := (*results)[0]
				if result.Code != http.StatusOK || result.Post == nil {
					t.Fatalf("claim failed: %+v", result)
				}
				slug := result.Post.Slug.String
				if alias == "blue0a6m5c" {
					if !regexp.MustCompile(`^\d{14}$`).MatchString(slug) {
						t.Fatalf("expected datetime slug, got %q", slug)
					}
					// The implementation uses processing time in time.Local, not
					// the saved publication date. Accept a second boundary crossing.
					stamp, err := time.ParseInLocation("20060102150405", slug, time.Local)
					if err != nil {
						t.Fatal(err)
					}
					if stamp.Before(before) || stamp.After(after) {
						t.Errorf("slug time %s outside [%s, %s]", stamp, before, after)
					}
				} else if slug != "draft-title" {
					t.Errorf("other blog slug = %q, want draft-title", slug)
				}
				var savedSlug string
				var savedCollection int64
				if err := app.db.QueryRow("SELECT slug, collection_id FROM posts WHERE id = ?", draft.ID).Scan(&savedSlug, &savedCollection); err != nil {
					t.Fatal(err)
				}
				if savedSlug != slug || savedCollection != coll.ID {
					t.Errorf("saved slug/collection = %q/%d, want %q/%d", savedSlug, savedCollection, slug, coll.ID)
				}
			})
		}
	}
}
