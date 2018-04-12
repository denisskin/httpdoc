package httpdoc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHelloWorld(t *testing.T) {

	title := NewDocument("https://golang.org/").Title()

	assert.Equal(t, "The Go Programming Language", title)
}

func TestSimpleRequest(t *testing.T) {
	doc := NewDocument("https://golang.org/")
	err := doc.Load()

	assert.NoError(t, err)
	assert.Equal(t, "text/html", doc.ContentType())
	assert.Equal(t, "utf-8", doc.Charset())
	assert.Equal(t, "The Go Programming Language", doc.Title())
	assert.True(t, len(doc.Content()) > 0)
}

func TestCheckRedirect(t *testing.T) {
	doc := NewDocument("http://facebook.com/")
	err := doc.Load()

	assert.NoError(t, err)
	assert.Equal(t, "https://www.facebook.com/", doc.URL().String())
}

func TestSubmitForm(t *testing.T) {
	form := NewDocument("https://golang.org/").
		Forms().
		Eq(0)

	assert.NotNil(t, form)
	assert.Equal(t, "/search", form.Attributes["action"])
	assert.True(t, len(form.FormParams()) > 0)

	doc := form.Doc()
	doc.SetParam("q", "sha256")
	doc.Submit()

	assert.Equal(t, "https://golang.org/search?q=sha256", doc.URL().String())
	assert.Equal(t, "sha256 - The Go Programming Language", doc.Title())
}

func TestGoogleTranslateWebSite(t *testing.T) {
	site, from, to := "https://golang.org/", "en", "ru"

	doc := NewDocument("http://translate.google.com/translate?hl=").
		SetParam("sl", from).
		SetParam("tl", to).
		SetParam("u", site).
		Frames().Eq(0).Doc(). // get document from first frame
		Links().Eq(0).Doc()   // get document from first link (A-tag)

	assert.Equal(t, "Язык программирования Go", doc.Title())
}
