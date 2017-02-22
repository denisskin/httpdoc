package httpdoc

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

type Document struct {
	Client   *http.Client
	Request  *http.Request
	Response *http.Response
	rawBody  []byte
	Body     []byte
}

var (
	DefaultHeader = http.Header{
		"Accept":          {"text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8"},
		"Accept-Encoding": {"gzip, deflate"},
		"Cache-Control":   {"max-age=0"},
		"Connection":      {"keep-alive"},
		"User-Agent":      {"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/55.0.2883.95 Safari/537.36"},
	}
)

func NewDocument(url string) *Document {
	jar, err := cookiejar.New(nil)
	panicOnErr(err)

	client := &http.Client{
		Jar: jar,
	}
	return newDocument(url, client)
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
		d.SetPostParam(name, val)
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

func (d *Document) SetPostParam(name, val string) {
	d.Request.Method = "POST"
	d.Request.PostForm.Set(name, val)
}

func (d *Document) SetPostBody(postBody []byte) {
	d.Request.Method = "POST"
	d.Request.Body = ioutil.NopCloser(bytes.NewBuffer(postBody))
}

func (d *Document) SetPostJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	d.SetPostBody(data)
}

//func (d *Document) SetCookie(val string) {
//	c := &http.Cookie{}
//	d.Client.Jar.SetCookies(d.URL(), []*http.Cookie{c})
//}

func (d *Document) Submit() error {
	return d.load()
}

//---------- response method --------------
func (d *Document) Loaded() bool {
	return d.Response != nil
}

func (d *Document) Load() {
	err := d.load()
	panicOnErr(err)
}

func (d *Document) load() (err error) {
	if d.Loaded() {
		return
	}

	// set request body
	if len(d.Request.PostForm) > 0 {
		reqBody := d.Request.PostForm.Encode()
		d.Request.Method = "POST"
		d.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		d.Request.Body = ioutil.NopCloser(bytes.NewBufferString(reqBody))
		d.Request.ContentLength = int64(len(reqBody))
	}
	if d.Request.ContentLength > 0 {
		d.Request.Header.Set("Content-Length", strconv.FormatInt(d.Request.ContentLength, 10))
	}

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
	if charset := d.Charset(); charset != "utf-8" {
		d.Body, _ = iconv(d.rawBody, charset)
	} else {
		d.Body = d.rawBody
	}
	return
}

func iconv(buf []byte, charset string) ([]byte, error) {
	enc, err := htmlindex.Get(charset)
	if err != nil {
		return nil, err
	}
	tr := transform.NewReader(bytes.NewBuffer(buf), enc.NewDecoder())
	return ioutil.ReadAll(tr)
}

func (d *Document) ContentType() string {
	d.load()
	sContType := d.Response.Header.Get("Content-Type")
	s, _, _ := mime.ParseMediaType(sContType)
	return s
}

func (d *Document) Charset() string {
	d.load()
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
	cont := d.Content()
	fmt.Printf(
		"\n======== httpdoc.Request %s ========"+
			"\nRequest URL: %s"+
			"\nRequest Method: %s"+
			"\nStatus Code: %s"+
			"\nRemote Address: %s"+
			"\nRequest Headers: %s"+
			"\nQuery String Parameters: %s"+
			"\nForm Data: %s"+
			"\nResponse Headers: %s"+
			"\nRESPONSE:\n%s"+
			"\n",
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

func (d *Document) Content() string {
	d.load()
	return string(d.Body)
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
	return normRe(regExp).FindStringSubmatch(d.Content())
}

func (d *Document) Submatch(regExp interface{}, submatchNum int) string {
	if submatches := d.Match(regExp); len(submatches) > 0 {
		return submatches[submatchNum]
	}
	return ""
}

func (d *Document) MatchAll(regExp interface{}) []string {
	return normRe(regExp).FindAllString(d.Content(), -1)
}

func (d *Document) SubmatchAll(regExp interface{}) [][]string {
	return normRe(regExp).FindAllStringSubmatch(d.Content(), -1)
}

func (d *Document) GetJSON(v interface{}) error {
	return json.Unmarshal([]byte(d.Content()), v)
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

func (d *Document) Forms() HTMLElements {
	return d.GetElementsByTagName("form")
}

func (d *Document) Links() HTMLElements {
	return d.GetElementsByTagName("a")
}

func (d *Document) Frames() HTMLElements {
	return append(d.GetElementsByTagName("iframe"), d.GetElementsByTagName("frame")...)
}

func (d *Document) Images() HTMLElements {
	return d.GetElementsByTagName("img")
}

func (d *Document) Scripts() HTMLElements {
	return d.GetElementsByTagName("script")
}
