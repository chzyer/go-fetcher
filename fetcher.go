package fetcher

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type CacheResponse struct {
	Resp      *http.Response
	Body      []byte
	CacheTime int64
}

type Transport struct {
	tr        http.RoundTripper
	BeforeReq func(req *http.Request)
	AfterReq  func(resp *http.Response, req *http.Request)
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
	if err != nil {
		return
	}
	t.AfterReq(resp, req)
	return
}

type CustomHeader struct {
	Agent  string
	Custom map[string]string
}

func (c *CustomHeader) Set(field string, value string) {
	if c.Custom == nil {
		c.Custom = make(map[string]string)
	}
	c.Custom[field] = value
}

type Fetcher struct {
	Https     bool
	Host      string
	Referer   string
	CacheTime int64
	AutoHost  bool
	Cookies   []*http.Cookie
	Header    CustomHeader
	Client    *http.Client             `json:"-"`
	Cache     map[string]CacheResponse `json:"-"`
}

func NewFetcher(host string) (f *Fetcher) {
	f = newFetcher(nil)
	f.Host = host
	return
}

func NewFetcherHttps(host string) (f *Fetcher) {
	tr := &http.Transport{
		TLSClientConfig:    &tls.Config{RootCAs: nil, InsecureSkipVerify: true},
		DisableCompression: true,
	}
	f = newFetcher(tr)
	f.Host = host
	f.Https = true
	return
}

func newFetcher(tr http.RoundTripper) (f *Fetcher) {
	f = &Fetcher{}
	newTr := NewTransport(tr)
	newTr.AfterReq = func(resp *http.Response, req *http.Request) {
		f.mergeCookie(resp)
		f.Referer = req.URL.String()
	}
	newTr.BeforeReq = func(req *http.Request) {
		f.makeOtherHeader(req)
		f.insertCookie(req)
		f.insertReferer(req)
	}
	f.Client = &http.Client{
		Transport: newTr,
	}
	f.Cache = make(map[string]CacheResponse)
	return
}

func (f *Fetcher) GetBase64(path string) (data string, err error) {
	resp, body, err := f.Get(path)
	if err != nil {
		return
	}
	if resp.StatusCode/100 != 2 {
		if err == nil {
			return "", errors.New("fetcher: error not excepted!")
		}
		err = errors.New(err.Error())
		return
	}
	data = base64.StdEncoding.EncodeToString(body)
	return
}

func (f *Fetcher) SaveFile(path, dstPath string) (err error) {
	_, body, err := f.Get(path)
	if err != nil {
		return
	}
	ioutil.WriteFile(dstPath, body, os.ModePerm)
	return
}

func (f *Fetcher) RemoveGetCache(path string) {
	key := "get-" + "http"
	if f.Https {
		key += "s"
	}
	key += "://" + f.Host + path
	_, ok := f.Cache[key]
	if !ok {
		return
	}
	delete(f.Cache, key)
}

func (f *Fetcher) RemovePostCache(path string, params url.Values) {
	key := "post-" + path + params.Encode()
	delete(f.Cache, key)
}

func (f *Fetcher) Get(path string) (resp *http.Response, body []byte, err error) {
	path = f.makeUrl(path)
	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return
	}

	key := "get-" + path
	resp, body, ok := f.loadCache(key)
	if ok {
		return
	}
	resp, body, err = f.request(req)
	if err != nil {
		return
	}
	if f.CacheTime > 0 {
		f.saveCache(key, resp, body)
	}
	return
}

func (f *Fetcher) GetWithNoCache(path string) (resp *http.Response, body []byte, err error) {
	path = f.makeUrl(path)
	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return
	}
	key := "get-" + path
	resp, body, err = f.request(req)
	if err != nil {
		return
	}
	if f.CacheTime > 0 {
		f.saveCache(key, resp, body)
	}
	return
}

func (f *Fetcher) Post(
	path, contentType string, content io.Reader) (resp *http.Response, body []byte, err error) {

	path = f.makeUrl(path)
	req, err := http.NewRequest("POST", path, content)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", contentType)
	resp, body, err = f.request(req)
	if err != nil {
		return
	}
	f.Referer = path
	return
}

