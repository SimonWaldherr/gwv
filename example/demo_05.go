// +build local

package main

import (
	gwv "../../gwv"
	"path/filepath"
	"simonwaldherr.de/go/golibs/gopath"
)

func main() {
	dir := gopath.Dir()
	HTTPD := gwv.NewWebServer(8080, 60)
	HTTPD.ConfigSSL(4443, filepath.Join(dir, "..", "ssl.key"), filepath.Join(dir, "..", "ssl.cert"), true)

	HTTPD.URLhandler(
		gwv.Favicon(filepath.Join(".", "static", "favicon.ico")),
		gwv.Redirect("^/go/$", "/golang/", 301),
		gwv.Proxy("^/proxy/", "http://selfcss.org/"),
		gwv.Proxy("^/golang/", "https://golang.org/"),
	)

	HTTPD.Start()
	HTTPD.WG.Wait()
}
