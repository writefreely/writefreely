package writefreely

import (
	"database/sql"
)

type (
	Collection struct {
		ID              int64          `datastore:"id" json:"-"`
		Alias           string         `datastore:"alias" schema:"alias" json:"alias"`
		Title           string         `datastore:"title" schema:"title" json:"title"`
		Description     string         `datastore:"description" schema:"description" json:"description"`
		Direction       string         `schema:"dir" json:"dir,omitempty"`
		Language        string         `schema:"lang" json:"lang,omitempty"`
		StyleSheet      string         `datastore:"style_sheet" schema:"style_sheet" json:"style_sheet"`
		Script          string         `datastore:"script" schema:"script" json:"script,omitempty"`
		Public          bool           `datastore:"public" json:"public"`
		Visibility      collVisibility `datastore:"private" json:"-"`
		Format          string         `datastore:"format" json:"format,omitempty"`
		Views           int64          `json:"views"`
		OwnerID         int64          `datastore:"owner_id" json:"-"`
		PublicOwner     bool           `datastore:"public_owner" json:"-"`
		PreferSubdomain bool           `datastore:"prefer_subdomain" json:"-"`
		Domain          string         `datastore:"domain" json:"domain,omitempty"`
		IsDomainActive  bool           `datastore:"is_active" json:"-"`
		IsSecure        bool           `datastore:"is_secure" json:"-"`
		CustomHandle    string         `datastore:"handle" json:"-"`
		Email           string         `json:"email,omitempty"`
		URL             string         `json:"url,omitempty"`

		app *app
	}
	CollectionObj struct {
		Collection
		TotalPosts int           `json:"total_posts"`
		Owner      *User         `json:"owner,omitempty"`
		Posts      *[]PublicPost `json:"posts,omitempty"`
	}
	SubmittedCollection struct {
		// Data used for updating a given collection
		ID      int64
		OwnerID uint64

		// Form helpers
		PreferURL string `schema:"prefer_url" json:"prefer_url"`
		Privacy   int    `schema:"privacy" json:"privacy"`
		Pass      string `schema:"password" json:"password"`
		Federate  bool   `schema:"federate" json:"federate"`
		MathJax   bool   `schema:"mathjax" json:"mathjax"`
		Handle    string `schema:"handle" json:"handle"`

		// Actual collection values updated in the DB
		Alias           *string         `schema:"alias" json:"alias"`
		Title           *string         `schema:"title" json:"title"`
		Description     *string         `schema:"description" json:"description"`
		StyleSheet      *sql.NullString `schema:"style_sheet" json:"style_sheet"`
		Script          *sql.NullString `schema:"script" json:"script"`
		Visibility      *int            `schema:"visibility" json:"public"`
		Format          *sql.NullString `schema:"format" json:"format"`
		PreferSubdomain *bool           `schema:"prefer_subdomain" json:"prefer_subdomain"`
		Domain          *sql.NullString `schema:"domain" json:"domain"`
	}
	CollectionFormat struct {
		Format string
	}
)

// collVisibility represents the visibility level for the collection.
type collVisibility int
