package httpdoc

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"github.com/dsnet/compress/brotli"
	"github.com/goldic/js"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Document struct {
	Client   *http.Client
	Request  *http.Request
	Response *http.Response
	rawBody  []byte
	Body     []byte

	multiParts []*multipartPart
}

type multipartPart struct {
	header textproto.MIMEHeader
	io.ReadCloser
}

var DefaultHeader = http.Header{
	"Accept":          {"text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8"},
	"Accept-Encoding": {"gzip, deflate, br"},
	"Accept-Language": {"en-US,en;q=0.9"},
	"Cache-Control":   {"max-age=0"},
	"Connection":      {"keep-alive"},
	"Pragma":          {"no-cache"},
	"User-Agent":      {"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"},
}

func NewDocument(url string) *Document {
	return newDocument(url, DefaultClient)
}

func LoadJSON(url string, v any) error {
	doc := NewDocument(url)
	if err := doc.Load(); err != nil {
		return err
	}
	return doc.GetJSON(v)
}

func (d *Document) NewDoc(relURL string) *Document {
	if err := d.Load(); err != nil {
		panic(err)
	}
	org := d.Request.URL
	u, err := url.Parse(relURL)
	panicOnErr(err)
	u = org.ResolveReference(u)

	doc := newDocument(u.String(), d.Client)
	doc.SetHeader("Origin", org.Scheme+"://"+org.Host)
	doc.SetHeader("Referer", org.String())
	return doc
}

func (d *Document) NewAjax(relURL string) *Document {
	doc := d.NewDoc(relURL)
	if csrf := d.MetaCSRFToken(); csrf != "" {
		doc.SetHeader("x-csrf-token-auth", csrf)
	}
	doc.SetHeader("X-Requested-With", "XMLHttpRequest")
	return doc
}

func newDocument(urlStr string, client *http.Client) *Document {
	req, err := http.NewRequest("GET", urlStr, nil)
	panicOnErr(err)
	req.PostForm = url.Values{}
	if auth := req.URL.User; auth != nil {
		if username := auth.Username(); username != "" {
			password, _ := auth.Password()
			req.SetBasicAuth(username, password)
		}
	}
	for name, values := range DefaultHeader {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	return &Document{
		Client:  client,
		Request: req,
	}
}

func panicOnErr(err error) {
	if err != nil {
		panic(err)
	}
}

// ---------- request method --------------
func (d *Document) String() string {
	return fmt.Sprintf("httpdoc.Document(url=%s loaded=%v)", d.URL(), d.Loaded())
}

func (d *Document) Method() string {
	return d.Request.Method
}

func (d *Document) SetMethod(method string) *Document {
	d.Request.Method = method
	return d
}

func (d *Document) URL() *url.URL {
	if resp := d.Response; resp != nil {
		return resp.Request.URL
	}
	return d.Request.URL
}

func (d *Document) QueryParams() url.Values {
	return d.URL().Query()
}

func (d *Document) Param(name string) string {
	val := d.QueryParams().Get(name)
	if val != "" {
		return val
	}
	val = d.Request.PostForm.Get(name)
	return val
}

func (d *Document) SetParams(vals url.Values) *Document {
	if d.Request.Method == "POST" {
		d.Request.PostForm = vals
	} else {
		d.Request.URL.RawQuery = vals.Encode()
	}
	return d
}

func (d *Document) SetParam(name, val string) *Document {
	if d.Request.Method == "POST" {
		d.SetPOSTParam(name, val)
	} else {
		d.SetQueryParam(name, val)
	}
	return d
}

func (d *Document) SetQueryParam(name, val string) *Document {
	q := d.Request.URL.Query()
	q.Set(name, val)
	d.Request.URL.RawQuery = q.Encode()
	return d
}

func (d *Document) SetGETParam(name, val string) *Document {
	return d.SetQueryParam(name, val)
}

func (d *Document) SetPOSTParam(name, val string) *Document {
	d.Request.Method = "POST"
	d.Request.PostForm.Set(name, val)
	return d
}

func (d *Document) SetPOSTParams(vals url.Values) *Document {
	d.Request.Method = "POST"
	d.Request.PostForm = vals
	return d
}

func (d *Document) SetPOSTData(data []byte, contentType string) *Document {
	if contentType == "" {
		contentType = "application/x-www-form-urlencoded"
	}
	d.Request.Method = "POST"
	d.Request.Header.Set("Content-Type", contentType)
	d.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))
	d.Request.ContentLength = int64(len(data))
	return d
}

