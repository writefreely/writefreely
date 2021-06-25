/*
 * Copyright © 2018-2021 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
	stripmd "github.com/writeas/go-strip-markdown"
	"github.com/writeas/impart"
	blackfriday "github.com/writeas/saturday"
	"github.com/writeas/web-core/log"
	"github.com/writeas/web-core/stringmanip"
	"github.com/writefreely/writefreely/config"
	"github.com/writefreely/writefreely/parse"
)

var (
	blockReg        = regexp.MustCompile("<(ul|ol|blockquote)>\n")
	endBlockReg     = regexp.MustCompile("</([a-z]+)>\n</(ul|ol|blockquote)>")
	youtubeReg      = regexp.MustCompile("(https?://www.youtube.com/embed/[a-zA-Z0-9\\-_]+)(\\?[^\t\n\f\r \"']+)?")
	titleElementReg = regexp.MustCompile("</?h[1-6]>")
	hashtagReg      = regexp.MustCompile(`{{\[\[\|\|([^|]+)\|\|\]\]}}`)
	markeddownReg   = regexp.MustCompile("<p>(.+)</p>")
	mentionReg      = regexp.MustCompile(`@([A-Za-z0-9._%+-]+)(@[A-Za-z0-9.-]+\.[A-Za-z]+)\b`)
)

func (p *Post) handlePremiumContent(c *Collection, isOwner, postPage bool, cfg *config.Config) {
	if c.Monetization != "" {
		// User has Web Monetization enabled, so split content if it exists
		spl := strings.Index(p.Content, shortCodePaid)
		p.IsPaid = spl > -1
		if postPage {
			// We're viewing the individual post
			if isOwner {
				p.Content = strings.Replace(p.Content, shortCodePaid, "\n\n"+`<p class="split">Your subscriber content begins here.</p>`+"\n\n", 1)
			} else {
				if spl > -1 {
					p.Content = p.Content[:spl+len(shortCodePaid)]
					p.Content = strings.Replace(p.Content, shortCodePaid, "\n\n"+`<p class="split">Continue reading with a <strong>Coil</strong> membership.</p>`+"\n\n", 1)
				}
			}
		} else {
			// We've viewing the post on the collection landing
			if spl > -1 {
				baseURL := c.CanonicalURL()
				if isOwner {
					baseURL = "/" + c.Alias + "/"
				}

				p.Content = p.Content[:spl+len(shortCodePaid)]
				p.HTMLExcerpt = template.HTML(applyMarkdown([]byte(p.Content[:spl]), baseURL, cfg))
			}
		}
	}
}

func (p *Post) formatContent(cfg *config.Config, c *Collection, isOwner bool, isPostPage bool) {
	baseURL := c.CanonicalURL()
	// TODO: redundant
	if !isSingleUser {
		baseURL = "/" + c.Alias + "/"
	}

	p.handlePremiumContent(c, isOwner, isPostPage, cfg)
	p.Content = strings.Replace(p.Content, "&lt;!--paid-->", "<!--paid-->", 1)

	p.HTMLTitle = template.HTML(applyBasicMarkdown([]byte(p.Title.String)))
	p.HTMLContent = template.HTML(applyMarkdown([]byte(p.Content), baseURL, cfg))
	if exc := strings.Index(string(p.Content), "<!--more-->"); exc > -1 {
		p.HTMLExcerpt = template.HTML(applyMarkdown([]byte(p.Content[:exc]), baseURL, cfg))
	}
}

func (p *PublicPost) formatContent(cfg *config.Config, isOwner bool, isPostPage bool) {
	p.Post.formatContent(cfg, &p.Collection.Collection, isOwner, isPostPage)
}

func (p *Post) augmentContent(c *Collection) {
	if p.PinnedPosition.Valid {
		// Don't augment posts that are pinned
		return
	}
	if strings.Index(p.Content, "<!--nosig-->") > -1 {
		// Don't augment posts with the special "nosig" shortcode
		return
	}
	// Add post signatures
	if c.Signature != "" {
		p.Content += "\n\n" + c.Signature
	}
}

func (p *PublicPost) augmentContent() {
	p.Post.augmentContent(&p.Collection.Collection)
}

func (p *PublicPost) augmentReadingDestination() {
	if p.IsPaid {
		p.HTMLContent += template.HTML("\n\n" + `<p><a class="read-more" href="` + p.Collection.CanonicalURL() + p.Slug.String + `">` + localStr("Read more...", p.Language.String) + `</a> ($)</p>`)
	}
}

func applyMarkdown(data []byte, baseURL string, cfg *config.Config) string {
	return applyMarkdownSpecial(data, false, baseURL, cfg)
}

func disableYoutubeAutoplay(outHTML string) string {
	for _, match := range youtubeReg.FindAllString(outHTML, -1) {
		u, err := url.Parse(match)
		if err != nil {
			continue
		}
		u.RawQuery = html.UnescapeString(u.RawQuery)
		q := u.Query()
		// Set Youtube autoplay url parameter, if any, to 0
		if len(q["autoplay"]) == 1 {
			q.Set("autoplay", "0")
		}
		u.RawQuery = q.Encode()
		cleanURL := u.String()
		outHTML = strings.Replace(outHTML, match, cleanURL, 1)
	}
	return outHTML
}

func applyMarkdownSpecial(data []byte, skipNoFollow bool, baseURL string, cfg *config.Config) string {
	mdExtensions := 0 |
		blackfriday.EXTENSION_TABLES |
		blackfriday.EXTENSION_FENCED_CODE |
		blackfriday.EXTENSION_AUTOLINK |
		blackfriday.EXTENSION_STRIKETHROUGH |
		blackfriday.EXTENSION_SPACE_HEADERS |
		blackfriday.EXTENSION_AUTO_HEADER_IDS
	htmlFlags := 0 |
		blackfriday.HTML_USE_SMARTYPANTS |
		blackfriday.HTML_SMARTYPANTS_DASHES

	if baseURL != "" {
		htmlFlags |= blackfriday.HTML_HASHTAGS
	}

	// Generate Markdown
	md := blackfriday.Markdown([]byte(data), blackfriday.HtmlRenderer(htmlFlags, "", ""), mdExtensions)
	if baseURL != "" {
		// Replace special text generated by Markdown parser
		tagPrefix := baseURL + "tag:"
		if cfg.App.Chorus {
			tagPrefix = "/read/t/"
		}
		md = []byte(hashtagReg.ReplaceAll(md, []byte("<a href=\""+tagPrefix+"$1\" class=\"hashtag\"><span>#</span><span class=\"p-category\">$1</span></a>")))
		handlePrefix := cfg.App.Host + "/@/"
		md = []byte(mentionReg.ReplaceAll(md, []byte("<a href=\""+handlePrefix+"$1$2\" class=\"u-url mention\">@<span>$1$2</span></a>")))
	}
	// Strip out bad HTML
	policy := getSanitizationPolicy()
	policy.RequireNoFollowOnLinks(!skipNoFollow)
	outHTML := string(policy.SanitizeBytes(md))
	// Strip newlines on certain block elements that render with them
	outHTML = blockReg.ReplaceAllString(outHTML, "<$1>")
	outHTML = endBlockReg.ReplaceAllString(outHTML, "</$1></$2>")
	outHTML = disableYoutubeAutoplay(outHTML)
	return outHTML
}

func applyBasicMarkdown(data []byte) string {
	if len(bytes.TrimSpace(data)) == 0 {
		return ""
	}

	mdExtensions := 0 |
		blackfriday.EXTENSION_STRIKETHROUGH |
		blackfriday.EXTENSION_SPACE_HEADERS |
		blackfriday.EXTENSION_HEADER_IDS
	htmlFlags := 0 |
		blackfriday.HTML_SKIP_HTML |
		blackfriday.HTML_USE_SMARTYPANTS |
		blackfriday.HTML_SMARTYPANTS_DASHES

	// Generate Markdown
	// This passes the supplied title into blackfriday.Markdown() as an H1 header, so we only render HTML that
	// belongs in an H1.
	md := blackfriday.Markdown(append([]byte("# "), data...), blackfriday.HtmlRenderer(htmlFlags, "", ""), mdExtensions)
	// Remove H1 markup
	md = bytes.TrimSpace(md) // blackfriday.Markdown adds a newline at the end of the <h1>
	md = md[len("<h1>") : len(md)-len("</h1>")]
	// Strip out bad HTML
	policy := bluemonday.UGCPolicy()
	policy.AllowAttrs("class", "id").Globally()
	outHTML := string(policy.SanitizeBytes(md))
	outHTML = markeddownReg.ReplaceAllString(outHTML, "$1")
	outHTML = strings.TrimRightFunc(outHTML, unicode.IsSpace)

	return outHTML
}

func postTitle(content, friendlyId string) string {
	const maxTitleLen = 80

	content = stripHTMLWithoutEscaping(content)

	content = strings.TrimLeftFunc(stripmd.Strip(content), unicode.IsSpace)
	eol := strings.IndexRune(content, '\n')
	blankLine := strings.Index(content, "\n\n")
	if blankLine != -1 && blankLine <= eol && blankLine <= assumedTitleLen {
		return strings.TrimSpace(content[:blankLine])
	} else if utf8.RuneCountInString(content) <= maxTitleLen {
		return content
	}
	return friendlyId
}

// TODO: fix duplicated code from postTitle. postTitle is a widely used func we
// don't have time to investigate right now.
func friendlyPostTitle(content, friendlyId string) string {
	const maxTitleLen = 80

	content = stripHTMLWithoutEscaping(content)

	content = strings.TrimLeftFunc(stripmd.Strip(content), unicode.IsSpace)
	eol := strings.IndexRune(content, '\n')
	blankLine := strings.Index(content, "\n\n")
	if blankLine != -1 && blankLine <= eol && blankLine <= assumedTitleLen {
		return strings.TrimSpace(content[:blankLine])
	} else if eol == -1 && utf8.RuneCountInString(content) <= maxTitleLen {
		return content
	}
	title, truncd := parse.TruncToWord(parse.PostLede(content, true), maxTitleLen)
	if truncd {
		title += "..."
	}
	return title
}

// Strip HTML tags with bluemonday's StrictPolicy, then unescape the HTML
// entities added in by sanitizing the content.
func stripHTMLWithoutEscaping(content string) string {
	return html.UnescapeString(bluemonday.StrictPolicy().Sanitize(content))
}

func getSanitizationPolicy() *bluemonday.Policy {
	policy := bluemonday.UGCPolicy()
	policy.AllowAttrs("src", "style").OnElements("iframe", "video", "audio")
	policy.AllowAttrs("src", "type").OnElements("source")
	policy.AllowAttrs("frameborder", "width", "height").Matching(bluemonday.Integer).OnElements("iframe")
	policy.AllowAttrs("allowfullscreen").OnElements("iframe")
	policy.AllowAttrs("controls", "loop", "muted", "autoplay").OnElements("video")
	policy.AllowAttrs("controls", "loop", "muted", "autoplay", "preload").OnElements("audio")
	policy.AllowAttrs("target").OnElements("a")
	policy.AllowAttrs("title").OnElements("abbr")
	policy.AllowAttrs("style", "class", "id").Globally()
	policy.AllowElements("header", "footer")
	policy.AllowURLSchemes("http", "https", "mailto", "xmpp")
	return policy
}

func sanitizePost(content string) string {
	return strings.Replace(content, "<", "&lt;", -1)
}

// postDescription generates a description based on the given post content,
// title, and post ID. This doesn't consider a V2 post field, `title` when
// choosing what to generate. In case a post has a title, this function will
// fail, and logic should instead be implemented to skip this when there's no
// title, like so:
//    var desc string
//    if title == "" {
//        desc = postDescription(content, title, friendlyId)
//    } else {
//        desc = shortPostDescription(content)
//    }
func postDescription(content, title, friendlyId string) string {
	maxLen := 140

	if content == "" {
		content = "WriteFreely is a painless, simple, federated blogging platform."
	} else {
		fmtStr := "%s"
		truncation := 0
		if utf8.RuneCountInString(content) > maxLen {
			// Post is longer than the max description, so let's show a better description
			fmtStr = "%s..."
			truncation = 3
		}

		if title == friendlyId {
			// No specific title was found; simply truncate the post, starting at the beginning
			content = fmt.Sprintf(fmtStr, strings.Replace(stringmanip.Substring(content, 0, maxLen-truncation), "\n", " ", -1))
		} else {
			// There was a title, so return a real description
			blankLine := strings.Index(content, "\n\n")
			if blankLine < 0 {
				blankLine = 0
			}
			truncd := stringmanip.Substring(content, blankLine, blankLine+maxLen-truncation)
			contentNoNL := strings.Replace(truncd, "\n", " ", -1)
			content = strings.TrimSpace(fmt.Sprintf(fmtStr, contentNoNL))
		}
	}

	return content
}

func shortPostDescription(content string) string {
	maxLen := 140
	fmtStr := "%s"
	truncation := 0
	if utf8.RuneCountInString(content) > maxLen {
		// Post is longer than the max description, so let's show a better description
		fmtStr = "%s..."
		truncation = 3
	}
	return strings.TrimSpace(fmt.Sprintf(fmtStr, strings.Replace(stringmanip.Substring(content, 0, maxLen-truncation), "\n", " ", -1)))
}

func handleRenderMarkdown(app *App, w http.ResponseWriter, r *http.Request) error {
	if !IsJSON(r) {
		return impart.HTTPError{Status: http.StatusUnsupportedMediaType, Message: "Markdown API only supports JSON requests"}
	}

	in := struct {
		CollectionURL string `json:"collection_url"`
		RawBody       string `json:"raw_body"`
	}{}

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&in)
	if err != nil {
		log.Error("Couldn't parse markdown JSON request: %v", err)
		return ErrBadJSON
	}

	out := struct {
		Body string `json:"body"`
	}{
		Body: applyMarkdown([]byte(in.RawBody), in.CollectionURL, app.cfg),
	}

	return impart.WriteSuccess(w, out, http.StatusOK)
}
