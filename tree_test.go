package myhttprouter

import (
	"net/http"
	"testing"
)

var fakeHandlerValue string

func fakeHandler(val string) Handle {
	return func(http.ResponseWriter, *http.Request, Params) {
		fakeHandlerValue = val
	}
}

func TestNode_addRoute(t *testing.T) {

	tree := &Node{}

	routes := [...]string{
		"/",
		"/cmd/:tool/:sub",
		"/cmd/:tool/",
		"/search/",
		"/search/:query",
		"/user_:name",
		"/user_:name/about",
		"/files/:dir",
		"/doc/",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/info/:user/public",
		"/info/:user/project/:project",
	}
	for _, route := range routes {
		tree.addRoute(route, fakeHandler(route))
	}

	wantHandlers := []string{
		"/",
		"/cmd/tool1/abc1",
		"/user_kieren",
		"/user_kieren/about",
		"/files/root",
		"/doc/go1.html",
		"/info/kieren/public",
		"/info/kieren/project/httprouter",
		"/search/q",
	}

	for _, wantHandler := range wantHandlers {

		h, _, tsr := tree.getValue(wantHandler)
		if tsr {
			t.Fatalf("want tsr is false, but got true, fullpath=%s", wantHandler)
		}

		if h == nil {
			t.Fatalf("want handler, but got nil, fullpath=%s", wantHandler)
		}
	}

}
