package gwv

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/bradfitz/http2"
	"io"
	"mime"
	"net"
	"net/http"
	"path/filepath"
	"regexp"
	"simonwaldherr.de/go/golibs/as"
	"simonwaldherr.de/go/golibs/file"
	"simonwaldherr.de/go/golibs/ssl"
	"strings"
	"sync"
	"time"
)

type mimeCtrl int

const (
	AUTO mimeCtrl = iota
	HTML
	JSON
	ICON
	PLAIN
	REDIRECT
	DOWNLOAD
)

type handler func(http.ResponseWriter, *http.Request) (string, int)

type handlerWrapper struct {
	match   *regexp.Regexp
	handler handler
	mime    mimeCtrl
	rawre   string
}

type WebServer struct {
	port       int
	secureport int
	sslkey     string
	sslcert    string
	spdy       bool
	routes     []*handlerWrapper
	timeout    time.Duration
	handler404 handler
	handler500 handler
	WG         sync.WaitGroup
	Stop       bool
	LogChan    chan string
}

type Connections struct {
	clients      map[chan string]bool
	addClient    chan chan string
	removeClient chan chan string
	messages     chan string
}

func (GWV *WebServer) InitRealtimeHub() *Connections {
	var hub = &Connections{
		clients:      make(map[chan string]bool),
		addClient:    make(chan (chan string)),
		removeClient: make(chan (chan string)),
		messages:     make(chan string),
	}
	go func() {
		for {
			select {
			case s := <-hub.addClient:
				hub.clients[s] = true
				GWV.LogChan <- fmt.Sprint("Added new client")
			case s := <-hub.removeClient:
				delete(hub.clients, s)
				GWV.LogChan <- fmt.Sprint("Removed client")
			case msg := <-hub.messages:
				for s, _ := range hub.clients {
					s <- msg
				}
				GWV.LogChan <- fmt.Sprintf("Broadcast \"%v\" to %d clients", msg, len(hub.clients))
			}
		}
	}()
	return hub
}

func (u *handlerWrapper) String() string {
	return fmt.Sprintf(
		"{\n  URL: %s\n  Handler: %s\n}", u.match, u.handler,
	)
}

func handlerify(re string, handler handler, mime mimeCtrl) *handlerWrapper {
	match := regexp.MustCompile(re)

	return &handlerWrapper{
		match:   match,
		handler: handler,
		mime:    mime,
		rawre:   re,
	}
}

func URL(re string, view handler, handler mimeCtrl) *handlerWrapper {
	return handlerify(re, view, handler)
}

func SSE(re string, hub *Connections, ch chan string) *handlerWrapper {
	return handlerify(re, func(rw http.ResponseWriter, req *http.Request) (string, int) {
		f, ok := rw.(http.Flusher)
		if !ok {
			http.Error(rw, "Streaming not supported!", http.StatusInternalServerError)
			return "", http.StatusNotFound
		}

		hub.addClient <- ch
		notify := rw.(http.CloseNotifier).CloseNotify()

		rw.Header().Set("Content-Type", "text/event-stream")
		rw.Header().Set("Cache-Control", "no-cache")
		rw.Header().Set("Connection", "keep-alive")

		for i := 0; i < 1440; {
			select {
			case msg := <-ch:
				jsonData, _ := json.Marshal(msg)
				str := string(jsonData)
				fmt.Fprintf(rw, "data: {\"str\": %s, \"time\": \"%v\"}\n\n", str, time.Now())

				f.Flush()
			case <-time.After(time.Second * 45):
				fmt.Fprintf(rw, "data: {\"str\": \"No Data\"}\n\n")

				f.Flush()
				i++
			case <-notify:
				f.Flush()
				i = 1440
				hub.removeClient <- ch
			}
		}
		return "", http.StatusOK
	}, JSON)
}

func Download(re string, view handler) *handlerWrapper {
	return handlerify(re, view, DOWNLOAD)
}

func StaticFiles(reqpath string, paths ...string) *handlerWrapper {
	return handlerify(reqpath, func(rw http.ResponseWriter, req *http.Request) (string, int) {
		filename := req.URL.Path[len(reqpath):]
		for _, path := range paths {
			if strings.Count(path, "..") != 0 {
				return "", http.StatusNotFound
			}
			data, err := file.Read(filepath.Join(path, filename))
			if err != nil {
				continue
			}
			return data, http.StatusOK
		}
		return "", http.StatusNotFound
	}, AUTO)
}

func Favicon(path string) *handlerWrapper {
	return handlerify("^/favicon.ico$",
		func(rw http.ResponseWriter, req *http.Request) (string, int) {
			data, err := file.Read(path)
			if err != nil {
				return "", http.StatusNotFound
			}
			return data, http.StatusOK
		}, ICON)
}

func Redirect(path, destination string, code int) *handlerWrapper {
	return handlerify(path,
		func(rw http.ResponseWriter, req *http.Request) (string, int) {
			return destination, code
		}, REDIRECT)
}

func Robots(data string) *handlerWrapper {
	return handlerify("^/robots.txt$",
		func(rw http.ResponseWriter, req *http.Request) (string, int) {
			return data, http.StatusOK
		}, PLAIN)
}

func Humans(data string) *handlerWrapper {
	return handlerify("^/humans.txt$",
		func(rw http.ResponseWriter, req *http.Request) (string, int) {
			return data, http.StatusOK
		}, PLAIN)
}

func NewWebServer(port int, timeout time.Duration) *WebServer {
	return &WebServer{
		port:    port,
		routes:  make([]*handlerWrapper, 0),
		timeout: timeout,
		LogChan: make(chan string, 128),
	}
}

