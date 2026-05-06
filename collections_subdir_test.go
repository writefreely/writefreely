package writefreely

import "testing"

func TestCollectionCanonicalURLIncludesSubdirectoryForAppHost(t *testing.T) {
	prevHost := canonicalAppHost
	prevSubdir := canonicalSubdir
	defer func() {
		canonicalAppHost = prevHost
		canonicalSubdir = prevSubdir
	}()

	canonicalAppHost = "https://example.com"
	canonicalSubdir = "/blog"
	isSingleUser = false

	c := &Collection{Alias: "myblog", hostName: "https://example.com"}
	got := c.CanonicalURL()
	want := "https://example.com/blog/myblog/"
	if got != want {
		t.Fatalf("unexpected canonical URL: got %q want %q", got, want)
	}
}

func TestCollectionCanonicalURLDoesNotAlterCustomDomain(t *testing.T) {
	prevHost := canonicalAppHost
	prevSubdir := canonicalSubdir
	defer func() {
		canonicalAppHost = prevHost
		canonicalSubdir = prevSubdir
	}()

	canonicalAppHost = "https://example.com"
	canonicalSubdir = "/blog"
	isSingleUser = false

	c := &Collection{Alias: "myblog", hostName: "https://blog.example.net"}
	got := c.CanonicalURL()
	want := "https://blog.example.net/myblog/"
	if got != want {
		t.Fatalf("unexpected canonical URL for custom domain: got %q want %q", got, want)
	}
}
