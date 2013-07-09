package fetcher

import (
	"io"
	"os"
	"log"
	"errors"
	"strings"
	"net/url"
	"net/http"
	"io/ioutil"
	"crypto/tls"
	"encoding/json"
)

type Transport struct {
	tr http.RoundTripper
	BeforeReq  func (req *http.Request)
	AfterReq func(resp *http.Response, req *http.Request)
}
func NewTransport(tr http.RoundTripper) *Transport {
	t := &Transport{}
	if tr == nil {
		tr = http.DefaultTransport
	}
	t.tr = tr
	return t
}
func (t *Transport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	t.BeforeReq(req)
	resp, err = t.tr.RoundTrip(req)
	if err != nil { return }
	t.AfterReq(resp, req)
	return
}

type Fetcher struct {
	https bool
	Host string
	referer string
	Client *http.Client
	Cookies []*http.Cookie
}

func NewFetcher(host string) (f *Fetcher, err error) {
	f = newFetcher(nil)
	f.Host = host
	return
}

func NewFetcherHttps(host string) (f *Fetcher, err error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: nil, InsecureSkipVerify: true},
		DisableCompression: true,
	}
	f = newFetcher(tr)
	f.Host = host
	f.https = true
	return
}

func newFetcher(tr http.RoundTripper) (f *Fetcher) {
	f = &Fetcher{}
	newTr := NewTransport(tr)
	newTr.AfterReq = func(resp *http.Response, req *http.Request) {
		f.mergeCookie(resp)
		f.referer = req.URL.String()
	}
	newTr.BeforeReq = func(req *http.Request) {
		f.makeOtherHeader(req)
		f.insertCookie(req)
		f.insertReferer(req)
	}
	f.Client = &http.Client{
		Transport: newTr,
	}
	return
}

func (f *Fetcher) SaveFile(path, dstPath string) (err error) {
	_, body, err := f.Get(path)
	if err != nil { return }
	ioutil.WriteFile(dstPath, body, os.ModePerm)
	return
}

func (f *Fetcher) Get(path string) (resp *http.Response, body []byte, err error) {
	path = f.makeUrl(path)
	req, err := http.NewRequest("GET", path, nil)
	if err != nil { return }
	resp, body, err = f.request(req)
	if err != nil { return }
	return
}

func (f *Fetcher) Post(
	path, contentType string, content io.Reader) (resp *http.Response, body []byte, err error) {

	path = f.makeUrl(path)
	req, err := http.NewRequest("POST", path, content)
	if err != nil { return }
	req.Header.Set("Content-Type", contentType)
	resp, body, err = f.request(req)
	if err != nil { return }
	f.referer = path
	return
}

func (f *Fetcher) PostForm(path string, val url.Values) (resp *http.Response, body []byte, err error) {
	contentType := "application/x-www-form-urlencoded"
	if val == nil {
		val = url.Values{}
	}
	resp, body, err = f.Post(path, contentType, strings.NewReader(val.Encode()))
	return
}

func (f *Fetcher) PostFormRetry(
	path string, val url.Values, tryTime int) (resp *http.Response, body []byte, err error) {

	for i:=0; i<tryTime; i++ {
		resp, body, err = f.PostForm(path, val)
		if err == nil { break }
	}
	return
}

func (f *Fetcher) CallPostForm(v interface{}, path string, val url.Values) (err error) {
	_, body, err := f.PostForm(path, val)
	if err != nil { return }
	err = json.Unmarshal(body, v)
	if err != nil {
		err = errors.New("unmarshal fail: " + string(body) + ", " + err.Error())
		return
	}
	return
}

func (f *Fetcher) request(req *http.Request) (resp *http.Response, body []byte, err error) {
	log.Println(req.Method, req.URL, req.Header.Get("Cookie"))
	resp, err = f.Client.Do(req)
	if err != nil { return }
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	return
}

func (f *Fetcher) insertReferer(req *http.Request) (err error) {
	if f.referer != "" {
		req.Header.Set("Referer", f.referer)
	}
	return
}

func (f *Fetcher) insertCookie(req *http.Request) (err error) {
	for _, cookie := range f.Cookies {
		req.AddCookie(cookie)
	}
	return
}

func (f *Fetcher) mergeCookie(resp *http.Response) (err error) {
	cookies := resp.Cookies()
	newCookies := make([]*http.Cookie, len(cookies))
	length := 0
	for _, c := range cookies {
		for idx, cs := range f.Cookies {
			if c.Name == cs.Name {
				f.Cookies[idx] = c
				goto next
			}
		}
		newCookies[length] = c
		length ++
next:
		continue
	}
	
	f.Cookies = append(f.Cookies, newCookies[:length]...)
	return
}

func (f *Fetcher) makeUrl(path string) string {
	u := path
	idx := strings.Index(path, "://")
	if idx <= 0 && f.Host != "" {
		u = f.Host + u
	}
	if idx <= 0 {
		prefix := "http"
		if f.https { prefix = "https" }
		u = prefix + "://" + u
	}
	return u
}

func (f *Fetcher) makeOtherHeader(req *http.Request) (err error) {
	accept := "application/json, text/javascript, */*; q=0.01"
	origin := f.makeUrl("")
	requestWith := "XMLHttpRequest"
	agent := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_8_3) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/27.0.1453.116 Safari/537.36"
	// encoding := "deflate,sdch"
	encoding := "none"
	language := "en-US,en;q=0.8"
	req.Header.Set("Accept", accept)
	req.Header.Set("Origin", origin)
	req.Header.Set("X-Requested-With", requestWith)
	req.Header.Set("User-Agent", agent)
	req.Header.Set("Accept-Encoding", encoding)
	req.Header.Set("Accept-Language", language)
	return
}
