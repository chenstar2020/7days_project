package gee

import (
	"fmt"
	"reflect"
	"testing"
)

func newTestRouter() *router{
	r := newRouter()
	r.addRoute("GET", "/", nil)
	r.addRoute("GET", "/hello/:name", nil)
	r.addRoute("GET", "/hello/b/c", nil)
	r.addRoute("GET", "/hi/:name", nil)
	r.addRoute("GET", "/assets/*filepath", nil)
	return r
}

func TestParsePattern(t *testing.T){
	ok := reflect.DeepEqual(parsePattern("/p/:name"), []string{"p", ":name"})
	ok = ok && reflect.DeepEqual(parsePattern("/p/*"), []string{"p", "*"})
	ok = ok && reflect.DeepEqual(parsePattern("/p/*name"), []string{"p", "*name"})
	if !ok {
		t.Fatalf("test parsePattern fail")
	}
}

func TestGetRoute(t *testing.T){
	r := newTestRouter()
	n, ps := r.getRoute("GET", "/assets/aaa/bbb/ccc")
	if n == nil {
		t.Fatal("nil shouldn't be returned")
	}

	if n.pattern != "/assets/*filepath"{
		t.Fatal("should match /assets/*filepath")
	}

	fmt.Println(ps)

	n, ps = r.getRoute("GET", "/hello/zzz/a")
	if n == nil {
		t.Fatal("nil shouldn't be returned")
	}

	fmt.Println(ps)
}
