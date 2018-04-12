# httpdoc
golang advanced http-client

## Usage

#### Simple request
``` golang
    title := httpdoc.NewDocument("https://golang.org/").Title()
    
    println(title) // -> "The Go Programming Language"
```

#### Form request
``` golang
    form := httpdoc.NewDocument("https://golang.org/").Forms().Eq(0)

    doc := form.Doc()
    doc.SetParam("q", "gopher")
    doc.Submit()
    println(doc.Title()) // -> "gopher - The Go Programming Language"
```

#### Import list of friends from facebook profile
``` golang
    doc := httpdoc.NewDocument("https://facebook.com/")
    
    form := doc.Forms().GetByID("login_form")
    doc = form.Doc()
    doc.SetParam("email", "test@testmail.com")
    doc.SetParam("pass", "*******")
    doc.Submit()

    profileID := doc.Submatch(`facebook\.com/([a-z0-9\.\-]+)\?ref=bookmarks`, 1)
    println("My profileID: ", profileID)
    
    println("My friends:")
    doc = doc.NewDoc(fmt.Sprintf("/%s/friends", profileID))
    matches := doc.MatchAll(`\{id:"10\d{10,16}",[^{}]+\}`)
    for _, m := range matches {
        println(m[0])
    }
```

#### Google translate website
``` golang
    site, from, to := "https://golang.org/", "en", "ru"
    
    doc := httpdoc.NewDocument("http://translate.google.com/translate?hl=")
    doc.SetParam("sl", from)
    doc.SetParam("tl", to)
    doc.SetParam("u", site)
    
    doc = doc.Frames().Eq(0).Doc()  // get document from first iframe
    doc = doc.Links().Eq(0).Doc()   // get document from first link (A-tag)
    
    print(doc.Title()) // -> "Язык программирования Go"
```