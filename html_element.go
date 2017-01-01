package httpdoc

import (
	"fmt"
	"golang.org/x/net/html"
	"net/url"
	"regexp"
	"strings"
)

type HTMLElement struct {
	Document   *Document
	TagName    string
	Attributes map[string]string
	InnerHTML  string
}

var (
	reTag      = regexp.MustCompile(`<[a-zA-Z0-9\-]+[^<>]*/?>`)
	reInputs   = regexp.MustCompile(`<(input|textarea|select|button)\b([^<>]*)>`)
	reTagAttrs = regexp.MustCompile(`\s([a-zA-Z0-9\-]+)=('[^'<>]*'|"[^"<>]*")`)
)

func getElementsByTagName(d *Document, name string) (ee HTMLElements) {
	// todo: use html.NewTokenizer(bytes.NewBuffer(d.Body))

	name = regexp.QuoteMeta(name)
	re, err := regexp.Compile(`<(?i:` + name + `)\b([^<>]*)>([\s\S]*?)</(?i:` + name + `)>`)
	if err != nil {
		panic(err)
	}
	for _, ss := range re.FindAllStringSubmatch(d.Content(), -1) {
		ee = append(ee, &HTMLElement{
			Document:   d,
			TagName:    name,
			Attributes: parseTagAttrs(ss[1]),
			InnerHTML:  ss[2],
		})
	}
	return
}

func parseTagAttrs(s string) map[string]string {
	attrs := map[string]string{}
	for _, aa := range reTagAttrs.FindAllStringSubmatch(s, -1) {
		sVal := aa[2]
		attrs[strings.ToLower(aa[1])] = html.UnescapeString(sVal[1 : len(sVal)-1])
	}
	return attrs
}

func HtmlToText(s string) string {
	s = reTag.ReplaceAllString(s, "")
	return html.UnescapeString(s)
}

func (e *HTMLElement) InnerText() string {
	return HtmlToText(e.InnerHTML)
}

func (e *HTMLElement) String() string {
	sAttrs := ""
	for k, v := range e.Attributes {
		sAttrs += fmt.Sprintf(` %s="%s"`, k, html.EscapeString(v))
	}
	return fmt.Sprintf("<%s%s>\n%s\n</form>", e.TagName, sAttrs, e.InnerHTML)
}

func (e *HTMLElement) FormParams() url.Values {
	vals := url.Values{}
	for _, ss := range reInputs.FindAllStringSubmatch(e.InnerHTML, -1) {
		//tagName:=ss[1]
		attrs := parseTagAttrs(ss[2])
		if name, val := attrs["name"], attrs["value"]; name != "" {
			vals.Add(name, val)
		}
	}
	return vals
}

func (e *HTMLElement) Doc() *Document {
	switch e.TagName {
	case "form":
		doc := e.Document.NewDoc(e.Attributes["action"])
		if method := e.Attributes["method"]; method != "" {
			doc.Request.Method = strings.ToUpper(method)
		}
		doc.SetParams(e.FormParams())

		return doc

	case "a", "link", "image", "iframe", "frame":
		return e.Document.NewDoc(e.Attributes["src"])

	default:
		return e.Document.NewDoc("")
	}
}
