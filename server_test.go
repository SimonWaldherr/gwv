package gwv

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

func Index(rw http.ResponseWriter, req *http.Request) (string, int) {
	return "Do or do not, there is no try", http.StatusOK
}

func Teapot(rw http.ResponseWriter, req *http.Request) (string, int) {
	return "Remember... the Force will be with you, always", http.StatusTeapot
}

func H404(rw http.ResponseWriter, req *http.Request) (string, int) {
	return "These aren't the Droids your looking for", http.StatusNotFound
}

func H500(rw http.ResponseWriter, req *http.Request) (string, int) {
	return "I have a bad feeling about this", http.StatusInternalServerError
}

func Test_Full(t *testing.T) {
	HTTPD := NewWebServer(8080, 60)
	HTTPD.ConfigSSL(4443, "ssl.key", "ssl.cert", true)

	HTTPD.URLhandler(
		StaticFiles("/static/", filepath.Join(".", "static")),
		Favicon(filepath.Join(".", "static", "favicon.ico")),
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
