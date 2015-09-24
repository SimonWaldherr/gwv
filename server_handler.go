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
