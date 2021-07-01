/*
 * Copyright 2021 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package descriptor

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// func printChildren(n *node, prefix string) {
// 	fmt.Printf(" %02d %s%s[%d] %v %t %d \r\n", n.priority, prefix, n.path, len(n.children), n.handle, n.wildChild, n.nType)
// 	for l := len(n.path); l > 0; l-- {
// 		prefix += " "
// 	}
// 	for _, child := range n.children {
// 		printChildren(child, prefix)
// 	}
// }
func fakeHandler(val string) *FunctionDescriptor {
	return &FunctionDescriptor{Name: val}
}

type testRequests []struct {
	path       string
	nilHandler bool
	route      string
	ps         *Params
}

func getParams() *Params {
	return &Params{
		params: make([]Param, 0, 20),
	}
}

func checkRequests(t *testing.T, tree *node, requests testRequests) {
	for _, request := range requests {
		handler, psp, _ := tree.getValue(request.path, getParams)

		switch {
		case handler == nil:
			if !request.nilHandler {
				t.Errorf("handle mismatch for route '%s': Expected non-nil handle", request.path)
			}
		case request.nilHandler:
			t.Errorf("handle mismatch for route '%s': Expected nil handle", request.path)
		default:
			if handler.Name != request.route {
				t.Errorf("handle mismatch for route '%s': Wrong handle (%s != %s)", request.path, handler.Name, request.route)
			}
		}

		var ps *Params
		if psp != nil {
			ps = psp
		}

		if !reflect.DeepEqual(ps, request.ps) {
			t.Errorf("Params mismatch for route '%s'", request.path)
		}
	}
}

func checkPriorities(t *testing.T, n *node) uint32 {
	var prio uint32
	for i := range n.children {
		prio += checkPriorities(t, n.children[i])
	}

	if n.function != nil {
		prio++
	}

	if n.priority != prio {
		t.Errorf(
			"priority mismatch for node '%s': is %d, should be %d",
			n.path, n.priority, prio,
		)
	}

	return prio
}

func TestCountParams(t *testing.T) {
	if countParams("/path/:param1/static/*catch-all") != 2 {
		t.Fail()
	}
	if countParams(strings.Repeat("/:param", 256)) != 256 {
		t.Fail()
	}
}

func TestTreeAddAndGet(t *testing.T) {
	tree := &node{}

	routes := [...]string{
		"/hi",
		"/contact",
		"/co",
		"/c",
		"/a",
		"/ab",
		"/doc/",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/α",
		"/β",
	}
	for _, route := range routes {
		tree.addRoute(route, fakeHandler(route))
	}

	// printChildren(tree, "")

	checkRequests(t, tree, testRequests{
		{"/a", false, "/a", nil},
		{"/", true, "", nil},
		{"/hi", false, "/hi", nil},
		{"/contact", false, "/contact", nil},
		{"/co", false, "/co", nil},
		{"/con", true, "", nil},  // key mismatch
		{"/cona", true, "", nil}, // key mismatch
		{"/no", true, "", nil},   // no matching child
		{"/ab", false, "/ab", nil},
		{"/α", false, "/α", nil},
		{"/β", false, "/β", nil},
	})

	checkPriorities(t, tree)
}

func TestTreeWildcard(t *testing.T) {
	tree := &node{}

	routes := [...]string{
		"/",
		"/cmd/:tool/:sub",
		"/cmd/:tool/",
		"/src/*filepath",
		"/search/",
		"/search/:query",
		"/user_:name",
		"/user_:name/about",
		"/files/:dir/*filepath",
		"/doc/",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/info/:user/public",
		"/info/:user/project/:project",
	}
	for _, route := range routes {
		tree.addRoute(route, fakeHandler(route))
	}

	// printChildren(tree, "")

	checkRequests(t, tree, testRequests{
		{"/", false, "/", nil},
		{"/cmd/test/", false, "/cmd/:tool/", &Params{params: []Param{{"tool", "test"}}}},
		{"/cmd/test", true, "", &Params{params: []Param{{"tool", "test"}}}},
		{"/cmd/test/3", false, "/cmd/:tool/:sub", &Params{params: []Param{{"tool", "test"}, {"sub", "3"}}}},
		{"/src/", false, "/src/*filepath", &Params{params: []Param{{"filepath", "/"}}}},
		{"/src/some/file.png", false, "/src/*filepath", &Params{params: []Param{{"filepath", "/some/file.png"}}}},
		{"/search/", false, "/search/", nil},
		{"/search/someth!ng+in+ünìcodé", false, "/search/:query", &Params{params: []Param{{"query", "someth!ng+in+ünìcodé"}}}},
		{"/search/someth!ng+in+ünìcodé/", true, "", &Params{params: []Param{{"query", "someth!ng+in+ünìcodé"}}}},
		{"/user_gopher", false, "/user_:name", &Params{params: []Param{{"name", "gopher"}}}},
		{"/user_gopher/about", false, "/user_:name/about", &Params{params: []Param{{"name", "gopher"}}}},
		{"/files/js/inc/framework.js", false, "/files/:dir/*filepath", &Params{params: []Param{{"dir", "js"}, {"filepath", "/inc/framework.js"}}}},
		{"/info/gordon/public", false, "/info/:user/public", &Params{params: []Param{{"user", "gordon"}}}},
		{"/info/gordon/project/go", false, "/info/:user/project/:project", &Params{params: []Param{{"user", "gordon"}, {"project", "go"}}}},
	})

	checkPriorities(t, tree)
}

func catchPanic(testFunc func()) (recv interface{}) {
	defer func() {
		recv = recover()
	}()

	testFunc()
	return
}

type testRoute struct {
	path     string
	conflict bool
}

func testRoutes(t *testing.T, routes []testRoute) {
	tree := &node{}

	for i := range routes {
		route := routes[i]
		recv := catchPanic(func() {
			tree.addRoute(route.path, nil)
		})

		if route.conflict {
			if recv == nil {
				t.Errorf("no panic for conflicting route '%s'", route.path)
			}
		} else if recv != nil {
			t.Errorf("unexpected panic for route '%s': %v", route.path, recv)
		}
	}

	// printChildren(tree, "")
}

func TestTreeWildcardConflict(t *testing.T) {
	routes := []testRoute{
		{"/cmd/:tool/:sub", false},
		{"/cmd/vet", true},
		{"/src/*filepath", false},
		{"/src/*filepathx", true},
		{"/src/", true},
		{"/src1/", false},
		{"/src1/*filepath", true},
		{"/src2*filepath", true},
		{"/search/:query", false},
		{"/search/invalid", true},
		{"/user_:name", false},
		{"/user_x", true},
		{"/user_:name", false},
		{"/id:id", false},
		{"/id/:id", true},
	}
	testRoutes(t, routes)
}

func TestTreeChildConflict(t *testing.T) {
	routes := []testRoute{
		{"/cmd/vet", false},
		{"/cmd/:tool/:sub", false},
		{"/src/AUTHORS", false},
		{"/src/*filepath", true},
		{"/user_x", false},
		{"/user_:name", false},
		{"/id/:id", false},
		{"/id:id", false},
		{"/:id", false},
		{"/*filepath", true},
	}
	testRoutes(t, routes)
}

func TestTreeDupliatePath(t *testing.T) {
	tree := &node{}

	routes := [...]string{
		"/",
		"/doc/",
		"/src/*filepath",
		"/search/:query",
		"/user_:name",
	}
	for i := range routes {
		route := routes[i]
		recv := catchPanic(func() {
			tree.addRoute(route, fakeHandler(route))
		})
		if recv != nil {
			t.Fatalf("panic inserting route '%s': %v", route, recv)
		}

		// Add again
		recv = catchPanic(func() {
			tree.addRoute(route, nil)
		})
		if recv == nil {
			t.Fatalf("no panic while inserting duplicate route '%s", route)
		}
	}

	// printChildren(tree, "")

	checkRequests(t, tree, testRequests{
		{"/", false, "/", nil},
		{"/doc/", false, "/doc/", nil},
		{"/src/some/file.png", false, "/src/*filepath", &Params{params: []Param{{"filepath", "/some/file.png"}}}},
		{"/search/someth!ng+in+ünìcodé", false, "/search/:query", &Params{params: []Param{{"query", "someth!ng+in+ünìcodé"}}}},
		{"/user_gopher", false, "/user_:name", &Params{params: []Param{{"name", "gopher"}}}},
	})
}

func TestEmptyWildcardName(t *testing.T) {
	tree := &node{}

	routes := [...]string{
		"/user:",
		"/user:/",
		"/cmd/:/",
		"/src/*",
	}
	for i := range routes {
		route := routes[i]
		recv := catchPanic(func() {
			tree.addRoute(route, nil)
		})
		if recv == nil {
			t.Fatalf("no panic while inserting route with empty wildcard name '%s", route)
		}
	}
}

func TestTreeCatchAllConflict(t *testing.T) {
	routes := []testRoute{
		{"/src/*filepath/x", true},
		{"/src2/", false},
		{"/src2/*filepath/x", true},
		{"/src3/*filepath", false},
		{"/src3/*filepath/x", true},
	}
	testRoutes(t, routes)
}

func TestTreeCatchAllConflictRoot(t *testing.T) {
	routes := []testRoute{
		{"/", false},
		{"/*filepath", true},
	}
	testRoutes(t, routes)
}

func TestTreeCatchMaxParams(t *testing.T) {
	tree := &node{}
	var route = "/cmd/*filepath"
	tree.addRoute(route, fakeHandler(route))
}

func TestTreeDoubleWildcard(t *testing.T) {
	const panicMsg = "only one wildcard per path segment is allowed"

	routes := [...]string{
		"/:foo:bar",
		"/:foo:bar/",
		"/:foo*bar",
	}

	for i := range routes {
		route := routes[i]
		tree := &node{}
		recv := catchPanic(func() {
			tree.addRoute(route, nil)
		})

		if rs, ok := recv.(string); !ok || !strings.HasPrefix(rs, panicMsg) {
			t.Fatalf(`"Expected panic "%s" for route '%s', got "%v"`, panicMsg, route, recv)
		}
	}
}

func TestTreeTrailingSlashRedirect(t *testing.T) {
	tree := &node{}

	routes := [...]string{
		"/hi",
		"/b/",
		"/search/:query",
		"/cmd/:tool/",
		"/src/*filepath",
		"/x",
		"/x/y",
		"/y/",
		"/y/z",
		"/0/:id",
		"/0/:id/1",
		"/1/:id/",
		"/1/:id/2",
		"/aa",
		"/a/",
		"/admin",
		"/admin/:category",
		"/admin/:category/:page",
		"/doc",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/no/a",
		"/no/b",
		"/api/hello/:name",
		"/vendor/:x/*y",
	}
	for i := range routes {
		route := routes[i]
		recv := catchPanic(func() {
			tree.addRoute(route, fakeHandler(route))
		})
		if recv != nil {
			t.Fatalf("panic inserting route '%s': %v", route, recv)
		}
	}

	// printChildren(tree, "")

	tsrRoutes := [...]string{
		"/hi/",
		"/b",
		"/search/gopher/",
		"/cmd/vet",
		"/src",
		"/x/",
		"/y",
		"/0/go/",
		"/1/go",
		"/a",
		"/admin/",
		"/admin/config/",
		"/admin/config/permissions/",
		"/doc/",
		"/vendor/x",
	}
	for _, route := range tsrRoutes {
		handler, _, tsr := tree.getValue(route, nil)
		if handler != nil {
			t.Fatalf("non-nil handler for TSR route '%s", route)
		} else if !tsr {
			t.Errorf("expected TSR recommendation for route '%s'", route)
		}
	}

	noTsrRoutes := [...]string{
		"/",
		"/no",
		"/no/",
		"/_",
		"/_/",
		"/api/world/abc",
	}
	for _, route := range noTsrRoutes {
		handler, _, tsr := tree.getValue(route, nil)
		if handler != nil {
			t.Fatalf("non-nil handler for No-TSR route '%s", route)
		} else if tsr {
			t.Errorf("expected no TSR recommendation for route '%s'", route)
		}
	}
}

func TestTreeRootTrailingSlashRedirect(t *testing.T) {
	tree := &node{}

	recv := catchPanic(func() {
		tree.addRoute("/:test", fakeHandler("/:test"))
	})
	if recv != nil {
		t.Fatalf("panic inserting test route: %v", recv)
	}

	handler, _, tsr := tree.getValue("/", nil)
	if handler != nil {
		t.Fatalf("non-nil handler")
	} else if tsr {
		t.Errorf("expected no TSR recommendation")
	}
}

func TestTreeInvalidNodeType(t *testing.T) {
	const panicMsg = "invalid node type"

	tree := &node{}
	tree.addRoute("/", fakeHandler("/"))
	tree.addRoute("/:page", fakeHandler("/:page"))

	// set invalid node type
	tree.children[0].nType = 42

	// normal lookup
	recv := catchPanic(func() {
		tree.getValue("/test", nil)
	})
	if rs, ok := recv.(string); !ok || rs != panicMsg {
		t.Fatalf("Expected panic '"+panicMsg+"', got '%v'", recv)
	}
}

func TestTreeWildcardConflictEx(t *testing.T) {
	conflicts := [...]struct {
		route        string
		segPath      string
		existPath    string
		existSegPath string
	}{
		{"/who/are/foo", "/foo", `/who/are/\*you`, `/\*you`},
		{"/who/are/foo/", "/foo/", `/who/are/\*you`, `/\*you`},
		{"/who/are/foo/bar", "/foo/bar", `/who/are/\*you`, `/\*you`},
		{"/conxxx", "xxx", `/con:tact`, `:tact`},
		{"/conooo/xxx", "ooo", `/con:tact`, `:tact`},
	}

	for i := range conflicts {
		conflict := conflicts[i]

		// I have to re-create a 'tree', because the 'tree' will be
		// in an inconsistent state when the loop recovers from the
		// panic which threw by 'addRoute' function.
		tree := &node{}
		routes := [...]string{
			"/con:tact",
			"/who/are/*you",
			"/who/foo/hello",
		}

		for i := range routes {
			route := routes[i]
			tree.addRoute(route, fakeHandler(route))
		}

		recv := catchPanic(func() {
			tree.addRoute(conflict.route, fakeHandler(conflict.route))
		})

		if !regexp.MustCompile(fmt.Sprintf("'%s' in new path .* conflicts with existing wildcard '%s' in existing prefix '%s'", conflict.segPath, conflict.existSegPath, conflict.existPath)).MatchString(fmt.Sprint(recv)) {
			t.Fatalf("invalid wildcard conflict error (%v)", recv)
		}
	}
}
