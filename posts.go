package writefreely

import (
	"github.com/guregu/null"
	"github.com/guregu/null/zero"
	"github.com/kylemcc/twitter-text-go/extract"
	"github.com/writeas/monday"
	"github.com/writeas/slug"
	"github.com/writeas/web-core/converter"
	"github.com/writeas/web-core/parse"
	"github.com/writeas/web-core/tags"
	"html/template"
	"regexp"
	"time"
)

const (
	// Post ID length bounds
	minIDLen      = 10
	maxIDLen      = 10
	userPostIDLen = 10
	postIDLen     = 10

	postMetaDateFormat = "2006-01-02 15:04:05"
)

type (
	AuthenticatedPost struct {
		ID string `json:"id" schema:"id"`
		*SubmittedPost
	}

	// SubmittedPost represents a post supplied by a client for publishing or
	// updating. Since Title and Content can be updated to "", they are
	// pointers that can be easily tested to detect changes.
	SubmittedPost struct {
		Slug     *string                  `json:"slug" schema:"slug"`
		Title    *string                  `json:"title" schema:"title"`
		Content  *string                  `json:"body" schema:"body"`
		Font     string                   `json:"font" schema:"font"`
		IsRTL    converter.NullJSONBool   `json:"rtl" schema:"rtl"`
		Language converter.NullJSONString `json:"lang" schema:"lang"`
		Created  *string                  `json:"created" schema:"created"`

		// [{ "medium": "ev" }, { "twitter": "ilikebeans" }]
		Crosspost []map[string]string `json:"crosspost" schema:"crosspost"`
	}

	// Post represents a post as found in the database.
	Post struct {
		ID             string        `db:"id" json:"id"`
		Slug           null.String   `db:"slug" json:"slug,omitempty"`
		Font           string        `db:"text_appearance" json:"appearance"`
		Language       zero.String   `db:"language" json:"language"`
		RTL            zero.Bool     `db:"rtl" json:"rtl"`
		Privacy        int64         `db:"privacy" json:"-"`
		OwnerID        null.Int      `db:"owner_id" json:"-"`
		CollectionID   null.Int      `db:"collection_id" json:"-"`
		PinnedPosition null.Int      `db:"pinned_position" json:"-"`
		Created        time.Time     `db:"created" json:"created"`
		Updated        time.Time     `db:"updated" json:"updated"`
		ViewCount      int64         `db:"view_count" json:"-"`
		EmbedViewCount int64         `db:"embed_view_count" json:"-"`
		Title          zero.String   `db:"title" json:"title"`
		HTMLTitle      template.HTML `db:"title" json:"-"`
		Content        string        `db:"content" json:"body"`
		HTMLContent    template.HTML `db:"content" json:"-"`
		HTMLExcerpt    template.HTML `db:"content" json:"-"`
		Tags           []string      `json:"tags"`
		Images         []string      `json:"images,omitempty"`

		OwnerName string `json:"owner,omitempty"`
	}

	// PublicPost holds properties for a publicly returned post, i.e. a post in
	// a context where the viewer may not be the owner. As such, sensitive
	// metadata for the post is hidden and properties supporting the display of
	// the post are added.
	PublicPost struct {
		*Post
		IsSubdomain bool           `json:"-"`
		IsTopLevel  bool           `json:"-"`
		Domain      string         `json:"-"`
		DisplayDate string         `json:"-"`
		Views       int64          `json:"views"`
		Owner       *PublicUser    `json:"-"`
		IsOwner     bool           `json:"-"`
		Collection  *CollectionObj `json:"collection,omitempty"`
	}

	AnonymousAuthPost struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	ClaimPostRequest struct {
		*AnonymousAuthPost
		CollectionAlias  string `json:"collection"`
		CreateCollection bool   `json:"create_collection"`

		// Generated properties
		Slug string `json:"-"`
	}
	ClaimPostResult struct {
		ID           string      `json:"id,omitempty"`
		Code         int         `json:"code,omitempty"`
		ErrorMessage string      `json:"error_msg,omitempty"`
		Post         *PublicPost `json:"post,omitempty"`
	}
)

func (p *Post) processPost() PublicPost {
	res := &PublicPost{Post: p, Views: 0}
	res.Views = p.ViewCount
	// TODO: move to own function
	loc := monday.FuzzyLocale(p.Language.String)
	res.DisplayDate = monday.Format(p.Created, monday.LongFormatsByLocale[loc], loc)

	return *res
}

// TODO: merge this into getSlugFromPost or phase it out
func getSlug(title, lang string) string {
	return getSlugFromPost("", title, lang)
}

func getSlugFromPost(title, body, lang string) string {
	if title == "" {
		title = postTitle(body, body)
	}
	title = parse.PostLede(title, false)
	// Truncate lede if needed
	title, _ = parse.TruncToWord(title, 80)
	if lang != "" && len(lang) == 2 {
		return slug.MakeLang(title, lang)
	}
	return slug.Make(title)
}

// isFontValid returns whether or not the submitted post's appearance is valid.
func (p *SubmittedPost) isFontValid() bool {
	validFonts := map[string]bool{
		"norm": true,
		"sans": true,
		"mono": true,
		"wrap": true,
		"code": true,
	}

	if _, valid := validFonts[p.Font]; valid {
		return true
	}
	return false
}

func (p *Post) extractData() {
	p.Tags = tags.Extract(p.Content)
	p.extractImages()
}

var imageURLRegex = regexp.MustCompile(`(?i)^https?:\/\/[^ ]*\.(gif|png|jpg|jpeg)$`)

func (p *Post) extractImages() {
	matches := extract.ExtractUrls(p.Content)
	urls := map[string]bool{}
	for i := range matches {
		u := matches[i].Text
		if !imageURLRegex.MatchString(u) {
			continue
		}
		urls[u] = true
	}

	resURLs := make([]string, 0)
	for k := range urls {
		resURLs = append(resURLs, k)
	}
	p.Images = resURLs
}