func (GWV *WebServer) ConfigSSL(port int, sslkey string, sslcert string, spdy bool) {
	GWV.secureport = port
	GWV.sslkey = sslkey
	GWV.sslcert = sslcert
	GWV.spdy = spdy
}

func (GWV *WebServer) URLhandler(patterns ...*handlerWrapper) {
	for _, url := range patterns {
		GWV.routes = append(GWV.routes, url)
	}
}

func (GWV *WebServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	GWV.WG.Add(1)
	defer GWV.WG.Done()
	request := req.URL.Path
	rw.Header().Set("Server", "GWV")

	for _, route := range GWV.routes {
		matches := route.match.FindAllStringSubmatch(request, 1)
		if len(matches) > 0 {

			resp, status := route.handler(rw, req)

			switch status {
			case 200, 201, 202, 418:
				GWV.handle200(rw, req, resp, route, status)
				return
			case 301, 302, 303, 307:
				http.Redirect(rw, req, resp, status)
				return
			case 400, 401, 403, 404:
				GWV.handle404(rw, req, status)
				return
			case 500, 501, 502, 503:
				GWV.handle500(rw, req, status)
				return

			}
		}
	}
	GWV.handle404(rw, req, http.StatusNotFound)
}

func (GWV *WebServer) handle200(rw http.ResponseWriter, req *http.Request, resp string, route *handlerWrapper, code int) {
	rw.WriteHeader(code)
	switch route.mime {
	case HTML:
		rw.Header().Set("Content-type", "text/html")
		io.WriteString(rw, resp)
		return
	case PLAIN:
		rw.Header().Set("Content-type", "text/plain")
		io.WriteString(rw, resp)
		return
	case JSON:
		rw.Header().Set("Content-type", "application/json")
		json.NewEncoder(rw).Encode(map[string]string{
			"message": resp,
		})
		return
	case AUTO:
		reqstr := req.URL.Path[len(route.rawre):]
		ctype := mime.TypeByExtension(filepath.Ext(reqstr))
		rw.Header().Set("Content-Type", ctype)
		io.WriteString(rw, resp)
		return
	case ICON:
		rw.Header().Set("Content-Type", "image/x-icon")
		io.WriteString(rw, resp)
		return
	case DOWNLOAD:
		rw.Header().Set("Content-Type", "application/octet-stream")
		rw.Header().Set("Content-Disposition", "attachment")
		io.WriteString(rw, resp)
	default:
		GWV.LogChan <- fmt.Sprint("Unknown handler type: ", route.mime)
	}
}

func (GWV *WebServer) handle404(rw http.ResponseWriter, req *http.Request, code int) {
	GWV.LogChan <- fmt.Sprint("404 on path:", req.URL.Path)

	if GWV.handler404 != nil {
		resp, _ := GWV.handler404(rw, req)
		rw.WriteHeader(code)
		io.WriteString(rw, resp)
		return
	} else {
		http.NotFound(rw, req)
		return
	}
}

func (GWV *WebServer) Handler404(fn handler) {
	GWV.handler404 = fn
}

func (GWV *WebServer) handle500(rw http.ResponseWriter, req *http.Request, code int) {
	GWV.LogChan <- fmt.Sprint("500 on path:", req.URL.Path)

	if GWV.handler500 != nil {
		resp, _ := GWV.handler500(rw, req)
		rw.WriteHeader(code)
		io.WriteString(rw, resp)
		return
	} else {
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func (GWV *WebServer) Handler500(fn handler) {
	GWV.handler500 = fn
}

func GenerateSSL(options map[string]string) error {
	return ssl.Generate(options)
}

func CheckSSL(certPath string, keyPath string) error {
	return ssl.Check(certPath, keyPath)
}

func (GWV *WebServer) Start() {
	httpServer := http.Server{
		Addr:        ":" + as.String(GWV.port),
		Handler:     GWV,
		ReadTimeout: GWV.timeout * time.Second,
	}

	go func() {
		GWV.LogChan <- fmt.Sprintf("Serving HTTP on PORT: %s", GWV.port)
		listener, err := net.Listen("tcp", httpServer.Addr)
		for !GWV.Stop {
			httpServer.Serve(listener)
		}

		if err != nil {
			GWV.LogChan <- fmt.Sprint(err)
		}
	}()

	if GWV.secureport != 0 {
		if GWV.sslkey == "" || GWV.sslcert == "" || CheckSSL(GWV.sslcert, GWV.sslkey) != nil {
			options := map[string]string{}
			options["certPath"] = "ssl.cert"
			options["keyPath"] = "ssl.key"
			options["host"] = "*"
			GenerateSSL(options)
		}

		cert, err := tls.LoadX509KeyPair(GWV.sslcert, GWV.sslkey)
		if err != nil {
			GWV.LogChan <- fmt.Sprint(err)
		}

		httpsServer := http.Server{
			Addr:        ":" + as.String(GWV.secureport),
			Handler:     GWV,
			ReadTimeout: GWV.timeout * time.Second,
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS11,
			},
		}

		go func() {
			GWV.LogChan <- fmt.Sprintf("Serving HTTPS on PORT: %s", GWV.secureport)
			if GWV.spdy {
				http2.ConfigureServer(&httpsServer, &http2.Server{})
			}
			listener, err := tls.Listen("tcp", httpsServer.Addr, httpsServer.TLSConfig)

			for !GWV.Stop {
				httpsServer.Serve(listener)
			}

			if err != nil {
				GWV.LogChan <- fmt.Sprint(err)
			}
		}()
	}
}
