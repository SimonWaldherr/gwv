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
)

func Page404(w http.ResponseWriter, req *http.Request) (string, int) {
	return "These aren't the Droids your looking for", http.StatusNotFound
}

func main() {
	var stp bool = false
	dir := gopath.Dir()
	HTTPD := gwv.NewWebServer(8080, 60)

	go func() {
		for {
			msg := <-HTTPD.LogChan
			log.Println(msg)
		}
	}()

	HTTPD.URLhandler(
		gwv.URL("^/$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			return as.String(cachedfile.Read(filepath.Join(dir, "..", "static", "sse2.html"))), http.StatusOK
		}, gwv.HTML),
		gwv.SSEA("^/sse/\\S+"),
	)

	HTTPD.Handler404(Page404)
	HTTPD.Start()

	var i string
	for stp == false {
		_, _ = fmt.Scanf("%v", &i)
		if i == "stop" || i == "quit" {
			HTTPD.Stop()
			stp = true
		}
	}

	fmt.Println("stopping")

	HTTPD.WG.Wait()

	fmt.Println("stopped")
}
