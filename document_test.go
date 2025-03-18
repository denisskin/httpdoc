package httpdoc

import "testing"

func TestHelloWorld(t *testing.T) {

	title := NewDocument("https://golang.org/").Title()

	assert(t, "The Go Programming Language" == title)
}

func TestSimpleRequest(t *testing.T) {
	doc := NewDocument("https://go.dev/")
	err := doc.Load()

	assert(t, err == nil)
	assert(t, "text/html" == doc.ContentType())
	assert(t, "utf-8" == doc.Charset())
	assert(t, "The Go Programming Language" == doc.Title())
	assert(t, len(doc.Content()) > 0)
}

func TestCheckRedirect(t *testing.T) {
	doc := NewDocument("http://golang.org/")
	err := doc.Load()

	assert(t, err == nil)
	assert(t, "https://go.dev/" == doc.URL().String())
}

func TestSubmitForm(t *testing.T) {
	form := NewDocument("https://pkg.go.dev/about").
		Forms().
		Eq(0)

	assert(t, form != nil)
	assert(t, "/search" == form.Attributes["action"])
	assert(t, len(form.FormParams()) > 0)

	doc := form.Doc()
	doc.SetParam("q", "sha256")
	doc.Submit()

	assert(t, "https://pkg.go.dev/search?m=&q=sha256" == doc.URL().String())
	assert(t, "sha256 - Search Results - Go Packages" == doc.Title())
}

func _TestGoogleTranslateWebSite(t *testing.T) {
	site, from, to := "https://go.dev/", "en", "ru"

	doc := NewDocument("http://translate.google.com/translate?hl=").
		SetParam("sl", from).
		SetParam("tl", to).
		SetParam("u", site).
		Frames().Eq(0).Doc(). // get document from first frame
		Links().Eq(0).Doc()   // get document from first link (A-tag)

	assert(t, "Язык программирования Go" == doc.Title())
}

func assert(t *testing.T, ok bool) {
	if !ok {
		t.Fail()
	}
}
