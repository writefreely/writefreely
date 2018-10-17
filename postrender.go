package writefreely

import (
	"bytes"
	"github.com/microcosm-cc/bluemonday"
	stripmd "github.com/writeas/go-strip-markdown"
	"github.com/writeas/saturday"
	"html"
	"html/template"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	blockReg        = regexp.MustCompile("<(ul|ol|blockquote)>\n")
	endBlockReg     = regexp.MustCompile("</([a-z]+)>\n</(ul|ol|blockquote)>")
	youtubeReg      = regexp.MustCompile("(https?://www.youtube.com/embed/[a-zA-Z0-9\\-_]+)(\\?[^\t\n\f\r \"']+)?")
	titleElementReg = regexp.MustCompile("</?h[1-6]>")
	hashtagReg      = regexp.MustCompile(`#([\p{L}\p{M}\d]+)`)
	markeddownReg   = regexp.MustCompile("<p>(.+)</p>")
)

func (p *Post) formatContent(c *Collection, isOwner bool) {
	baseURL := c.CanonicalURL()
	if isOwner {
		baseURL = "/" + c.Alias + "/"
	}
	newCon := hashtagReg.ReplaceAllFunc([]byte(p.Content), func(b []byte) []byte {
		// Ensure we only replace "hashtags" that have already been extracted.
		// `hashtagReg` catches everything, including any hash on the end of a
		// URL, so we rely on p.Tags as the final word on whether or not to link
		// a tag.
		for _, t := range p.Tags {
			if string(b) == "#"+t {
				return bytes.Replace(b, []byte("#"+t), []byte("<a href=\""+baseURL+"tag:"+t+"\" class=\"hashtag\"><span>#</span><span class=\"p-category\">"+t+"</span></a>"), -1)
			}
		}
		return b
	})
	p.HTMLTitle = template.HTML(applyBasicMarkdown([]byte(p.Title.String)))
	p.HTMLContent = template.HTML(applyMarkdown([]byte(newCon)))
	if exc := strings.Index(string(newCon), "<!--more-->"); exc > -1 {
		p.HTMLExcerpt = template.HTML(applyMarkdown([]byte(newCon[:exc])))
	}
}

func (p *PublicPost) formatContent(isOwner bool) {
	p.Post.formatContent(&p.Collection.Collection, isOwner)
}

func applyMarkdown(data []byte) string {
	return applyMarkdownSpecial(data, false)
}

func applyMarkdownSpecial(data []byte, skipNoFollow bool) string {
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

	// Generate Markdown
	md := blackfriday.Markdown([]byte(data), blackfriday.HtmlRenderer(htmlFlags, "", ""), mdExtensions)
	// Strip out bad HTML
	policy := getSanitizationPolicy()
	policy.RequireNoFollowOnLinks(!skipNoFollow)
	outHTML := string(policy.SanitizeBytes(md))
	// Strip newlines on certain block elements that render with them
	outHTML = blockReg.ReplaceAllString(outHTML, "<$1>")
	outHTML = endBlockReg.ReplaceAllString(outHTML, "</$1></$2>")
	// Remove all query parameters on YouTube embed links
	// TODO: make this more specific. Taking the nuclear approach here to strip ?autoplay=1
	outHTML = youtubeReg.ReplaceAllString(outHTML, "$1")

	return outHTML
}

func applyBasicMarkdown(data []byte) string {
	mdExtensions := 0 |
		blackfriday.EXTENSION_STRIKETHROUGH |
		blackfriday.EXTENSION_SPACE_HEADERS |
		blackfriday.EXTENSION_HEADER_IDS
	htmlFlags := 0 |
		blackfriday.HTML_SKIP_HTML |
		blackfriday.HTML_USE_SMARTYPANTS |
		blackfriday.HTML_SMARTYPANTS_DASHES

	// Generate Markdown
	md := blackfriday.Markdown([]byte(data), blackfriday.HtmlRenderer(htmlFlags, "", ""), mdExtensions)
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

	// Strip HTML tags with bluemonday's StrictPolicy, then unescape the HTML
	// entities added in by sanitizing the content.
	content = html.UnescapeString(bluemonday.StrictPolicy().Sanitize(content))

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

func getSanitizationPolicy() *bluemonday.Policy {
	policy := bluemonday.UGCPolicy()
	policy.AllowAttrs("src", "style").OnElements("iframe", "video")
	policy.AllowAttrs("frameborder", "width", "height").Matching(bluemonday.Integer).OnElements("iframe")
	policy.AllowAttrs("allowfullscreen").OnElements("iframe")
	policy.AllowAttrs("controls", "loop", "muted", "autoplay").OnElements("video")
	policy.AllowAttrs("target").OnElements("a")
	policy.AllowAttrs("style", "class", "id").Globally()
	policy.AllowURLSchemes("http", "https", "mailto", "xmpp")
	return policy
}
