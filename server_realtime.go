package gwv

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

type Connections struct {
	clients      map[chan string]bool
	clientips    map[string]bool
	addClient    chan chan string
	removeClient chan chan string
	Messages     chan string
}

func initRealtimeHub() *Connections {
	hub := &Connections{
		clients:      make(map[chan string]bool),
		clientips:    make(map[string]bool),
		addClient:    make(chan chan string),
		removeClient: make(chan chan string),
		Messages:     make(chan string),
	}
	go hub.run()
	return hub
}

func (hub *Connections) run() {
	for {
		select {
		case s := <-hub.addClient:
			hub.clients[s] = true
		case s := <-hub.removeClient:
			delete(hub.clients, s)
		case msg := <-hub.Messages:
			for s := range hub.clients {
				s <- msg
			}
		}
	}
}

func (GWV *WebServer) InitRealtimeHub() *Connections {
	hub := &Connections{
		clients:      make(map[chan string]bool),
		clientips:    make(map[string]bool),
		addClient:    make(chan chan string),
		removeClient: make(chan chan string),
		Messages:     make(chan string),
	}
	go hub.runWithLogging(GWV)
	return hub
}

func (hub *Connections) runWithLogging(GWV *WebServer) {
	for {
		select {
		case s := <-hub.addClient:
			hub.clients[s] = true
			GWV.logChannelHandler("Added new client")
		case s := <-hub.removeClient:
			delete(hub.clients, s)
			GWV.logChannelHandler("Removed client")
		case msg := <-hub.Messages:
			for s := range hub.clients {
				s <- msg
			}
			GWV.logChannelHandler(fmt.Sprintf("Broadcast \"%v\" to %d clients", msg, len(hub.clients)))
		}
	}
}

func (hub *Connections) ClientDetails() (int, []string) {
	var l []string
	for v, b := range hub.clientips {
		if b {
			l = append(l, v)
		}
	}
	return len(l), l
}

func SSE(re string, hub *Connections) *HandlerWrapper {
	return handlerify(re, func(rw http.ResponseWriter, req *http.Request) (string, int) {
		f, ok := rw.(http.Flusher)
		if !ok {
			http.Error(rw, "Streaming not supported!", http.StatusInternalServerError)
			return "", http.StatusInternalServerError
		}

		ch := make(chan string, 16)
		hub.addClient <- ch
		hub.clientips[req.RemoteAddr] = true
		defer func() {
			hub.removeClient <- ch
			hub.clientips[req.RemoteAddr] = false
		}()

		notify := rw.(http.CloseNotifier).CloseNotify()

		rw.Header().Set("Content-Type", "text/event-stream")
		rw.Header().Set("Cache-Control", "no-cache")
		rw.Header().Set("Connection", "keep-alive")

		for i := 0; i < 1440; {
			select {
			case msg := <-ch:
				jsonData, _ := json.Marshal(msg)
				fmt.Fprintf(rw, "data: {\"str\": %s, \"time\": \"%v\"}\n\n", jsonData, time.Now())
				f.Flush()
			case <-time.After(45 * time.Second):
				fmt.Fprintf(rw, "data: {\"str\": \"No Data\"}\n\n")
				f.Flush()
				i++
			case <-notify:
				f.Flush()
				hub.removeClient <- ch
				i = 1440
			}
		}
		return "", http.StatusOK
	}, JSON)
}

var hubArray = make(map[string]*Connections)

func SSEA(re string) *HandlerWrapper {
	return handlerify(re, func(rw http.ResponseWriter, req *http.Request) (string, int) {
		requrl := fmt.Sprint(req.URL)

		if req.Method == "POST" {
			if hub, ok := hubArray[requrl]; ok {
				body, err := ioutil.ReadAll(req.Body)
				if err == nil {
					hub.Messages <- string(body)
					return "", http.StatusAccepted
				}
				return "", http.StatusBadRequest
			}
			return "", http.StatusMethodNotAllowed
		}

		f, ok := rw.(http.Flusher)
		if !ok {
			http.Error(rw, "Streaming not supported!", http.StatusInternalServerError)
			return "", http.StatusInternalServerError
		}

		if _, ok := hubArray[requrl]; !ok {
			hubArray[requrl] = initRealtimeHub()
		}

		ch := make(chan string, 16)
		hubArray[requrl].addClient <- ch
		hubArray[requrl].clientips[req.RemoteAddr] = true
		defer func() {
			hubArray[requrl].removeClient <- ch
			hubArray[requrl].clientips[req.RemoteAddr] = false
		}()

		notify := rw.(http.CloseNotifier).CloseNotify()

		rw.Header().Set("Content-Type", "text/event-stream")
		rw.Header().Set("Cache-Control", "no-cache")
		rw.Header().Set("Connection", "keep-alive")

		for i := 0; i < 1440; {
			select {
			case msg := <-ch:
				jsonData, _ := json.Marshal(msg)
				fmt.Fprintf(rw, "data: {\"str\": %s, \"time\": \"%v\"}\n\n", jsonData, time.Now())
				f.Flush()
			case <-time.After(45 * time.Second):
				fmt.Fprintf(rw, "data: {\"str\": \"No Data\"}\n\n")
				f.Flush()
				i++
			case <-notify:
				f.Flush()
				hubArray[requrl].removeClient <- ch
				i = 1440
			}
		}
		return "", http.StatusOK
	}, JSON)
}
