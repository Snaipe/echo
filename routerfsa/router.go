package routerfsa

import (
	"strings"

	"github.com/labstack/echo/v4"
)

type Router struct {
	fsa *node
}

type node struct {
	prefix   string
	next     *node
	branch   *node
	any      bool
	match    bool
	path     string
	varSave  int
	varName  string
	method   string
	handler  echo.HandlerFunc
}

func (r *Router) Add(method, path string, h echo.HandlerFunc) {
	n := r.nodify(method, path, h)
	if r.fsa == nil {
		r.fsa = n
	} else {
		r.fsa = &node{
			next: r.fsa,
			branch: n,
		}
	}
}

func (r *Router) nodify(method, path string, h echo.HandlerFunc) *node {
	var (
		root   *node
		parent *node
		prev   int
	)
	for {
		i := strings.IndexByte(path[prev:], ':')
		if i == -1 {
			break
		}
		end := strings.IndexByte(path[prev+i:], '/')
		if end == -1 {
			end = len(path)-prev-i
		}
		name := path[prev+i:prev+i+end]

		endNode := &node{
			varSave: 1,
			varName: name,
		}

		matchAny := &node{
			any:  true,
			next: endNode,
		}
		matchAny.branch = matchAny

		startNode := &node{
			varSave: 0,
			varName: name,
			next:    matchAny,
		}

		cur := &node{
			prefix: path[prev:prev+i],
			next: startNode,
		}

		if parent != nil {
			parent.next = cur
		}
		if root == nil {
			root = cur
		}
		parent = endNode
		prev += end
	}

	if prev < len(path) {
		cur := &node{
			prefix:  path[prev:],
			match:   true,
			path:    path,
			method:  method,
			handler: h,
		}
		if parent != nil {
			parent.next = cur
		}
		if root == nil {
			root = cur
			parent = cur
		}
	}
	return root
}

func (r *Router) Find(method, path string, c echo.Context) {

	cur := []*node{r.fsa}

	type pathVarKey struct {
		name string
		key  int
	}

	pathVars := map[pathVarKey]int{}

	var exec func(cur, next []*node, i int) bool

	exec = func(cur, next []*node, i int) bool {
		for len(cur) != 0 && i < len(path) {
			for _, node := range cur {
				switch {
				case node.match && node.method == method:
					c.SetPath(node.path)
					c.SetHandler(node.handler)
					return true
				case node.any:
				case node.prefix != "":
					if !strings.HasPrefix(path[i:], node.prefix) {
						continue
					}
				case node.varName != "":
					key := pathVarKey{node.varName, node.varSave}
					old := pathVars[key]
					pathVars[key] = i
					if exec(next, nil, i) {
						return true
					}
					pathVars[key] = old
					continue
				}

				next = append(next, node.next)
				if node.branch != nil {
					next = append(next, node.branch)
				}
			}
			cur, next = next, cur[:0]
			i++
		}
		return false
	}

	exec(cur, nil, 0)
}
