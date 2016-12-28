# httpdoc
golang advanced http client


    form := NewDocument("https://golang.org/").
		Forms().
		Eq(0)

	doc := form.NewDoc()
	doc.SetParam("q", "sha256")
	doc.Submit()
	assert.Equal(t, "sha256 - The Go Programming Language", doc.Title())
