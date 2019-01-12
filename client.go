package httpdoc

import (
	"net/http"
	"net/http/cookiejar"
	"time"
)

var DefaultClient = NewClient()

func NewClient() *http.Client {
	jar, err := cookiejar.New(nil)
	panicOnErr(err)
	return &http.Client{
		Jar:     jar,
		Timeout: 60 * time.Second,
	}
}
