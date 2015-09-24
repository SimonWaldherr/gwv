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

type HandlerWrapper struct {
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
	routes     []*HandlerWrapper
	timeout    time.Duration
	handler404 handler
	handler500 handler
	WG         sync.WaitGroup
	Stop       bool
	LogChan    chan string
}

func (u *HandlerWrapper) String() string {
	return fmt.Sprintf(
		"{\n  URL: %v\n  Handler: %v\n}", u.match, u.handler,
	)
}

func handlerify(re string, handler handler, mime mimeCtrl) *HandlerWrapper {
	match := regexp.MustCompile(re)

	return &HandlerWrapper{
		match:   match,
		handler: handler,
		mime:    mime,
		rawre:   re,
	}
}

func URL(re string, view handler, handler mimeCtrl) *HandlerWrapper {
	return handlerify(re, view, handler)
}

func Download(re string, view handler) *HandlerWrapper {
	return handlerify(re, view, DOWNLOAD)
}

func StaticFiles(reqpath string, paths ...string) *HandlerWrapper {
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

func Favicon(path string) *HandlerWrapper {
	return handlerify("^/favicon.ico$",
		func(rw http.ResponseWriter, req *http.Request) (string, int) {
			data, err := file.Read(path)
			if err != nil {
				return "", http.StatusNotFound
			}
			return data, http.StatusOK
		}, ICON)
}

func Redirect(path, destination string, code int) *HandlerWrapper {
	return handlerify(path,
		func(rw http.ResponseWriter, req *http.Request) (string, int) {
			return destination, code
		}, REDIRECT)
}

func Robots(data string) *HandlerWrapper {
	return handlerify("^/robots.txt$",
		func(rw http.ResponseWriter, req *http.Request) (string, int) {
			return data, http.StatusOK
		}, PLAIN)
}

func Humans(data string) *HandlerWrapper {
	return handlerify("^/humans.txt$",
		func(rw http.ResponseWriter, req *http.Request) (string, int) {
			return data, http.StatusOK
		}, PLAIN)
}

func NewWebServer(port int, timeout time.Duration) *WebServer {
	return &WebServer{
		port:    port,
		routes:  make([]*HandlerWrapper, 0),
		timeout: timeout,
	}
}

func (GWV *WebServer) InitLogChan() {
	GWV.LogChan = make(chan string, 128)
}

func (GWV *WebServer) ConfigSSL(port int, sslkey string, sslcert string, spdy bool) {
	GWV.secureport = port
	GWV.sslkey = sslkey
	GWV.sslcert = sslcert
	GWV.spdy = spdy
}

func (GWV *WebServer) URLhandler(patterns ...*HandlerWrapper) {
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

func (GWV *WebServer) handle200(rw http.ResponseWriter, req *http.Request, resp string, route *HandlerWrapper, code int) {
	var err error
	rw.WriteHeader(code)
	switch route.mime {
	case HTML:
		rw.Header().Set("Content-type", "text/html")
		_, err = io.WriteString(rw, resp)
		break
	case PLAIN:
		rw.Header().Set("Content-type", "text/plain")
		_, err = io.WriteString(rw, resp)
		break
	case JSON:
		rw.Header().Set("Content-type", "application/json")
		err = json.NewEncoder(rw).Encode(map[string]string{
			"message": resp,
		})
		break
	case AUTO:
		reqstr := req.URL.Path[len(route.rawre):]
		ctype := mime.TypeByExtension(filepath.Ext(reqstr))
		rw.Header().Set("Content-Type", ctype)
		_, err = io.WriteString(rw, resp)
		break
	case ICON:
		rw.Header().Set("Content-Type", "image/x-icon")
		_, err = io.WriteString(rw, resp)
		break
	case DOWNLOAD:
		rw.Header().Set("Content-Type", "application/octet-stream")
		rw.Header().Set("Content-Disposition", "attachment")
		_, err = io.WriteString(rw, resp)
		break
	default:
		if GWV.LogChan != nil {
			GWV.LogChan <- fmt.Sprint("Unknown handler type: ", route.mime)
		}
		break
	}
	if err != nil && GWV.LogChan != nil {
		GWV.LogChan <- fmt.Sprint("Error on WriteString to client: ", err)
	}
}

func (GWV *WebServer) handle404(rw http.ResponseWriter, req *http.Request, code int) {
	var err error
	if GWV.LogChan != nil {
		GWV.LogChan <- fmt.Sprint("404 on path:", req.URL.Path)
	}

	if GWV.handler404 != nil {
		resp, _ := GWV.handler404(rw, req)
		rw.WriteHeader(code)
		_, err = io.WriteString(rw, resp)
		if err != nil && GWV.LogChan != nil {
			GWV.LogChan <- fmt.Sprint("Error on WriteString to client at 404:", err)
		}
		return
	}
	http.NotFound(rw, req)
	return
}

func (GWV *WebServer) Handler404(fn handler) {
	GWV.handler404 = fn
}

func (GWV *WebServer) handle500(rw http.ResponseWriter, req *http.Request, code int) {
	var err error
	if GWV.LogChan != nil {
		GWV.LogChan <- fmt.Sprint("500 on path:", req.URL.Path)
	}

	if GWV.handler500 != nil {
		resp, _ := GWV.handler500(rw, req)
		rw.WriteHeader(code)
		_, err = io.WriteString(rw, resp)
		if err != nil && GWV.LogChan != nil {
			GWV.LogChan <- fmt.Sprint("Error on WriteString to client at 404:", err)
		}
		return
	}
	http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
	return
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
		var err error
		if GWV.LogChan != nil {
			GWV.LogChan <- fmt.Sprintf("Serving HTTP on PORT: %v", GWV.port)
		}

		listener, err := net.Listen("tcp", httpServer.Addr)
		for !GWV.Stop {
			err = httpServer.Serve(listener)
			if err != nil && GWV.LogChan != nil {
				GWV.LogChan <- fmt.Sprint("can't start server:", err)
			}
		}
		if err != nil && GWV.LogChan != nil {
			GWV.LogChan <- fmt.Sprint("can't start server:", err)
		}
	}()

	if GWV.secureport != 0 {
		if GWV.sslkey == "" || GWV.sslcert == "" || CheckSSL(GWV.sslcert, GWV.sslkey) != nil {
			options := map[string]string{}
			options["certPath"] = "ssl.cert"
			options["keyPath"] = "ssl.key"
			options["host"] = "*"
			err := GenerateSSL(options)
			if err != nil && GWV.LogChan != nil {
				GWV.LogChan <- fmt.Sprint("can't generate ssl cert:", err)
			}
		}

		cert, err := tls.LoadX509KeyPair(GWV.sslcert, GWV.sslkey)
		if err != nil {
			if GWV.LogChan != nil {
				GWV.LogChan <- fmt.Sprint(err)
			}
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
			var err error
			if GWV.LogChan != nil {
				GWV.LogChan <- fmt.Sprintf("Serving HTTPS on PORT: %v", GWV.secureport)
			}

			if GWV.spdy {
				http2.ConfigureServer(&httpsServer, &http2.Server{})
			}
			listener, err := tls.Listen("tcp", httpsServer.Addr, httpsServer.TLSConfig)

			for !GWV.Stop {
				err = httpsServer.Serve(listener)
				if err != nil && GWV.LogChan != nil {
					GWV.LogChan <- fmt.Sprint("can't start server:", err)
				}
			}

			if err != nil && GWV.LogChan != nil {
				GWV.LogChan <- fmt.Sprint("can't start server:", err)
			}
		}()
	}
}
