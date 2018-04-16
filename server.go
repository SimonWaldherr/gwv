package gwv

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"golang.org/x/net/http2"
	"io"
	"io/ioutil"
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

//URL creates a handler for a given URL, the URL can contain a regular expression
func URL(re string, view handler, handler mimeCtrl) *HandlerWrapper {
	return handlerify(re, view, handler)
}

//Download creates a handler for a given URL and sends the attachment header
func Download(re string, view handler) *HandlerWrapper {
	return handlerify(re, view, DOWNLOAD)
}

var extensions = []string{
	"",
	".htm",
	".html",
	".shtml",
}

//StaticFiles creates a handler for a given request path and a folder
func StaticFiles(reqpath string, paths ...string) *HandlerWrapper {
	return handlerify(reqpath, func(rw http.ResponseWriter, req *http.Request) (string, int) {
		filename := req.URL.Path[len(reqpath):]
		for _, path := range paths {
			if strings.Count(path, "..") != 0 {
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

//Favicon creates a handler for a favicon, its only argument is the path to the favicon file
func Favicon(path string) *HandlerWrapper {
	data, err := file.Read(path)
	return handlerify("^/favicon.ico$",
		func(rw http.ResponseWriter, req *http.Request) (string, int) {
			if err != nil {
				return "", http.StatusNotFound
			}
			return data, http.StatusOK
		}, ICON)
}

//Redirect creates a handler for HTTP redirects
func Redirect(path, destination string, code int) *HandlerWrapper {
	return handlerify(path,
		func(rw http.ResponseWriter, req *http.Request) (string, int) {
			return destination, code
		}, REDIRECT)
}

//Proxy creates proxy handler
func Proxy(path, destination string) *HandlerWrapper {
	re := regexp.MustCompile(path)
	return handlerify(path,
		func(rw http.ResponseWriter, req *http.Request) (string, int) {
			httpClient := http.Client{}

			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				http.Error(rw, err.Error(), http.StatusInternalServerError)
				return "", http.StatusInternalServerError
			}

			req.Body = ioutil.NopCloser(bytes.NewReader(body))
			url := fmt.Sprintf("%s%s", destination, re.ReplaceAllString(req.RequestURI, ""))
			proxyReq, err := http.NewRequest(req.Method, url, bytes.NewReader(body))
			proxyReq.Header = req.Header
			resp, err := httpClient.Do(proxyReq)
			if err != nil {
				http.Error(rw, err.Error(), http.StatusBadGateway)
				return "", http.StatusBadGateway
			}
			defer resp.Body.Close()

			for name, values := range resp.Header {
				rw.Header()[name] = values
			}

			rw.WriteHeader(resp.StatusCode)
			io.Copy(rw, resp.Body)

			return "", 0
		}, PROXY)
}

//Robots creates a handler for the robots.txt file
func Robots(data string) *HandlerWrapper {
	return handlerify("^/robots.txt$",
		func(rw http.ResponseWriter, req *http.Request) (string, int) {
			return data, http.StatusOK
		}, PLAIN)
}

//Humans creates a handler for the humans.txt file
func Humans(data string) *HandlerWrapper {
	return handlerify("^/humans.txt$",
		func(rw http.ResponseWriter, req *http.Request) (string, int) {
			return data, http.StatusOK
		}, PLAIN)
}

//NewWebServer returns a pointer to the webserver object
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

//ConfigSSL sets parameter for the HTTPS configuration
func (GWV *WebServer) ConfigSSL(port int, sslkey string, sslcert string, spdy bool) {
	GWV.secureport = port
	GWV.secureconf = append(GWV.secureconf, sslconf{sslkey: sslkey, sslcert: sslcert})
	GWV.spdy = spdy
}

//ConfigSSLAddCert adds additional SSL Certs (select Cert by Server Name Indication (SNI))
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
		matches := route.match.FindAllStringSubmatch(request, 1)
		if len(matches) > 0 {

			resp, status := route.handler(rw, req)

			switch status {
			case 0:
				return
			case 200, 201, 202, 418:
				GWV.handle200(rw, req, resp, route, status)
				return
			case 301, 302, 303, 307:
				http.Redirect(rw, req, resp, status)
				return
			case 400, 401, 403, 404, 405:
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

func GenerateSSL(options map[string]string) error {
	return ssl.Generate(options)
}

func CheckSSL(certPath string, keyPath string) error {
	return ssl.Check(certPath, keyPath)
}

//Start starts the web server
func (GWV *WebServer) Start() {
	GWV.WG.Add(1)
	defer func() {
		if r := recover(); r != nil {
			GWV.logChannelHandler(fmt.Sprint("Recovered in f", r))
		}
	}()
	httpServer := http.Server{
		Addr:        ":" + as.String(GWV.port),
		Handler:     GWV,
		ReadTimeout: GWV.timeout * time.Second,
	}

	go func() {
		var err error
		GWV.logChannelHandler(fmt.Sprint("Serving HTTP on PORT: ", GWV.port))

		listener, err := net.Listen("tcp", httpServer.Addr)
		for !GWV.stop {
			err = httpServer.Serve(listener)
			GWV.extendedErrorHandler("can't start server:", err, true)
		}
		GWV.extendedErrorHandler("can't start server:", err, true)
	}()

	if GWV.secureport != 0 {
		var err error
		noCert := true
		tlsConf := &tls.Config{
			MinVersion: tls.VersionTLS11,
		}
		tlsConf.Certificates = make([]tls.Certificate, len(GWV.secureconf))

		for cert := range GWV.secureconf {
			tlsConf.Certificates[cert], err = tls.LoadX509KeyPair(GWV.secureconf[cert].sslcert, GWV.secureconf[cert].sslkey)
			if err == nil {
				noCert = false
			} else {
				GWV.extendedErrorHandler("can't load key pair: ", err, true)
			}
		}
		tlsConf.BuildNameToCertificate()

		if noCert == true {
			options := map[string]string{}
			options["certPath"] = "ssl.cert"
			options["keyPath"] = "ssl.key"
			options["host"] = "*"
			err := GenerateSSL(options)
			GWV.extendedErrorHandler("can't generate ssl cert:", err, true)
		}

		httpsServer := http.Server{
			Addr:        ":" + as.String(GWV.secureport),
			Handler:     GWV,
			ReadTimeout: GWV.timeout * time.Second,
			TLSConfig:   tlsConf,
		}

		go func() {
			var err error
			GWV.logChannelHandler(fmt.Sprint("Serving HTTPS on PORT: ", GWV.secureport))

			if GWV.spdy {
				http2.ConfigureServer(&httpsServer, &http2.Server{})
			}
			listener, err := tls.Listen("tcp", httpsServer.Addr, httpsServer.TLSConfig)

			for !GWV.stop {
				err = httpsServer.Serve(listener)
				GWV.extendedErrorHandler("can't start server:", err, true)
			}

			GWV.extendedErrorHandler("can't start server:", err, true)
		}()
	}
}

//Stop stops all listeners and wait until all connections are closed
func (GWV *WebServer) Stop() {
	if !GWV.stop {
		GWV.stop = true
		GWV.WG.Done()
	}
}
