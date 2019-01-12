package httpdoc

import (
	"net/http"
	"net/http/cookiejar"
	"time"
)

var DefaultClient = NewClient()

func NewClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Jar:     jar,
		Timeout: 60 * time.Second,
	}
}
