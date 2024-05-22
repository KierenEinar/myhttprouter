package myhttprouter

import "net/http"

type Handle func(http.ResponseWriter, *http.Request, Params)

type nodeType int

const (
	nStatic nodeType = iota
	nRoot
	nParam
)

type Node struct {
	Path      string
	Indices   string
	Handle    Handle
	Children  []*Node
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

func (n *Node) addRoute(fullPath string, h Handle) {

	// check if current node is root node
	if len(n.Path) == 0 && len(n.Indices) == 0 {
		n.insertChild(fullPath, h)
		n.nType = nRoot
		return
	}

	path := fullPath

walk:
	for {

		i := longestCommonPrefix(n.Path, path)

		if i < len(n.Path) { // split current node, because path has common prefix with n.Path[:i]

			child := &Node{
				Path:      n.Path[i:],
				Indices:   n.Indices,
				Handle:    n.Handle,
				Children:  n.Children,
				nType:     nStatic,
				wildChild: n.wildChild,
			}

			n.Path = n.Path[:i]
			n.Indices = string(child.Path[0])
			n.Handle = nil
			n.Children = []*Node{child}
			n.wildChild = false

		}

		if i < len(path) { // we split the path into path[:i] and the path[i:] part, first one is the common prefix,
			// second one is the rest and will look up the child node to insert or just insert a new node,
			//and current instructions just take care of the second one.

			path = path[i:]

			if n.wildChild { // see if the node has wildcard, because indices is empty.

				// per segment only have one param type path, so we need to check it.

				n = n.Children[0]
				if len(path) >= len(n.Path) && path[:len(n.Path)] == n.Path &&
					((len(path) > len(n.Path) && path[len(n.Path)] == '/') || (len(path) == len(n.Path))) {
					continue walk
				}

				panic("per segment only have one param type path")

			}

			idxc := path[0]

			for ix, c := range n.Indices {
				if byte(c) == idxc {
					n = n.Children[ix]
					continue walk
				}
			}

			if idxc != '*' && idxc != ':' {
				n.Indices += string(idxc)
				child := &Node{}
				n.Children = append(n.Children, child)
				n = child
			}

			n.insertChild(path, h)

			return
		}

		n.Handle = h
		return

	}

}

func (n *Node) insertChild(path string, h Handle) {

	for {

		wildcard, start, valid := findWildChild(path)
		if start < 0 {
			break
		}

		if !valid {
			panic("per segment only have one param type path")
		}

		if len(wildcard) < 2 {
			panic("invalid segment params")
		}

		if len(n.Children) > 0 {
			panic("only one wildcard segment is allowed in path '" + path + "'")
		}

		if wildcard[0] == ':' {

			if start > 0 {
				n.Path = path[:start]
				n.wildChild = true
			}
			child := &Node{
				Path:  wildcard,
				nType: nParam,
			}
			n.Children = append(n.Children, child)
			n.wildChild = true
			n = child
			path = path[start+len(wildcard):]
			if len(path) > 0 {
				n.Indices += string(path[0])
				child = &Node{}
				n.Children = append(n.Children, child)
				n = child
				continue
			}

			n.Handle = h
			return

		}

	}

	n.Path = path
	n.Handle = h
}

func (n *Node) getValue(path string) (h Handle, params []Params, tsr bool) {

walk:
	for {

		prefix := n.Path
		/**
		case1: search path len > prefix len (which we should deal look up from parent to child)
		case2: search path len == prefix len (which we should compare the prefix and the path)
		case3: search path len < prefix len
		*/
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
					tsr = len(path) == len(prefix)+1 && idxc == '/' && n.Handle != nil
					return
				}

				// handle wild child
				n = n.Children[0]
				path = path[len(prefix):]

				switch n.nType {
				case nParam:

					// iterate the path, and see it has loop until the end or match '/' to stop.
					end := 0
					// /abc -> end=4
					// /abc/ -> end=5
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
						if end+1 == len(path) && idxc == '/' && n.Handle != nil {
							tsr = true
						}
						return
					}

					if h = n.Handle; h != nil {
						return
					}

					// no handle found !!!
					/**
					  case1:
					  -- /admin
					        --- /
					          ---- :config
					            ----- /
					  search /admin/config
					*/
					tsr = len(n.Children) == 1 && n.Children[0].Path == "/" && n.Children[0].Handle != nil
					return

				default:
					return
				}

			}

			return

		} else if prefix == path {

			if h = n.Handle; h != nil {
				return
			}

			// no handle found !!!
			// check if node is wildcard, there has two case
			/**
			case1:
			/admin
			/admin/:config
			/admin/:config/:name
			-- /admin
			 --- /
			  ---- :config
				----- /
			      ------ :name
			search /admin/config/
			*/
			if path == "/" && n.wildChild && n.nType == nParam {
				tsr = true
				return
			}

			/**
			case2:
			-- /admin
			 --- /
			  ---- :config
			   ----- /
			    ----- abc
			    ----- cdf
			/admin
			/admin/:config/
			/admin/:config/abc
			/admin/:config/cdf
			search /admin/area/

			*/

			if path == "/" && n.nType == nStatic {
				tsr = true
				return
			}

			// see if we have the same path with prefix, and next child is "/" and handle not nil
			for _, child := range n.Children {
				if child.Path == "/" && child.Handle != nil {
					tsr = true
					return
				}
			}

			return
		}

		// path is shortest then prefix

		// /admin/
		// /admin

		// -- /
		//  --- abc
		//  --- ccc
		// /abc /ccc

		// /x
		// /x/y
		// search query /x/

		tsr = path == "/" || (len(prefix) == len(path)+1 && prefix[:len(path)] == path &&
			prefix[len(path)] == '/' && n.Handle != nil)
		return

	}

}