func (d *Document) SetJSON(v any) *Document {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	d.SetPOSTData(data, "application/json")
	return d
}

func (d *Document) SetCookies(cookies map[string]string) {
	d.Request.Header.Set("Cookie", "")
	d.AddCookies(cookies)
}

func (d *Document) AddCookies(cookies map[string]string) {
	cc := make([]*http.Cookie, 0, len(cookies))
	for name, val := range cookies {
		cc = append(cc, &http.Cookie{Name: name, Value: val})
	}
	d.addCookies(cc...)
}

func (d *Document) AddCookie(name, value string) {
	d.addCookies(&http.Cookie{Name: name, Value: value})
}

func (d *Document) addCookies(cookies ...*http.Cookie) {
	for _, c := range cookies {
		d.Request.AddCookie(c)
	}
	d.Client.Jar.SetCookies(d.URL(), cookies)
}

func (d *Document) IsMultipartRequest() bool {
	return d.multiParts != nil
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

func (d *Document) SetMultipartContent(paramName string, r io.ReadCloser, contentType string) {

	h := make(textproto.MIMEHeader)
	fileName := paramName
	if ext, _ := mime.ExtensionsByType(contentType); len(ext) > 0 {
		fileName += ext[0]
	}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
		escapeQuotes(paramName),
		escapeQuotes(fileName),
	))
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}

	d.Request.Method = "POST"
	d.multiParts = append(d.multiParts, &multipartPart{h, r})
}

func (d *Document) SetMultipartParams(vals url.Values) *Document {
	d.multiParts = []*multipartPart{} // set no nil
	return d.SetPOSTParams(vals)
}

func (d *Document) SetFile(name, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	d.SetMultipartContent(name, file, mime.TypeByExtension(path.Ext(filePath)))
	return nil
}

func (d *Document) SetUserAgent(ua string) {
	d.Request.Header.Set("User-Agent", ua)
}

func (d *Document) SetBasicAuth(username, password string) {
	d.Request.SetBasicAuth(username, password)
}

func (d *Document) Submit() error {
	return d.Load()
}

func (d *Document) AddHeader(name, value string) *Document {
	d.Request.Header.Add(name, value)
	return d
}

func (d *Document) SetHeaders(headers js.Object) *Document {
	for name := range headers {
		d.SetHeader(name, headers.GetStr(name))
	}
	return d
}

func (d *Document) SetHeader(name, value string) *Document {
	d.Request.Header.Set(name, value)
	return d
}

// ---------- response method --------------
func (d *Document) Loaded() bool {
	return d.Response != nil
}

func (d *Document) Load() error {
	if d.Loaded() {
		return nil
	}
	if d.IsMultipartRequest() {

		pr, pw := io.Pipe()
		mpWriter := multipart.NewWriter(pw)

		d.Request.Header.Set("Content-Type", mpWriter.FormDataContentType())
		d.Request.Body = pr

		go func() { // async write milti-parts to request.Body
			defer pw.Close()
			defer mpWriter.Close()

			for name, values := range d.Request.PostForm {
				for _, val := range values {
					if err := mpWriter.WriteField(name, val); err != nil {
						pw.CloseWithError(err)
						return
					}
				}
			}
			for _, mp := range d.multiParts {
				if w, err := mpWriter.CreatePart(mp.header); err != nil {
					pw.CloseWithError(err)
					return
				} else if _, err := io.Copy(w, mp); err != nil {
					pw.CloseWithError(err)
					return
				}
				mp.Close()
			}
		}()

	} else if len(d.Request.PostForm) > 0 {
		// set request body
		buf := bytes.NewBufferString(d.Request.PostForm.Encode())
		d.Request.Method = "POST"
		d.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		d.Request.Body = ioutil.NopCloser(buf)
		d.Request.ContentLength = int64(buf.Len())
	}
	if d.Request.ContentLength > 0 {
		d.Request.Header.Set("Content-Length", strconv.FormatInt(d.Request.ContentLength, 10))
	}
	if err := d.doRequest(); err != nil {
		return err
	}
	if charset := d.Charset(); charset != "utf-8" {
		d.Body, _ = Iconv(d.rawBody, charset)
	} else {
		d.Body = d.rawBody
	}

	// handle middleware
	for _, fn := range middlewares {
		err := func(fn middlewareFunc) (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("httpdoc.Document.Load-PANIC: %v", r)
				}
			}()
			return fn(d)
		}(fn)
		if err != nil {
			return err
		}
	}
	if status := d.Response.StatusCode; status >= 400 {
		return fmt.Errorf("http-status-code %d", status)
	}
	return nil
}

