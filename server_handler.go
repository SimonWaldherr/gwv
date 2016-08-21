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
		break
	case PLAIN:
		rw.Header().Set("Content-Type", "text/plain")
		break
	case JSON:
		rw.Header().Set("Content-Type", "application/json")
		break
	case AUTO:
		if len(req.URL.Path) > len(route.rawre) {
			reqstr := req.URL.Path[len(route.rawre):]
			ctype := mime.TypeByExtension(filepath.Ext(reqstr))
			rw.Header().Set("Content-Type", ctype)
		} else {
			rw.Header().Set("Content-Type", "text/plain")
		}
		break
	case MANUAL:
		return
	case ICON:
		rw.Header().Set("Content-Type", "image/x-icon")
		break
	case DOWNLOAD:
		rw.Header().Set("Content-Type", "application/octet-stream")
		rw.Header().Set("Content-Disposition", "attachment")
		break
	default:
		GWV.logChannelHandler(fmt.Sprint("Unknown handler type: ", route.mime))
		break
	}

	rw.WriteHeader(code)

	switch route.mime {
	case JSON:
		err = json.NewEncoder(rw).Encode(map[string]string{
			"message": resp,
		})
	default:
		_, err = io.WriteString(rw, resp)
	}
	GWV.extendedErrorHandler("Error on WriteString to client: ", err, false)
}

func (GWV *WebServer) handle404(rw http.ResponseWriter, req *http.Request, code int) {
	var err error
	GWV.logChannelHandler(fmt.Sprint("404 on path:", req.URL.Path))

	if GWV.handler404 != nil {
		resp, _ := GWV.handler404(rw, req)
		rw.WriteHeader(code)
		_, err = io.WriteString(rw, resp)
		GWV.extendedErrorHandler("Error on WriteString to client at 404:", err, false)
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
	GWV.logChannelHandler(fmt.Sprint("500 on path:", req.URL.Path))

	if GWV.handler500 != nil {
		resp, _ := GWV.handler500(rw, req)
		rw.WriteHeader(code)
		_, err = io.WriteString(rw, resp)
		GWV.extendedErrorHandler("Error on WriteString to client at 404:", err, false)
		return
	}
	http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
	return
}

func (GWV *WebServer) Handler500(fn handler) {
	GWV.handler500 = fn
}
