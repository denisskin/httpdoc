package httpdoc

import (
	"net/http"
	"net/http/cookiejar"
	"time"
	//"crypto/tls"
)

var DefaultClient = NewClient()

func NewClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Jar:     jar,
		Timeout: 60 * time.Second,
		//Transport: &http.Transport{  // use HTTP/2 by default
		//	TLSNextProto: make(map[string]func(string,  *tls.Conn) http.RoundTripper),
		//},
	}
}
