go-fetcher
==========

爬虫器(golang), 模拟浏览器特征保存cookie，referer，以达到爬虫的目的

feature
=========
1. 支持http和https, 全方位爬虫
2. 针对登录验证关键Header(cookie, referer)均自动添加，无需手工干预
3. 自动base64转码
    当遇到验证码的时候，往往需要将验证码图片返回给前端人工输入，一般的方法是将其[]byte写入文件，然后在前端页面引用这个文件，但是这个方法比较拙略，更好的办法是将其[]byte进行base64编码，然后嵌入到html代码里面，利用css将其渲染为图片。而go-fetcher提供了直接拿到Base64的接口
4. 数据缓存
    当获取的内容耗时比较久或者目标数据更新频率比较慢的时候，最为透明的做法就是使用数据缓存，而go-fetcher内置了数据缓存功能，并且可以设置缓存时间(0为不缓存)，必要的时候可以删除缓存。
5. 可序列化
    一个Fetcher实例一般代表了一个会话，如果想将其用于网页，就必须有办法在不同请求中维护一个fetcher实例，如果通过序列化的话这将非常方便，通过Store方法会将cookie/referer等数据转成文本，可以直接写入html，放入post form里面，在另一个页面读到序列化后的fetcher文本并将其反序列化。一般使用场景是验证码。

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

