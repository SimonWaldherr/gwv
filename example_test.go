package gwv_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"simonwaldherr.de/go/golibs/as"
	"simonwaldherr.de/go/golibs/cachedfile"
	"simonwaldherr.de/go/golibs/gopath"
	"simonwaldherr.de/go/gwv"
	"time"
)

func HTTPRequest(url string) string {
	timeout := 2 * time.Second
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println(err)
		return ""
	}
	rsp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return ""
	}
	defer rsp.Body.Close()

	if rsp.StatusCode == http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(rsp.Body)
		return string(bodyBytes)
	}
	return as.String(rsp.StatusCode)
}

func Example() {
	dir := gopath.Dir()
	server := gwv.NewWebServer(8095, 60)

	server.URLhandler(
		gwv.Robots(as.String(cachedfile.Read(filepath.Join(dir, "static", "robots.txt")))),
	)

	server.Start()

	time.Sleep(50 * time.Millisecond)

	response := HTTPRequest("http://127.0.0.1:8095/robots.txt")
	fmt.Println(response)

	server.Stop()
	server.WG.Wait()

	// Output:
	// User-agent: *
	// Disallow: /
	// Allow: /humans.txt
}