func (d *Document) doRequest() (err error) {
	if d.Response, err = d.Client.Do(d.Request); err != nil {
		return
	}
	defer d.Response.Body.Close()

	// Check that the server actually sent compressed data
	var reader io.ReadCloser
	switch d.Response.Header.Get("Content-Encoding") {
	// todo: "compress"
	// todo: "sdch"

	case "br":
		reader, err = brotli.NewReader(d.Response.Body, nil)
		defer reader.Close()

	case "gzip":
		reader, err = gzip.NewReader(d.Response.Body)
		defer reader.Close()

	case "deflate":
		reader = flate.NewReader(d.Response.Body)
		defer reader.Close()

	default:
		reader = d.Response.Body
	}
	if err != nil {
		return err
	}
	if d.rawBody, err = ioutil.ReadAll(reader); err != nil {
		return
	}
	return
}

func Iconv(buf []byte, charset string) ([]byte, error) {
	enc, err := htmlindex.Get(charset)
	if err != nil {
		return nil, err
	}
	tr := transform.NewReader(bytes.NewBuffer(buf), enc.NewDecoder())
	return ioutil.ReadAll(tr)
}

func (d *Document) ContentType() string {
	panicOnErr(d.Load())
	sContType := d.Response.Header.Get("Content-Type")
	s, _, _ := mime.ParseMediaType(sContType)
	return strings.ToLower(s)
}

func (d *Document) IsImage() bool {
	return strings.HasPrefix(d.ContentType(), "image")
}

func (d *Document) Charset() string {
	panicOnErr(d.Load())
	sContType := d.Response.Header.Get("Content-Type")
	if _, p, _ := mime.ParseMediaType(sContType); p != nil && p["charset"] != "" {
		return p["charset"]
	}
	return "utf-8"
}

func valuesToStr(vals map[string][]string) (res string) {
	for name, ss := range vals {
		for _, s := range ss {
			res += "\n- " + name + ": " + s
		}
	}
	return
}

func (d *Document) Trace() *Document {
	cont := d.ContentStr()
	fmt.Printf(
		`======== httpdoc.Request %s ========
Request URL: %s
Request Method: %s
Status Code: %s
Remote Address: %s
Request Headers: %s
Query String Parameters: %s
Form Data: %s
Response Headers: %s
RESPONSE:
%s
`,
		time.Now(),
		d.Request.URL,
		d.Request.Method,
		d.Response.Status,
		d.Request.RemoteAddr,
		valuesToStr(d.Request.Header),
		valuesToStr(d.Request.URL.Query()),
		valuesToStr(d.Request.PostForm),
		valuesToStr(d.Response.Header),
		cont,
	)
	return d
}

func (d *Document) ContentStr() string {
	return string(d.Content())
}

func (d *Document) Content() []byte {
	panicOnErr(d.Load())
	return d.Body
}

func (d *Document) ContentBuffer() *bytes.Buffer {
	panicOnErr(d.Load())
	return bytes.NewBuffer(d.Body)
}

func normRe(regExp any) *regexp.Regexp {
	switch v := regExp.(type) {
	case *regexp.Regexp:
		return v

	case string:
		return regexp.MustCompile(v)

	default:
		panic(fmt.Sprintf("Unknown req-exp format (%v)", regExp))
	}
}

