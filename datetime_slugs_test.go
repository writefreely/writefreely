package writefreely

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/writefreely/writefreely/config"
)

func slugTestPost(t *testing.T, app *App, userID, collID int64, created, explicit string) *Post {
	t.Helper()
	title, content := "Draft Title", "Draft body"
	submitted := &SubmittedPost{Title: &title, Content: &content, Created: &created}
	if explicit != "" {
		submitted.Slug = &explicit
	}
	post, err := app.db.CreatePost(app.cfg, userID, collID, submitted)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := app.db.Exec("DELETE FROM posts WHERE id = ?", post.ID); err != nil {
			t.Error(err)
		}
	})
	return post
}

func slugTestClaim(t *testing.T, app *App, userID int64, alias, id string, viaClaim bool) string {
	t.Helper()
	requests := []ClaimPostRequest{{AnonymousAuthPost: &AnonymousAuthPost{ID: id}}}
	target := alias
	if viaClaim {
		target = ""
		requests[0].CollectionAlias = alias
	}
	results, err := app.db.ClaimPosts(app.cfg, userID, target, &requests)
	if err != nil {
		t.Fatal(err)
	}
	if results == nil || len(*results) != 1 || (*results)[0].Code != http.StatusOK || (*results)[0].Post == nil {
		t.Fatalf("claim failed: %+v", results)
	}
	post := (*results)[0].Post
	var saved string
	var collection int64
	if err := app.db.QueryRow("SELECT slug, collection_id FROM posts WHERE id = ?", id).Scan(&saved, &collection); err != nil {
		t.Fatal(err)
	}
	if saved != post.Slug.String || collection != post.Collection.ID {
		t.Fatal("claim response differs from persisted metadata")
	}
	return saved
}

func TestBlueNoteConfiguredDatetimeSlugs(t *testing.T) {
	app, _ := newTemplateTestApp(t, func(c *config.Config) { c.App.SingleUser = false })
	for _, alias := range []string{"blue0a6m5c", "otherblog"} {
		user, coll, _ := createTemplateTestUser(t, app, alias)
		for _, enabled := range []bool{false, true} {
			for _, route := range []string{"direct", "collect", "claim"} {
				for _, created := range []string{"2001-02-03T15:05:06Z", "2099-02-03T15:05:06Z"} {
					t.Run(fmt.Sprintf("%s/on=%t/%s/%s", alias, enabled, route, created), func(t *testing.T) {
						app.cfg.App.DatetimeSlugs, app.cfg.App.DatetimeSlugTimezone = enabled, "Asia/Tokyo"
						collID := int64(0)
						if route == "direct" {
							collID = coll.ID
						}
						post := slugTestPost(t, app, user.ID, collID, created, "")
						got := post.Slug.String
						if route != "direct" {
							if got != "" {
								t.Fatal("draft must not generate a slug")
							}
							got = slugTestClaim(t, app, user.ID, alias, post.ID, route == "claim")
						}
						want := "draft-title"
						if enabled {
							want = created[:4] + "0204000506"
						}
						if got != want {
							t.Fatalf("slug = %q, want %q", got, want)
						}
					})
				}
			}
		}
		app.cfg.App.DatetimeSlugs = false
	}
}

func TestBlueNoteSavedSlugStability(t *testing.T) {
	app, _ := newTemplateTestApp(t, func(c *config.Config) { c.App.SingleUser = false })
	user, coll, _ := createTemplateTestUser(t, app, "stable")
	for _, explicit := range []string{"", "my-permanent-url"} {
		t.Run("explicit="+explicit, func(t *testing.T) {
			app.cfg.App.DatetimeSlugs, app.cfg.App.DatetimeSlugTimezone = true, "Asia/Tokyo"
			post := slugTestPost(t, app, user.ID, coll.ID, "2001-02-03T15:05:06Z", explicit)
			want := "20010204000506"
			if explicit != "" {
				want = explicit
			}
			if post.Slug.String != want {
				t.Fatalf("initial slug = %q, want %q", post.Slug.String, want)
			}
			for _, enabled := range []bool{true, false, true} {
				app.cfg.App.DatetimeSlugs, app.cfg.App.DatetimeSlugTimezone = enabled, "UTC"
				created := "2099-01-01T00:00:00Z"
				if err := app.db.UpdateOwnedPost(&AuthenticatedPost{ID: post.ID, SubmittedPost: &SubmittedPost{Created: &created}}, user.ID); err != nil {
					t.Fatal(err)
				}
				var saved string
				if err := app.db.QueryRow("SELECT slug FROM posts WHERE id = ?", post.ID).Scan(&saved); err != nil {
					t.Fatal(err)
				}
				if saved != want {
					t.Fatal("date edit changed slug")
				}
				if got := slugTestClaim(t, app, user.ID, coll.Alias, post.ID, false); got != want {
					t.Fatal("repeated collect changed slug")
				}
				ids := []string{post.ID}
				results, err := app.db.DispersePosts(user.ID, ids)
				if err != nil || (*results)[0].Code != http.StatusOK {
					t.Fatalf("disperse failed: %v", err)
				}
				if got := slugTestClaim(t, app, user.ID, coll.Alias, post.ID, true); got != want {
					t.Fatal("reclaim changed saved slug")
				}
			}
		})
	}
}

