package httpdoc

import (
	"bytes"
	"fmt"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"
	"io/ioutil"
	"mime"
	"net/http"
	"net/http/cookiejar"
	"net/url"
)

type Document struct {
	Client   *http.Client
	Request  *http.Request
	Response *http.Response
	rawBody  []byte
	Body     []byte
}

var DefaultHeaders = map[string]string{
	"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/55.0.2883.95 Safari/537.36",
}

func NewDocument(url string) *Document {

	jar, err := cookiejar.New(nil)
	panicOnErr(err)

	client := &http.Client{
		Jar: jar,
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}

	for name, value := range DefaultHeaders {
		req.Header.Set(name, value)
	}

	return &Document{
		Client:  client,
		Request: req,
	}
}

func (d *Document) NewDoc(relURL string) *Document {
	u, err := url.Parse(relURL)
	panicOnErr(err)
	u = d.Request.URL.ResolveReference(u)

	req, err := http.NewRequest("GET", u.String(), nil)
	panicOnErr(err)

	for name, value := range DefaultHeaders {
		req.Header.Set(name, value)
	}
	req.Header.Set("Referer", d.URL().String())

	return &Document{
		Client:  d.Client,
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

func (d *Document) QueryParams() url.Values {
	return d.Request.URL.Query()
}

func (d *Document) URL() *url.URL {
	return d.Request.URL
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

func (d *Document) SetParam(name, val string) {
	if d.Request.Method == "POST" {
		d.SetPostFormParam(name, val)
	} else {
		d.SetQueryParam(name, val)
	}
}

func (d *Document) SetQueryParam(name, val string) {
	q := d.Request.URL.Query()
	q.Set(name, val)
	d.Request.URL.RawQuery = q.Encode()
}

func (d *Document) SetPostFormParam(name, val string) {
	d.Request.Method = "POST"
	d.Request.PostForm.Set(name, val)
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

func (d *Document) load() (err error) {
	if d.Loaded() {
		return
	}
	if d.Response, err = d.Client.Do(d.Request); err != nil {
		return
	}
	defer d.Response.Body.Close()

	if d.rawBody, err = ioutil.ReadAll(d.Response.Body); err != nil {
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
	if _, p, _ := mime.ParseMediaType(sContType); p != nil {
		return p["charset"]
	}
	return "utf-8"
}

func (d *Document) Content() string {
	d.load()
	return string(d.Body)
}

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
