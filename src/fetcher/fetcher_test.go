package fetcher

import (
	"net/url"
	"encoding/json"
	"testing"
)

func TestAliBench(t *testing.T) {
	f, err := NewFetcher("alibench.com")
	if err != nil {
		t.Error(err)
		return
	}
	
	f.Get("/")
	data := url.Values {
		"task_from": {"self"},
		"target": {"http://golang.org"},
		"ac": {"http"},
	}
	_, body, err := f.PostForm("/new_task.php", data)
	if err != nil {
		t.Fatal(err)
	}
	var a map[string] interface{}
	json.Unmarshal(body, &a)
	if a["code"].(float64) != 0 {
		t.Error("result not match", a)
	}
}
