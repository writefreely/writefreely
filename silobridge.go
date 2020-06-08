package writefreely

// fakeAPInstances contains a list of sites that we allow writers to mention
// with the @handle@instance.tld syntax, plus the corresponding prefix to
// insert between `https://instance.tld/` and `handle` (e.g.
// https://medium.com/@handle)
var fakeAPInstances = map[string]string{
	"twitter.com": "",
	"medium.com":  "@",
}