func (f *Fetcher) PostForm(path string, val url.Values) (resp *http.Response, body []byte, err error) {
	contentType := "application/x-www-form-urlencoded"
	if val == nil {
		val = url.Values{}
	}
	// key := "post-" + path + val.Encode()
	// resp, body, ok := f.loadCache(key)
	// if ok { return }
	resp, body, err = f.Post(path, contentType, strings.NewReader(val.Encode()))
	// if f.CacheTime > 0 {
	// f.saveCache(key, resp, body)
	// }
	return
}

func (f *Fetcher) PostFormRetry(
	path string, val url.Values, tryTime int) (resp *http.Response, body []byte, err error) {

	for i := 0; i < tryTime; i++ {
		resp, body, err = f.PostForm(path, val)
		if err == nil {
			break
		}
	}
	return
}

func (f *Fetcher) CallPostForm(v interface{}, path string, val url.Values) (err error) {
	_, body, err := f.PostForm(path, val)
	if err != nil {
		return
	}
	err = json.Unmarshal(body, v)
	if err != nil {
		err = errors.New("unmarshal fail: " + string(body) + ", " + err.Error())
		return
	}
	return
}

func (f *Fetcher) getCacheKey(req *http.Request) string {
	q := req.URL.Query()
	key := req.URL.Path + q.Encode()
	return key
}

func (f *Fetcher) loadCache(key string) (resp *http.Response, body []byte, ok bool) {
	r, ok := f.Cache[key]
	if !ok {
		return
	}
	ok = false
	if time.Now().Unix()-r.CacheTime > f.CacheTime {
		delete(f.Cache, key)
		return
	}
	resp, body = r.Resp, r.Body
	ok = true
	return
}

func (f *Fetcher) saveCache(key string, resp *http.Response, body []byte) {
	r := CacheResponse{
		resp, body, time.Now().Unix(),
	}
	f.Cache[key] = r
}

func (f *Fetcher) request(req *http.Request) (resp *http.Response, body []byte, err error) {
	resp, err = f.Client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	return
}

func (f *Fetcher) insertReferer(req *http.Request) (err error) {
	if f.Referer != "" {
		req.Header.Set("Referer", f.Referer)
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
		length++
	next:
		continue
	}

	f.Cookies = append(f.Cookies, newCookies[:length]...)
	return
}

func (f *Fetcher) makeUrl(path string) string {
	u := path
	idx := strings.Index(path, "://")
	if idx <= 0 {
		if f.Host != "" {
			u = f.Host + u
		}
		prefix := "http"
		if f.Https {
			prefix = "https"
		}
		u = prefix + "://" + u
	} else if uu, err := url.Parse(path); err != nil && f.AutoHost && uu.Host != "" {
		f.Host = uu.Host
	}
	return u
}

func (f *Fetcher) makeOtherHeader(req *http.Request) (err error) {
	accept := "application/json, text/javascript, */*; q=0.01"
	origin := f.makeUrl("")
	requestWith := "XMLHttpRequest"
	agent := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_8_3) "
	agent += "AppleWebKit/537.36 (KHTML, like Gecko) "
	agent += "Chrome/27.0.1453.116 Safari/537.36"
	if f.Header.Agent != "" {
		agent = f.Header.Agent
	}
	// encoding := "deflate,sdch"
	encoding := "none"
	language := "en-US,en;q=0.8"
	req.Header.Set("Accept", accept)
	req.Header.Set("Origin", origin)
	req.Header.Set("X-Requested-With", requestWith)
	req.Header.Set("User-Agent", agent)
	req.Header.Set("Accept-Encoding", encoding)
	req.Header.Set("Accept-Language", language)
	for key, val := range f.Header.Custom {
		req.Header.Set(key, val)
	}
	return
}

func (f *Fetcher) Store() (ret string, err error) {
	data, err := json.Marshal(f)
	if err != nil {
		return
	}
	ret = base64.StdEncoding.EncodeToString(data)
	return
}

func Restore(str string) (f *Fetcher, err error) {
	data, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return
	}
	f = newFetcher(nil)
	err = json.Unmarshal(data, f)
	return
}
