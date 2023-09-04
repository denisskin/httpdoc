package httpdoc

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

type HTMLElement struct {
	Document   *Document
	TagName    string
	Attributes map[string]string
	InnerHTML  string
}

var (
	reTag      = regexp.MustCompile(`</?[a-zA-Z0-9\-]+[^>]*>`)
	reSpec     = regexp.MustCompile(`(?si:<!--.*?-->|<[?!].*?>|<style.*?</style>|<script.*?</script>)`)
	reSpace    = regexp.MustCompile(`\s+`)
	reBr       = regexp.MustCompile(`<(?i:br|p)[^<>]*/?>`)
	reLi       = regexp.MustCompile(`<(?i:li)[^<>]*>`)
	reNewLine  = regexp.MustCompile(`(?s:\n+)`)
	reInputs   = regexp.MustCompile(`<(?i:input|textarea|select|button)\b([^>]*)>`)
	reTagAttrs = regexp.MustCompile(`\s([a-zA-Z0-9\-]+)=('[^']*'|"[^"]*")`)
)

var isSingleTag = map[string]bool{
	"meta":   true,
	"link":   true,
	"br":     true,
	"hr":     true,
	"input":  true,
	"option": true,
}

func getElementsByTagName(d *Document, name string) (ee HTMLElements) {
	// todo: use html.NewTokenizer(bytes.NewBuffer(d.Body))

	name = regexp.QuoteMeta(strings.ToLower(name))
	var reStr string
	if isSingleTag[name] {
		reStr = `(?i:<` + name + `\b([^>]*)/?>)`
	} else {
		reStr = `(?i:<` + name + `\b([^>]*)(?s:/>|>(.*?)</` + name + `>))`
	}
	re, err := regexp.Compile(reStr)
	if err != nil {
		panic(err)
	}
	for _, ss := range re.FindAllStringSubmatch(d.ContentStr(), -1) {
		e := &HTMLElement{
			Document:   d,
			TagName:    name,
			Attributes: parseTagAttrs(ss[1]),
		}
		if len(ss) > 2 {
			e.InnerHTML = strings.TrimSpace(ss[2])
		}
		ee = append(ee, e)
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
	s = reSpec.ReplaceAllString(s, "")
	s = reSpace.ReplaceAllString(s, " ")
	s = reBr.ReplaceAllString(s, "\n")
	s = reLi.ReplaceAllString(s, "\n- ")
	s = reTag.ReplaceAllString(s, "")
	s = reNewLine.ReplaceAllString(s, "\n")
	s = strings.TrimSpace(html.UnescapeString(s))
	return s
}

func (e *HTMLElement) InnerText() string {
	if e == nil {
		return ""
	}
	return HtmlToText(e.InnerHTML)
}

func (e *HTMLElement) String() string {
	s := `<` + e.TagName
	for k, v := range e.Attributes {
		s += fmt.Sprintf(` %s="%s"`, k, html.EscapeString(v))
	}
	if isSingleTag[e.TagName] {
		return s + ` />`
	}
	return s + `>` + e.InnerHTML + `</` + e.TagName + `>`
}

func (e *HTMLElement) FormParams() url.Values {
	vals := url.Values{}
	for _, ss := range reInputs.FindAllStringSubmatch(e.InnerHTML, -1) {
		attrs := parseTagAttrs(ss[1])
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

	case "a", "link":
		return e.Document.NewDoc(e.Attributes["href"])

	default:
		return e.Document.NewDoc(e.Attributes["src"])
	}
}
