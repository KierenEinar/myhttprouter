package myhttprouter

import "net/http"

type Handle func(http.ResponseWriter, *http.Request, Params)

type nodeType int

const (
	nStatic nodeType = iota
	nRoot
	nParam
	nCatchAll
)

type node struct {
	Path      string
	Indices   string
	Handle    Handle
	Children  []*node
	nType     nodeType
	wildChild bool
}

type Params struct {
	Param string
	Value string
}

func longestCommonPrefix(a, b string) int {
	minLen := len(a)
	if minLen > len(b) {
		minLen = len(b)
	}
	var i = 0
	for ; i < minLen; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return i
}

func findWildChild(path string) (wildChild string, start int, valid bool) {

	for i, c := range path {

		if c != ':' && c != '*' {
			continue
		}

		for j := i + 1; j < len(path); j++ {
			if path[j] == '/' {
				return path[i:j], i, true
			}
			if path[j] == ':' || path[j] == '*' {
				return path[i:j], i, false
			}
		}
		return path[i:], i, true
	}

	return "", -1, false

}

func (n *node) addRoute(fullPath string, h Handle) {

	path := fullPath

	if len(n.Path) == 0 && len(n.Indices) == 0 {
		n.insertChild(fullPath, h)
		n.nType = nRoot
		return
	}

walk:

	for {

		i := longestCommonPrefix(n.Path, path)

		if i < len(n.Path) {

			// split current node with 2part, first part is the common prefix, second part is the different path.
			child := &node{
				Path:      n.Path[i:],
				Indices:   n.Indices,
				Handle:    n.Handle,
				Children:  n.Children,
				nType:     nStatic, // we're sure that current node must not params and catchall type.
				wildChild: n.wildChild,
			}

			n.Path = n.Path[:i]
			n.Indices = string(child.Path[0])
			n.Handle = nil
			n.Children = []*node{child}
			n.wildChild = false
		}

		if i < len(path) {

			path = path[i:]

			if n.wildChild {
				n = n.Children[0]
				if len(path) >= len(n.Path) && path[:len(n.Path)] == n.Path &&
					n.nType != nCatchAll &&
					((len(path) == len(n.Path)) || (len(path) > len(n.Path) && path[len(n.Path)] == '/')) {
					continue walk
				} else {
					panic("param segment path math an existing path which means conflict")
				}
			}

			idxc := path[0]

			for ix, c := range n.Indices {
				if byte(c) == idxc {
					n = n.Children[ix]
					continue walk
				}
			}

			if idxc != ':' && idxc != '*' {
				child := &node{}
				n.Children = append(n.Children, child)
				n.Indices += string(idxc)
				n = child
			}

			n.insertChild(path, h)
			return
		}

		if h == nil {
			panic("handler can't replace with an existing path because is nil")
		}
		n.Handle = h
		return

	}

}

func (n *node) insertChild(path string, h Handle) {

	for {

		wildcard, start, valid := findWildChild(path)
		if start < 0 {
			break
		}

		if !valid {
			panic("only one wildcard per path segment is allowed")
		}

		if len(wildcard) < 2 {
			panic("wildcards must be named with a non-empty name in path")
		}

		if len(n.Children) > 0 {
			panic("wildcard segment conflicts with existing children in path")
		}

		n.wildChild = true

		// params
		if wildcard[0] == ':' {

			if start > 0 {
				n.Path = path[:start]
				n.nType = nStatic
				path = path[start:]
			}

			child := &node{
				Path:  wildcard,
				nType: nParam,
			}

			n.Children = append(n.Children, child)
			n = child

			if len(wildcard) < len(path) {

				path = path[len(wildcard):]
				child := &node{
					nType: nStatic,
				}

				n.Indices = "/"
				n.Children = append(n.Children, child)
				n = child
				continue
			}

			n.Handle = h
			return
		}

		// catchall

		if start+len(wildcard) != len(path) {
			panic("catch-all routes are only allowed at the end of the path")
		}

		if len(n.Path) > 0 && n.Path[len(n.Path)-1] == '/' {
			panic("catch-all conflicts with existing handle for the path segment root")
		}

		if path[start-1] != '/' {
			panic("no / before catch-all")
		}

		n.Path = path[:start]
		n.nType = nStatic

		child := &node{
			Path:   wildcard,
			Handle: h,
			nType:  nCatchAll,
		}

		n.Children = append(n.Children, child)

		return

	}

	n.Path = path
	n.Handle = h

}

func (n *node) getValue(path string) (h Handle, params []Params, tsr bool) {

walk:

	for {

		prefix := n.Path

		if len(path) > len(prefix) {

			if path[:len(prefix)] == prefix {
				if !n.wildChild {
					idxc := path[len(prefix)]
					for ix, c := range n.Indices {
						if byte(c) == idxc {
							n = n.Children[ix]
							path = path[len(prefix):]
							continue walk
						}
					}
					path = path[len(prefix):]
					tsr = path == "/" && n.Handle != nil
					return
				}

				n = n.Children[0]
				path = path[len(prefix):]
				switch n.nType {

				case nParam:

					end := 0
					for ; end < len(path) && path[end] != '/'; end++ {

					}

					params = append(params, Params{
						Param: n.Path[1:],
						Value: path[:end],
					})

					if end < len(path) {
						idxc := path[end]
						for ix, c := range n.Indices {
							if byte(c) == idxc {
								n = n.Children[ix]
								path = path[end:]
								continue walk
							}
						}
						tsr = end+1 == len(path) && n.Handle != nil
						return
					}

					if h = n.Handle; h != nil {
						return
					}

					tsr = len(n.Children) == 1 && n.Children[0].Path == "/" && n.Children[0].Handle != nil

					return
				case nCatchAll:

					params = append(params, Params{
						Param: n.Path[1:],
						Value: path,
					})

					h = n.Handle
					return

				default:
					panic("invalid node type")
				}

			}

			return

		} else if path == prefix {

			if h = n.Handle; h != nil {
				return
			}

			if n.wildChild && n.Children[0].nType == nCatchAll {
				h = n.Children[0].Handle
				return
			}

			// enter for the scenario path = "/"

			// case1 before entering the param
			if path == "/" && n.wildChild && n.nType != nRoot {
				tsr = true
				return
			}
			// case2 after exiting the param

			if path == "/" && n.nType == nStatic {
				tsr = true
				return
			}

			for ix, c := range n.Indices {
				if byte(c) == '/' {
					n = n.Children[ix]
					tsr = n.Path == "/" && n.Handle != nil
					return
				}
			}

			return

		}

		// path is shorter than n.path
		//  - path is part of n.path
		//  - path not part of n.path
		// path len eq n.path len
		//  - path not eq n.path

		tsr = path == "/" || (len(path) < len(n.Path) && path == n.Path[:len(path)] &&
			n.Path[len(path):] == "/" && n.Handle != nil)

		return

	}

}