func TestBlueNoteDatetimeSlugCollisions(t *testing.T) {
	app, _ := newTemplateTestApp(t, func(c *config.Config) { c.App.SingleUser = false })
	user, coll, _ := createTemplateTestUser(t, app, "collisions")
	app.cfg.App.DatetimeSlugs = true
	created, base := "2001-02-03T04:05:06Z", "20010203040506"
	first := slugTestPost(t, app, user.ID, coll.ID, created, "")
	second := slugTestPost(t, app, user.ID, coll.ID, created, "")
	if first.Slug.String != base || second.Slug.String == base || !strings.HasPrefix(second.Slug.String, base) {
		t.Fatal("direct creation did not resolve timestamp collision")
	}
	draft := slugTestPost(t, app, user.ID, 0, created, "")
	got := slugTestClaim(t, app, user.ID, coll.Alias, draft.ID, false)
	if got == base || got == second.Slug.String || !strings.HasPrefix(got, base) {
		t.Fatal("claim did not resolve timestamp collision")
	}
	other, err := app.db.CreateCollection(app.cfg, "destination", "", user.ID)
	if err != nil {
		t.Fatal(err)
	}
	slugTestPost(t, app, user.ID, other.ID, created, base)
	requests := []ClaimPostRequest{{AnonymousAuthPost: &AnonymousAuthPost{ID: first.ID}}}
	results, err := app.db.ClaimPosts(app.cfg, user.ID, other.Alias, &requests)
	if err != nil || (*results)[0].Code == http.StatusOK {
		t.Fatalf("saved slug collision should reject move: %v", err)
	}
	var saved string
	var collID int64
	if err := app.db.QueryRow("SELECT slug, collection_id FROM posts WHERE id = ?", first.ID).Scan(&saved, &collID); err != nil {
		t.Fatal(err)
	}
	if saved != base || collID != coll.ID {
		t.Fatal("failed move modified persisted URL")
	}

}

func TestBlueNoteSlugDefaultsAndExplicitValues(t *testing.T) {
	app, _ := newTemplateTestApp(t, func(c *config.Config) { c.App.SingleUser = false })
	user, coll, _ := createTemplateTestUser(t, app, "defaults")
	for _, tc := range []struct{ name, title, body, explicit, want string }{
		{"title", "Some Title", "Some body", "", "some-title"},
		{"body", "", "Some body", "", "some-body"},
		{"id fallback", "", "!!!", "", ""},
		{"explicit", "Title", "Body", "chosen-url", "chosen-url"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app.cfg.App.DatetimeSlugs = false
			post, err := app.db.CreatePost(app.cfg, user.ID, coll.ID, &SubmittedPost{Title: &tc.title, Content: &tc.body, Slug: &tc.explicit})
			if err != nil {
				t.Fatal(err)
			}
			want := tc.want
			if want == "" {
				want = post.ID
			}
			if post.Slug.String != want {
				t.Fatalf("slug = %q, want %q", post.Slug.String, want)
			}
		})
	}
	app.cfg.App.DatetimeSlugs, app.cfg.App.DatetimeSlugTimezone = true, "UTC"
	title, content := "Current", "Body"
	post, err := app.db.CreatePost(app.cfg, user.ID, coll.ID, &SubmittedPost{Title: &title, Content: &content})
	if err != nil {
		t.Fatal(err)
	}
	if post.Slug.String != post.Created.UTC().Format("20060102150405") {
		t.Fatal("slug does not match the resolved default Created")
	}

	// Exercise the access-token creation path too.
	token, err := app.db.GetAccessToken(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	created := "2002-03-04T05:06:07Z"
	owned, err := app.db.CreateOwnedPost(app.cfg, &SubmittedPost{Title: &title, Content: &content, Created: &created}, "Token "+token, coll.Alias, app.cfg.App.Host)
	if err != nil {
		t.Fatal(err)
	}
	if owned.Slug.String != "20020304050607" {
		t.Fatalf("token creation slug = %q", owned.Slug.String)
	}
}

func TestBlueNoteAnonymousClaimDatetimeSlug(t *testing.T) {
	app, _ := newTemplateTestApp(t, func(c *config.Config) { c.App.SingleUser = false })
	user, coll, _ := createTemplateTestUser(t, app, "anonymous-claim")
	app.cfg.App.DatetimeSlugs, app.cfg.App.DatetimeSlugTimezone = true, "Asia/Tokyo"
	post := slugTestPost(t, app, 0, 0, "2001-02-03T15:05:06Z", "")
	if _, err := app.db.Exec("UPDATE posts SET modify_token = ? WHERE id = ?", "edit-token", post.ID); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"wrong-token", "edit-token"} {
		requests := []ClaimPostRequest{{AnonymousAuthPost: &AnonymousAuthPost{ID: post.ID, Token: token}}}
		results, err := app.db.ClaimPosts(app.cfg, user.ID, coll.Alias, &requests)
		if err != nil {
			t.Fatal(err)
		}
		result := (*results)[0]
		if token == "wrong-token" {
			if result.Code != http.StatusForbidden {
				t.Fatalf("invalid token accepted: %+v", result)
			}
			continue
		}
		if result.Code != http.StatusOK || result.Post.Slug.String != "20010204000506" {
			t.Fatalf("anonymous claim: %+v", result)
		}
		if !result.Post.Created.Equal(time.Date(2001, 2, 3, 15, 5, 6, 0, time.UTC)) {
			t.Fatal("claim changed publication time")
		}
	}
}
