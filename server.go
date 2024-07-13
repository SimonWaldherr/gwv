package gwv

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http2"
	"simonwaldherr.de/go/golibs/file"
	"simonwaldherr.de/go/golibs/ssl"
)

type mimeCtrl int

const (
	AUTO mimeCtrl = iota
	HTML
	JSON
	ICON
	PLAIN
	REDIRECT
	PROXY
	DOWNLOAD
	MANUAL
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
	secureconf []sslconf
	spdy       bool
	routes     []*HandlerWrapper
	timeout    time.Duration
	handler404 handler
	handler500 handler
	WG         sync.WaitGroup
	stop       bool
	LogChan    chan string
}

type sslconf struct {
	sslkey  string
	sslcert string
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

var extensions = []string{
	"",
	".htm",
	".html",
	".shtml",
}

func StaticFiles(reqpath string, paths ...string) *HandlerWrapper {
	return handlerify(reqpath, func(rw http.ResponseWriter, req *http.Request) (string, int) {
		filename := req.URL.Path[len(reqpath):]
		for _, path := range paths {
			if strings.Contains(path, "..") {
				return "", http.StatusNotFound
			}
			for _, ext := range extensions {
				if file.IsFile(filepath.Join(path, filename) + ext) {
					http.ServeFile(rw, req, filepath.Join(path, filename)+ext)
					return "", 0
				}
			}
		}
		return "", http.StatusNotFound
	}, AUTO)
}

func Favicon(path string) *HandlerWrapper {
	data, err := file.Read(path)
	return handlerify("^/favicon.ico$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
		if err != nil {
			return "", http.StatusNotFound
		}
		return data, http.StatusOK
	}, ICON)
}

func Redirect(path, destination string, code int) *HandlerWrapper {
	return handlerify(path, func(rw http.ResponseWriter, req *http.Request) (string, int) {
		return destination, code
	}, REDIRECT)
}

func Proxy(path, destination string) *HandlerWrapper {
	re := regexp.MustCompile(path)
	return handlerify(path, func(rw http.ResponseWriter, req *http.Request) (string, int) {
		httpClient := http.Client{}

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return "", http.StatusInternalServerError
		}
		req.Body = ioutil.NopCloser(bytes.NewReader(body))

		url := fmt.Sprintf("%s%s", destination, re.ReplaceAllString(req.RequestURI, ""))
		proxyReq, err := http.NewRequest(req.Method, url, bytes.NewReader(body))
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return "", http.StatusInternalServerError
		}
		proxyReq.Header = req.Header

		resp, err := httpClient.Do(proxyReq)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadGateway)
			return "", http.StatusBadGateway
		}
		defer resp.Body.Close()

		copyHeaders(rw.Header(), resp.Header)
		rw.WriteHeader(resp.StatusCode)
		io.Copy(rw, resp.Body)

		return "", 0
	}, PROXY)
}

func copyHeaders(dst, src http.Header) {
	for k, v := range src {
		for _, vv := range v {
			dst.Add(k, vv)
		}
	}
}

func Robots(data string) *HandlerWrapper {
	return handlerify("^/robots.txt$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
		return data, http.StatusOK
	}, PLAIN)
}

func Humans(data string) *HandlerWrapper {
	return handlerify("^/humans.txt$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
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
	GWV.secureconf = append(GWV.secureconf, sslconf{sslkey: sslkey, sslcert: sslcert})
	GWV.spdy = spdy
}

func (GWV *WebServer) ConfigSSLAddCert(sslkey, sslcert string) {
	GWV.secureconf = append(GWV.secureconf, sslconf{sslkey: sslkey, sslcert: sslcert})
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
		if route.match.MatchString(request) {
			resp, status := route.handler(rw, req)
			switch status {
			case 0:
				return
			case http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusTeapot:
				GWV.handle200(rw, req, resp, route, status)
				return
			case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect:
				http.Redirect(rw, req, resp, status)
				return
			case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusMethodNotAllowed:
				GWV.handle404(rw, req, status)
				return
			case http.StatusInternalServerError, http.StatusNotImplemented, http.StatusBadGateway, http.StatusServiceUnavailable:
				GWV.handle500(rw, req, status)
				return
			}
		}
	}
	GWV.handle404(rw, req, http.StatusNotFound)
}

func GenerateSSL(options map[string]string) error {
	return ssl.Generate(options)
}

func CheckSSL(certPath string, keyPath string) error {
	return ssl.Check(certPath, keyPath)
}

func (GWV *WebServer) Start() {
	GWV.WG.Add(1)
	defer GWV.recoverFromPanic()

	httpServer := &http.Server{
		Addr:        fmt.Sprintf(":%d", GWV.port),
		Handler:     GWV,
		ReadTimeout: GWV.timeout * time.Second,
	}

	go GWV.startHTTPServer(httpServer)
	if GWV.secureport != 0 {
		go GWV.startHTTPSServer()
	}
}

func (GWV *WebServer) recoverFromPanic() {
	if r := recover(); r != nil {
		GWV.logChannelHandler(fmt.Sprintf("Recovered from panic: %v", r))
	}
}

func (GWV *WebServer) startHTTPServer(server *http.Server) {
	defer GWV.WG.Done()
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		GWV.extendedErrorHandler("Unable to start HTTP server: ", err, true)
		return
	}
	GWV.logChannelHandler(fmt.Sprintf("Serving HTTP on PORT: %d", GWV.port))

	for !GWV.stop {
		if err := server.Serve(listener); err != nil {
			GWV.extendedErrorHandler("HTTP server error: ", err, false)
		}
	}
}

func (GWV *WebServer) startHTTPSServer() {
	defer GWV.WG.Done()
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS11,
	}

	tlsConfig.Certificates = make([]tls.Certificate, len(GWV.secureconf))
	for i, conf := range GWV.secureconf {
		cert, err := tls.LoadX509KeyPair(conf.sslcert, conf.sslkey)
		if err != nil {
			GWV.extendedErrorHandler("Unable to load SSL certificate: ", err, true)
			return
		}
		tlsConfig.Certificates[i] = cert
	}
	tlsConfig.BuildNameToCertificate()

	httpsServer := &http.Server{
		Addr:        fmt.Sprintf(":%d", GWV.secureport),
		Handler:     GWV,
		ReadTimeout: GWV.timeout * time.Second,
		TLSConfig:   tlsConfig,
	}

	GWV.logChannelHandler(fmt.Sprintf("Serving HTTPS on PORT: %d", GWV.secureport))
	listener, err := tls.Listen("tcp", httpsServer.Addr, tlsConfig)
	if err != nil {
		GWV.extendedErrorHandler("Unable to start HTTPS server: ", err, true)
		return
	}
	if GWV.spdy {
		http2.ConfigureServer(httpsServer, &http2.Server{})
	}

	for !GWV.stop {
		if err := httpsServer.Serve(listener); err != nil {
			GWV.extendedErrorHandler("HTTPS server error: ", err, false)
		}
	}
}

func (GWV *WebServer) Stop() {
	if !GWV.stop {
		GWV.stop = true
		GWV.WG.Done()
	}
}
