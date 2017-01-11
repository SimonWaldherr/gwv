// +build local

package main

import (
	gwv "../../gwv"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"simonwaldherr.de/go/golibs/as"
	"simonwaldherr.de/go/golibs/cachedfile"
	"simonwaldherr.de/go/golibs/gopath"
	"time"
)

func main() {
	dir := gopath.Dir()
	fmt.Println("DIR 1:", gopath.WD())
	fmt.Println("DIR 2:", dir)
	HTTPD := gwv.NewWebServer(8080, 60)
	HTTPD.ConfigSSL(4443, filepath.Join(dir, "..", "ssl.key"), filepath.Join(dir, "..", "ssl.cert"), true)

	HTTPD.URLhandler(
		gwv.URL("^/wait/?$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			for i := 15; i > 0; i-- {
				time.Sleep(1 * time.Second)
				log.Printf("waiting for %v seconds\n", i)
			}
			return "", http.StatusOK
		}, gwv.HTML),
		gwv.URL("^/stop/?$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			HTTPD.Stop()
			return "", http.StatusOK
		}, gwv.HTML),
		gwv.Robots(as.String(cachedfile.Read(filepath.Join(dir, "..", "static", "robots.txt")))),
		gwv.Favicon(filepath.Join(dir, "..", "static", "favicon.ico")),
		gwv.StaticFiles("/", dir),
	)

	HTTPD.Start()
	HTTPD.WG.Wait()
}