func (d *Document) Match(regExp any) []string {
	return normRe(regExp).FindStringSubmatch(d.ContentStr())
}

func (d *Document) Submatch(regExp any, submatchNum int) string {
	if submatches := d.Match(regExp); len(submatches) > 0 {
		return submatches[submatchNum]
	}
	return ""
}

func (d *Document) MatchAll(regExp any) []string {
	return normRe(regExp).FindAllString(d.ContentStr(), -1)
}

func (d *Document) SubmatchAll(regExp any) [][]string {
	return normRe(regExp).FindAllStringSubmatch(d.ContentStr(), -1)
}

func (d *Document) AllSubmatches(regExp any, submatchNum int) (submatches []string) {
	for _, ss := range d.SubmatchAll(regExp) {
		submatches = append(submatches, ss[submatchNum])
	}
	return
}

func (d *Document) GetJSON(v any) error {
	if err := d.Load(); err != nil {
		return err
	}
	return json.Unmarshal(d.Content(), v)
}

func (d *Document) GetJSObject() (res js.Object, err error) {
	err = d.GetJSON(&res)
	return
}

// --------- html-document methods ---------------
func (d *Document) Title() string {
	if ee := d.GetElementsByTagName("title"); len(ee) > 0 {
		return ee[0].InnerText()
	}
	return ""
}

func (d *Document) GetElementsByTagName(name string) HTMLElements {
	return getElementsByTagName(d, name)
}

// Forms gets collections of html-tags <form>
func (d *Document) Forms() HTMLElements {
	return d.GetElementsByTagName("form")
}

// Links gets collections of html-tags <a>
func (d *Document) Links() HTMLElements {
	return d.GetElementsByTagName("a")
}

// Frames gets collections of html-tags <frame>, <iframe>
func (d *Document) Frames() HTMLElements {
	return append(d.GetElementsByTagName("iframe"), d.GetElementsByTagName("frame")...)
}

// Images gets collections of html-tags <img>
func (d *Document) Images() HTMLElements {
	return d.GetElementsByTagName("img")
}

// Scripts gets collections of html-tags <script>
func (d *Document) Scripts() HTMLElements {
	return d.GetElementsByTagName("script")
}

// MetaTags gets collections of html-tags <meta>
func (d *Document) MetaTags() HTMLElements {
	return d.GetElementsByTagName("meta")
}

// MetaDescription gets html-document description from <meta name="description">
func (d *Document) MetaDescription() string {
	if e := d.MetaTags().FilterByAttrValue("name", "description").First(); e != nil {
		return e.Attributes["content"]
	}
	return ""
}

func (d *Document) MetaCSRFToken() string {
	if e := d.MetaTags().FilterByAttrValue("name", "csrf-token").First(); e != nil {
		return e.Attributes["content"]
	}
	return ""
}

// MetaIcon gets image-url from meta-info for html-document
func (d *Document) MetaIcon() string {
	linkTags := d.GetElementsByTagName("link").FilterByAttr("href")
	for _, relVal := range []string{"icon", "shortcut icon", "apple-touch-icon"} {
		if tags := linkTags.FilterByAttrValue("rel", relVal); len(tags) > 0 {
			if tags := tags.FilterByAttrValue("type", "image/ico"); len(tags) > 0 {
				return tags[0].Attributes["href"]
			}
			return tags[0].Attributes["href"]
		}
	}
	return ""
}

// MetaImage gets image-url from meta-info for html-document
func (d *Document) MetaImage() string {
	if tag := d.GetElementsByTagName("link").FilterByAttrValue("rel", "image").FilterByAttr("href").First(); tag != nil {
		return tag.Attributes["href"]
	}
	if tag := d.GetElementsByTagName("link").FilterByAttrValue("rel", "image_src").FilterByAttr("href").First(); tag != nil {
		return tag.Attributes["href"]
	}
	if tag := d.GetElementsByTagName("meta").FilterByAttrValue("property", "og:image").FilterByAttr("content").First(); tag != nil {
		return tag.Attributes["content"]
	}
	return ""
}
