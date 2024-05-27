package myhttprouter

type Router struct {
	Root                  map[string]node
	TrailingSlashRedirect bool
}
