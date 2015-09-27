package gwv

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"runtime"
	"simonwaldherr.de/go/golibs/as"
	"simonwaldherr.de/go/golibs/cachedfile"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func HTTPRequest(url string) string {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	timeout := time.Duration(2 * time.Second)
	client := &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}
	bodymsg := "lorem ipsum"
	body := bytes.NewBufferString(bodymsg)
	length := strconv.Itoa(len(bodymsg))

	req, _ := http.NewRequest("POST", url, body)
	req.Header.Add("User-Agent", "GWV-TEST")
	req.Header.Add("Content-Length", length)
	rsp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	} else {
		if rsp.StatusCode == 200 {
			bodyBytes, _ := ioutil.ReadAll(rsp.Body)
			return string(bodyBytes)
		} else if err != nil {
			fmt.Println(err)
		} else {
			return as.String(rsp.StatusCode)
		}
		rsp.Body.Close()
	}
	return ""
}

func Index(rw http.ResponseWriter, req *http.Request) (string, int) {
	return "Do or do not, there is no try", http.StatusOK
}

var messages = make(chan string)

func Teapot(rw http.ResponseWriter, req *http.Request) (string, int) {
	cn, ok := rw.(http.CloseNotifier)
	if !ok {
		return "cannot stream", http.StatusInternalServerError
	}
	go func() {
		time.Sleep(1 * time.Second)
		messages <- "Remember... the Force will be with you, always"
	}()
	select {
	case <-cn.CloseNotify():
		fmt.Println("done: closed connection")
		return "", http.StatusInternalServerError
	case msg := <-messages:
		return msg, http.StatusTeapot
	}
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

	time.Sleep(50 * time.Millisecond)
	HTTPRequest("http://localhost:8080/")
	HTTPRequest("https://localhost:4443/")
	HTTPRequest("http://localhost:8080/favicon.ico")
	HTTPRequest("http://localhost:8080/go/")
	time.Sleep(50 * time.Millisecond)

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

	HTTPRequest("http://localhost:8081/sse")
	HTTPRequest("http://localhost:8081/err")

	HTTPD.Stop = true

	t.Logf("stopping")
	HTTPD.WG.Wait()
	t.Logf("stopped")
}

func Test_LogChan(t *testing.T) {
	HTTPD := NewWebServer(8082, 60)

	hub = HTTPD.InitRealtimeHub()

	HTTPD.URLhandler(
		URL("^/$", Index, HTML),
		URL("^/tea$", Teapot, HTML),
	)
	HTTPD.InitLogChan()

	go func() {
		for {
			msg := <-HTTPD.LogChan
			t.Logf("%v\n", msg)
		}
	}()

	t.Logf("starting")
	HTTPD.Start()
	t.Logf("started")

	HTTPRequest("http://localhost:8082/tea")
	HTTPRequest("http://localhost:8082/")
	HTTPRequest("http://localhost:8082/err")
	time.Sleep(50 * time.Millisecond)
	HTTPRequest("http://localhost:8082/tea")

	HTTPD.Stop = true

	t.Logf("stopping")
	HTTPD.WG.Wait()
	t.Logf("stopped")
}

func Test_ServerPanicRecover(t *testing.T) {
	HTTPD := NewWebServer(8083, 30)

	HTTPD.URLhandler(
		URL("^/test/$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			t.Logf("everything is fine")
			return "everything is fine", http.StatusOK
		}, HTML),
		URL("^/$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			panic("panic")
			return "panic", http.StatusInternalServerError
		}, HTML),
	)

	t.Logf("starting")
	HTTPD.Start()
	t.Logf("started")

	time.Sleep(50 * time.Millisecond)
	HTTPRequest("http://localhost:8083/test/")
	HTTPRequest("http://localhost:8083/")
	HTTPRequest("http://localhost:8083/test/")
	time.Sleep(50 * time.Millisecond)

	HTTPD.Stop = true

	t.Logf("stopping")
	HTTPD.WG.Wait()
	t.Logf("stopped")
}

func Test_ServerStopByRequest(t *testing.T) {
	HTTPD := NewWebServer(8084, 30)

	HTTPD.URLhandler(
		URL("^/stop/$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			t.Logf("stopping")
			time.Sleep(1 * time.Second)
			HTTPD.Stop = true
			return "stopping", http.StatusOK
		}, HTML),
		URL("^/$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			time.Sleep(50 * time.Millisecond)
			return "", http.StatusOK
		}, HTML),
	)

	t.Logf("starting")
	HTTPD.Start()
	t.Logf("started")

	time.Sleep(50 * time.Millisecond)
	HTTPRequest("http://localhost:8084/")
	go HTTPRequest("http://localhost:8084/stop/")
	go HTTPRequest("http://localhost:8084/stop/")
	go HTTPRequest("http://localhost:8084/stop/")

	time.Sleep(50 * time.Millisecond)
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
