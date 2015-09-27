package gwv

import (
	"fmt"
	"log"
)

func (GWV *WebServer) extendedErrorHandler(msg string, err error, pnc bool) {
	if err != nil {
		if GWV.LogChan != nil {
			GWV.LogChan <- fmt.Sprint(msg, err)
		} else {
			log.Print(msg, err)
		}
		if pnc {
			panic(err)
		}
	}
}

func (GWV *WebServer) logChannelHandler(msg string) {
	if GWV.LogChan != nil {
		GWV.LogChan <- msg
	} else {
		log.Print(msg)
	}
}
