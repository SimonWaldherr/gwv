// +build local

package main

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"simonwaldherr.de/go/golibs/as"
	"simonwaldherr.de/go/golibs/cachedfile"
	"simonwaldherr.de/go/golibs/gopath"
	gwv "../../gwv"
)

func Page404(w http.ResponseWriter, req *http.Request) (string, int) {
	return "These aren't the Droids your looking for", http.StatusNotFound
}

var hub *gwv.Connections

func main() {
  var stp bool = false
	dir := gopath.Dir()
	HTTPD := gwv.NewWebServer(8080, 60)

	hub = HTTPD.InitRealtimeHub()

	go func() {
		for {
			msg := <-HTTPD.LogChan
			log.Println(msg)
		}
	}()

	HTTPD.URLhandler(
  	gwv.URL("^/$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
  	  return as.String(cachedfile.Read(filepath.Join(dir, "..", "static", "sse.html"))), http.StatusOK
  	}, gwv.HTML),
		gwv.SSE("^/sse$", hub),
	)

	HTTPD.Handler404(Page404)
	HTTPD.Start()

	var i string
	for stp == false {
		_, _ = fmt.Scanf("%v", &i)
		if i == "stop" || i == "quit" {
			HTTPD.Stop()
			stp = true
		} else {
  		hub.Messages <- i
  		cc, cd := hub.ClientDetails()
  		fmt.Printf("sending \"%v\" to these %d clients: %v\n", i, cc, cd)
		}
	}

	fmt.Println("stopping")

	HTTPD.WG.Wait()

	fmt.Println("stopped")
}
