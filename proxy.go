package httpdoc

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/denisskin/gosync"
)

func NewProxyClient(proxyAddr string) *http.Client {
	c := NewClient()
	c.Transport = NewProxyTransport(proxyAddr)
	return c
}

func NewProxyTransport(proxyAddr string) *http.Transport {
	if !strings.Contains(proxyAddr, "//") {
		proxyAddr = "//" + proxyAddr
	}
	p, err := url.Parse(proxyAddr)
	if err != nil {
		panic(err)
	}
	if p.Scheme == "" {
		p.Scheme = "http"
	}
	return &http.Transport{
		Proxy: http.ProxyURL(p),
	}
}

var proxyClientsCache = gosync.NewCache(1000)

func (d *Document) SetProxy(proxyAddr string) *Document {
	if proxyAddr != "" {
		d.Client, _ = proxyClientsCache.Get(proxyAddr).(*http.Client)
		if d.Client == nil {
			d.Client = NewProxyClient(proxyAddr)
			proxyClientsCache.Set(proxyAddr, d.Client)
		}
	}
	return d
}
