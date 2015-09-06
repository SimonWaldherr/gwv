package gwv

import (
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"simonwaldherr.de/go/golibs/as"
	"simonwaldherr.de/go/golibs/cachedfile"
	"syscall"
	"testing"
	"time"
)

func Index(rw http.ResponseWriter, req *http.Request) (string, int) {
	return "Do or do not, there is no try", http.StatusOK
}

func Teapot(rw http.ResponseWriter, req *http.Request) (string, int) {
	return "Remember... the Force will be with you, always", http.StatusTeapot
}

func Golang(rw http.ResponseWriter, req *http.Request) (string, int) {
	str := fmt.Sprintf("This page is served via Golang %v<br>the process ID of the HTTP Daemon is %v<br>and it runs on a server with %v cores.", runtime.Version(), syscall.Getpid(), runtime.NumCPU())
	return str, http.StatusOK
}

func H404(rw http.ResponseWriter, req *http.Request) (string, int) {
	return "These aren't the Droids your looking for", http.StatusNotFound
}

func H500(rw http.ResponseWriter, req *http.Request) (string, int) {
	return "I have a bad feeling about this", http.StatusInternalServerError
}

func Test_Webserver(t *testing.T) {
	HTTPD := NewWebServer(8080, 60)
	HTTPD.ConfigSSL(4443, "ssl.key", "ssl.cert", true)

	HTTPD.URLhandler(
		Robots(as.String(cachedfile.Read(filepath.Join(".", "static", "robots.txt")))),
		StaticFiles("/static/", filepath.Join(".", "static")),
		Favicon(filepath.Join(".", "static", "favicon.ico")),
		Redirect("^/go/$", "/golang/", 301),
		URL("^/golang/$", Golang, HTML),
		URL("^/tea$", Teapot, HTML),
		URL("^/$", Index, HTML),
	)

	HTTPD.Handler404(H404)
	HTTPD.Handler500(H500)

	t.Logf("starting")
	HTTPD.Start()
	t.Logf("started")

	time.Sleep(100 * time.Millisecond)
	HTTPD.Stop = true

	t.Logf("stopping")
	HTTPD.WG.Wait()
	t.Logf("stopped")
}

var messageChannel = make(chan string, 16)
var hub *Connections

func Test_Realtime(t *testing.T) {
	HTTPD := NewWebServer(8081, 60)

	hub = HTTPD.InitRealtimeHub()

	HTTPD.URLhandler(
		URL("^/$", Index, HTML),
		SSE("^/sse$", hub, messageChannel),
	)

	t.Logf("starting")
	HTTPD.Start()
	t.Logf("started")

	HTTPD.Stop = true

	t.Logf("stopping")
	HTTPD.WG.Wait()
	t.Logf("stopped")
}

func Test_SSL(t *testing.T) {
	CheckSSL("ssl.cert", "ssl.key")

	options := map[string]string{}
	options["certPath"] = "ssl.cert"
	options["keyPath"] = "ssl.key"
	options["host"] = "*"
	options["countryName"] = "DE"
	options["provinceName"] = "Bavaria"
	options["organizationName"] = "Lorem Ipsum Ltd"
	options["commonName"] = "*"

	GenerateSSL(options)
}
