package gwv

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
)

func (GWV *WebServer) handle200(rw http.ResponseWriter, req *http.Request, resp string, route *HandlerWrapper, code int) {
	var err error

	switch route.mime {
	case HTML:
		rw.Header().Set("Content-Type", "text/html")
	case PLAIN:
		rw.Header().Set("Content-Type", "text/plain")
	case JSON:
		rw.Header().Set("Content-Type", "application/json")
	case AUTO:
		if len(req.URL.Path) > len(route.rawre) {
			reqstr := req.URL.Path[len(route.rawre):]
			ctype := mime.TypeByExtension(filepath.Ext(reqstr))
			rw.Header().Set("Content-Type", ctype)
		} else {
			rw.Header().Set("Content-Type", "text/plain")
		}
	case ICON:
		rw.Header().Set("Content-Type", "image/x-icon")
	case DOWNLOAD:
		rw.Header().Set("Content-Type", "application/octet-stream")
		rw.Header().Set("Content-Disposition", "attachment")
	case MANUAL:
		return
	default:
		GWV.logChannelHandler(fmt.Sprintf("Unknown mime type: %v", route.mime))
	}

	rw.WriteHeader(code)

	if route.mime == JSON {
		err = json.NewEncoder(rw).Encode(map[string]string{
			"message": resp,
		})
	} else {
		_, err = io.WriteString(rw, resp)
	}

	if err != nil {
		GWV.extendedErrorHandler("Error writing response: ", err, false)
	}
}

func (GWV *WebServer) handle404(rw http.ResponseWriter, req *http.Request, code int) {
	GWV.logChannelHandler(fmt.Sprintf("404 on path: %s", req.URL.Path))

	if GWV.handler404 != nil {
		resp, _ := GWV.handler404(rw, req)
		rw.WriteHeader(code)
		if _, err := io.WriteString(rw, resp); err != nil {
			GWV.extendedErrorHandler("Error writing 404 response: ", err, false)
		}
		return
	}
	http.NotFound(rw, req)
}

func (GWV *WebServer) handle500(rw http.ResponseWriter, req *http.Request, code int) {
	GWV.logChannelHandler(fmt.Sprintf("500 on path: %s", req.URL.Path))

	if GWV.handler500 != nil {
		resp, _ := GWV.handler500(rw, req)
		rw.WriteHeader(code)
		if _, err := io.WriteString(rw, resp); err != nil {
			GWV.extendedErrorHandler("Error writing 500 response: ", err, false)
		}
		return
	}
	http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
}

func (GWV *WebServer) Handler404(fn handler) {
	GWV.handler404 = fn
}

func (GWV *WebServer) Handler500(fn handler) {
	GWV.handler500 = fn
}
