// +build local

package main

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"simonwaldherr.de/go/golibs/as"
	"simonwaldherr.de/go/golibs/cachedfile"
	"simonwaldherr.de/go/golibs/gopath"
	"simonwaldherr.de/go/gwv"
	"strings"
)

func copyHeader(dst, src http.Header) {
	for k, w := range src {
		for _, v := range w {
			dst.Add(k, v)
		}
	}
}

func handler(rw http.ResponseWriter, req *http.Request) (string, int) {
	fmt.Printf("%#v\n", req)
	url := strings.Replace(req.RequestURI, "/proxy/", "", 1)
	resp, _ := http.Get("https://golang.org/pkg/" + url)
	copyHeader(rw.Header(), resp.Header)
	rw.WriteHeader(resp.StatusCode)
	io.Copy(rw, resp.Body)
	return "", resp.StatusCode
}

func main() {
	dir := gopath.Dir()
	fmt.Println("DIR 1:", gopath.WD())
	fmt.Println("DIR 2:", dir)
	HTTPD := gwv.NewWebServer(8080, 60)
	HTTPD.ConfigSSL(4443, filepath.Join(dir, "..", "ssl.key"), filepath.Join(dir, "..", "ssl.cert"), true)

	HTTPD.URLhandler(
		gwv.URL("^/proxy/.*$", handler, gwv.MANUAL),
		gwv.Robots(as.String(cachedfile.Read(filepath.Join(dir, "..", "static", "robots.txt")))),
		gwv.Favicon(filepath.Join(dir, "..", "static", "favicon.ico")),
		gwv.StaticFiles("/", dir),
	)

	HTTPD.Start()
	HTTPD.WG.Wait()
}
