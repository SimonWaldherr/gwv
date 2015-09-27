package gwv

import (
	"encoding/json"
	"fmt"
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

func (GWV *WebServer) InitRealtimeHub() *Connections {
	var hub = &Connections{
		clients:      make(map[chan string]bool),
		clientips:    make(map[string]bool),
		addClient:    make(chan (chan string)),
		removeClient: make(chan (chan string)),
		Messages:     make(chan string),
	}
	go func() {
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
	}()
	return hub
}

func (hub *Connections) ClientDetails() (int, []string) {
	var l []string
	var i int
	for v, b := range hub.clientips {
		if b {
			l = append(l, v)
			i++
		}
	}
	return i, l
}

func SSE(re string, hub *Connections) *HandlerWrapper {
	return handlerify(re, func(rw http.ResponseWriter, req *http.Request) (string, int) {
		f, ok := rw.(http.Flusher)
		if !ok {
			http.Error(rw, "Streaming not supported!", http.StatusInternalServerError)
			return "", http.StatusNotFound
		}
		var ch = make(chan string, 16)
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
