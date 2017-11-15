package httpdoc

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/textproto"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"
)

type Document struct {
	Client   *http.Client
	Request  *http.Request
	Response *http.Response
	rawBody  []byte
	Body     []byte

	multipartWriter *multipart.Writer
}

var DefaultHeader = http.Header{
	"Accept":          {"text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8"},
	"Accept-Encoding": {"gzip, deflate"},
	"Cache-Control":   {"max-age=0"},
	"Connection":      {"keep-alive"},
	"User-Agent":      {"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36"},
}

var DefaultClient *http.Client

func init() {
	jar, err := cookiejar.New(nil)
	panicOnErr(err)
	DefaultClient = &http.Client{
		Jar:     jar,
		Timeout: 60 * time.Second,
	}
}

func NewDocument(url string) *Document {
	return newDocument(url, DefaultClient)
}

func (d *Document) NewDoc(relURL string) *Document {
	u, err := url.Parse(relURL)
	panicOnErr(err)
	u = d.Request.URL.ResolveReference(u)

	doc := newDocument(u.String(), d.Client)
	doc.Request.Header.Set("Referer", d.URL().String())
	return doc
}

func newDocument(urlStr string, client *http.Client) *Document {
	req, err := http.NewRequest("GET", urlStr, nil)
	panicOnErr(err)
	req.PostForm = url.Values{}

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

//---------- request method --------------
func (d *Document) String() string {
	return fmt.Sprintf("httpdoc.Document(url=%s loaded=%v)", d.URL(), d.Loaded())
}

func (d *Document) Method() string {
	return d.Request.Method
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

func (d *Document) SetParams(vals url.Values) {
	if d.Request.Method == "POST" {
		d.Request.PostForm = vals
	} else {
		d.Request.URL.RawQuery = vals.Encode()
	}
}

func (d *Document) SetParam(name, val string) *Document {
	if d.Request.Method == "POST" {
		d.SetPOSTParam(name, val)
	} else {
		d.SetQueryParam(name, val)
	}
	return d
}

func (d *Document) SetQueryParam(name, val string) {
	q := d.Request.URL.Query()
	q.Set(name, val)
	d.Request.URL.RawQuery = q.Encode()
}

func (d *Document) SetGETParam(name, val string) {
	d.SetQueryParam(name, val)
}

func (d *Document) SetPOSTParam(name, val string) {
	d.Request.Method = "POST"
	d.Request.PostForm.Set(name, val)
}

func (d *Document) SetPOSTParams(vals url.Values) {
	d.Request.Method = "POST"
	d.Request.PostForm = vals
}

func (d *Document) SetPOSTData(data []byte, contentType string) {
	if contentType == "" {
		contentType = "application/x-www-form-urlencoded"
	}
	d.Request.Method = "POST"
	d.Request.Header.Set("Content-Type", contentType)
	d.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))
	d.Request.ContentLength = int64(len(data))
}

func (d *Document) SetJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	d.SetPOSTData(data, "application/json")
}

func (d *Document) IsMultipartRequest() bool {
	return d.multipartWriter != nil
}

func (d *Document) setMultipartRequest() {
	if d.multipartWriter == nil {
		buf := bytes.NewBuffer(nil)
		d.Request.Method = "POST"
		d.multipartWriter = multipart.NewWriter(buf)
		d.Request.Header.Set("Content-Type", d.multipartWriter.FormDataContentType())
		d.Request.Body = ioutil.NopCloser(buf)
	}
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

func (d *Document) SetMultipartContent(name string, data io.Reader, contentType string) error {
	d.setMultipartRequest()

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeQuotes(name), escapeQuotes(name)))
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}
	if w, err := d.multipartWriter.CreatePart(h); err != nil {
		return err
	} else if _, err := io.Copy(w, data); err != nil {
		return err
	}
	return nil
}

func (d *Document) SetMultipartParams(vals url.Values) {
	d.setMultipartRequest()
	d.SetPOSTParams(vals)
}

func (d *Document) SetFile(name, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	// TODO: detect contentType by file extension
	return d.SetMultipartContent(name, file, "application/octet-stream")
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

//---------- response method --------------
func (d *Document) Loaded() bool {
	return d.Response != nil
}

func (d *Document) Load() error {
	if d.Loaded() {
		return nil
	}
	if d.multipartWriter != nil {
		for name, values := range d.Request.PostForm {
			for _, val := range values {
				d.multipartWriter.WriteField(name, val)
			}
		}
		d.multipartWriter.Close()

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
	// todo: "br" https://godoc.org/github.com/dsnet/compress/brotli
	// todo: "compress"
	// todo: "sdch"

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

func normRe(regExp interface{}) *regexp.Regexp {
	switch v := regExp.(type) {
	case *regexp.Regexp:
		return v

	case string:
		return regexp.MustCompile(v)

	default:
		panic(fmt.Sprintf("Unknown req-exp format (%v)", regExp))
	}
}

func (d *Document) Match(regExp interface{}) []string {
	return normRe(regExp).FindStringSubmatch(d.ContentStr())
}

func (d *Document) Submatch(regExp interface{}, submatchNum int) string {
	if submatches := d.Match(regExp); len(submatches) > 0 {
		return submatches[submatchNum]
	}
	return ""
}

func (d *Document) MatchAll(regExp interface{}) []string {
	return normRe(regExp).FindAllString(d.ContentStr(), -1)
}

func (d *Document) SubmatchAll(regExp interface{}) [][]string {
	return normRe(regExp).FindAllStringSubmatch(d.ContentStr(), -1)
}

func (d *Document) AllSubmatches(regExp interface{}, submatchNum int) (submatches []string) {
	for _, ss := range d.SubmatchAll(regExp) {
		submatches = append(submatches, ss[submatchNum])
	}
	return
}

func (d *Document) GetJSON(v interface{}) error {
	if err := d.Load(); err != nil {
		return err
	}
	return json.Unmarshal(d.Content(), v)
}

//--------- html-document methods ---------------
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
