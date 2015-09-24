package gwv

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

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
				if GWV.LogChan != nil {
					GWV.LogChan <- fmt.Sprint("Added new client")
				}
			case s := <-hub.removeClient:
				delete(hub.clients, s)
				if GWV.LogChan != nil {
					GWV.LogChan <- fmt.Sprint("Removed client")
				}
			case msg := <-hub.messages:
				for s, _ := range hub.clients {
					s <- msg
				}
				if GWV.LogChan != nil {
					GWV.LogChan <- fmt.Sprintf("Broadcast \"%v\" to %d clients", msg, len(hub.clients))
				}
			}
		}
	}()
	return hub
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
