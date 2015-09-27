# GWV β
Golang Web Valve - to be connected to your series of tubes

## examples

### demo_01

This demo represents a graceful stoppable static file server.
It provides its own source code for download.  

If you run it on your local machine, then you can use the following links:

* <http://localhost:8080/wait/> waits 15 seconds, then send an empty string
* <http://localhost:8080/stop/> stops the listener, execute all still open connections and then stops the server 
* <http://localhost:8080/demo_01.go> shows the source code of itself

### demo_02

This is a [Server Sent Event](https://en.wikipedia.org/wiki/Server-sent_events) example.  
Open <http://localhost:8080/> to establish a SSE-Connection, then enter anything at the CLI where your Go program runs.

### demo_03

This is another [Server Sent Event](https://en.wikipedia.org/wiki/Server-sent_events) example.  
The difference from the previous example is, that there is a formula on the webpage (<http://localhost:8080/>) where you can enter your text and another where you can choose a "chatroom".

