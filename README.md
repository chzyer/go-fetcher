go-fetcher
==========

爬虫器(golang), 模拟浏览器特征保存cookie，referer，以达到爬虫的目的

feature
=========
1. 支持http和https, 全方位爬虫
2. 针对登录验证关键Header(cookie, referer)均自动添加，无需手工干预

demo
=========
get:

```{go}
import "fetcher"
func main() {
    f, err := fetcher.NewFetcher("golang.org")
    if err != nil {
        return
    }
    resp, body, err := f.Get("/")
    if err != nil { return }
    println("status:", resp.StatusCode)
    println("body:", string(body))
}
```

post:

```{go}
import "fetcher"
func main() {
    f, err := fetcher.NewFetcher("alibench.com")
    if err != nil { return }
	
    f.Get("/") // create session
    data := url.Values {
        "task_from": {"self"},
        "target": {"http://golang.org"},
        "ac": {"http"},
    }
    _, body, err := f.PostForm("/new_task.php", data)
    if err != nil { return }
    println(string(body))
}
```
