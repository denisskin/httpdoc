package httpdoc

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSimpleRequest(t *testing.T) {
	doc := NewDocument("https://golang.org/")

	assert.Equal(t, "text/html", doc.ContentType())
	assert.Equal(t, "utf-8", doc.Charset())
	assert.Equal(t, "The Go Programming Language", doc.Title())
	assert.True(t, len(doc.Content()) > 0)
}

func TestSubmitForm(t *testing.T) {
	form := NewDocument("https://golang.org/").
		Forms().
		Eq(0)

	assert.NotNil(t, form)
	assert.Equal(t, "/search", form.Attributes["action"])
	assert.True(t, len(form.FormParams()) > 0)

	doc := form.NewDoc()
	doc.SetParam("q", "sha256")
	doc.Submit()

	assert.Equal(t, "https://golang.org/search?q=sha256", doc.URL().String())
	assert.Equal(t, "sha256 - The Go Programming Language", doc.Title())
}

func TestCheckRedirect(t *testing.T) {
	doc := NewDocument("http://facebook.com/")
	doc.Load()

	assert.Equal(t, "https://www.facebook.com/", doc.URL().String())
}
