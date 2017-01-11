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
		URL("^/500$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			return "500", 500
		}, PLAIN),
		URL("^/404$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			return "404", 404
		}, PLAIN),
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
	HTTPRequest("http://localhost:8080/404")
	HTTPRequest("http://localhost:8080/500")
	time.Sleep(50 * time.Millisecond)

	HTTPD.Stop()

	t.Logf("stopping")
	HTTPD.WG.Wait()
	t.Logf("stopped")
}

func Test_MimeTypes(t *testing.T) {
	var x mimeCtrl = 9
	HTTPD := NewWebServer(8081, 60)

	HTTPD.URLhandler(
		URL("^/HTML$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			return "HTML", http.StatusOK
		}, HTML),
		URL("^/PLAIN$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			return "PLAIN", http.StatusOK
		}, PLAIN),
		URL("^/AUTO$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			return "AUTO", http.StatusOK
		}, AUTO),
		URL("^/DOWNLOAD$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			return "DOWNLOAD", http.StatusOK
		}, DOWNLOAD),
		URL("^/nil$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			return "nil", http.StatusOK
		}, x),
	)

	HTTPD.Start()

	t.Logf("HTML")
	HTTPRequest("http://localhost:8081/HTML")
	t.Logf("PLAIN")
	HTTPRequest("http://localhost:8081/PLAIN")
	t.Logf("AUTO")
	HTTPRequest("http://localhost:8081/AUTO")
	t.Logf("DOWNLOAD")
	HTTPRequest("http://localhost:8081/DOWNLOAD")
	t.Logf("nil")
	HTTPRequest("http://localhost:8081/nil")
	t.Logf("stopping")

	HTTPD.Stop()
	HTTPD.WG.Wait()
}

var hub *Connections

func Test_Realtime(t *testing.T) {
	HTTPD := NewWebServer(8082, 60)

	hub = HTTPD.InitRealtimeHub()

	HTTPD.URLhandler(
		URL("^/$", Index, HTML),
		SSE("^/sse$", hub),
		SSEA("^/ssea/\\S+$"),
	)

	t.Logf("starting")
	HTTPD.Start()
	time.Sleep(2150 * time.Millisecond)
	t.Logf("started")

	HTTPRequest("http://localhost:8082/sse")
	HTTPRequest("http://localhost:8082/ssea/foobar")
	HTTPRequest("http://localhost:8082/err")

	i, ip := hub.ClientDetails()
	t.Logf("count: %v\nIP-Addresses: %v\n", i, ip)

	HTTPD.Stop()

	t.Logf("stopping")
	HTTPD.WG.Wait()
	t.Logf("stopped")
}

func Test_LogChan(t *testing.T) {
	HTTPD := NewWebServer(8083, 60)

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

	HTTPRequest("http://localhost:8083/tea")
	HTTPRequest("http://localhost:8083/")
	HTTPRequest("http://localhost:8083/err")
	time.Sleep(50 * time.Millisecond)
	HTTPRequest("http://localhost:8083/tea")

	HTTPD.Stop()

	t.Logf("stopping")
	HTTPD.WG.Wait()
	t.Logf("stopped")
}

func Test_ServerPanicRecover(t *testing.T) {
	HTTPD := NewWebServer(8084, 30)

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
	HTTPRequest("http://localhost:8084/test/")
	HTTPRequest("http://localhost:8084/")
	HTTPRequest("http://localhost:8084/test/")
	time.Sleep(50 * time.Millisecond)

	HTTPD.Stop()

	t.Logf("stopping")
	HTTPD.WG.Wait()
	t.Logf("stopped")
}

func Test_ServerStopByRequest(t *testing.T) {
	HTTPD := NewWebServer(8085, 30)

	HTTPD.URLhandler(
		URL("^/stop/$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			t.Logf("stopping")
			time.Sleep(1 * time.Second)
			HTTPD.Stop()
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
	HTTPRequest("http://localhost:8085/")
	go HTTPRequest("http://localhost:8085/stop/")
	go HTTPRequest("http://localhost:8085/stop/")
	go HTTPRequest("http://localhost:8085/stop/")

	time.Sleep(50 * time.Millisecond)
	HTTPD.WG.Wait()
	t.Logf("stopped")
}

func Test_StatusCodes(t *testing.T) {
	HTTPD := NewWebServer(8086, 10)

	HTTPD.URLhandler(
		URL("^/200$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			return "HTML", 200
		}, PLAIN),
		URL("^/300$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			return "300", 300
		}, PLAIN),
		URL("^/400$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			return "400", 400
		}, PLAIN),
		URL("^/500$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			return "500", 500
		}, PLAIN),
	)

	HTTPD.Start()

	HTTPRequest("http://localhost:8086/200")
	HTTPRequest("http://localhost:8086/300")
	HTTPRequest("http://localhost:8086/400")
	HTTPRequest("http://localhost:8086/500")

	HTTPD.Stop()
	HTTPD.WG.Wait()
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
