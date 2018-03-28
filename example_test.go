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
	timeout := time.Duration(2 * time.Second)
	client := &http.Client{Timeout: timeout}
	req, _ := http.NewRequest("GET", url, nil)
	rsp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	} else {
		if rsp.StatusCode == 200 {
			bodyBytes, _ := ioutil.ReadAll(rsp.Body)
			return string(bodyBytes)
		} else if err != nil {
			fmt.Println(err)
		} else {
			return as.String(rsp.StatusCode)
		}
		rsp.Body.Close()
	}
	return ""
}

func Example() {
	dir := gopath.Dir()
	HTTPD := gwv.NewWebServer(8095, 60)

	HTTPD.URLhandler(
		gwv.Robots(as.String(cachedfile.Read(filepath.Join(dir, "static", "robots.txt")))),
	)

	HTTPD.Start()

	time.Sleep(50 * time.Millisecond)

	str := HTTPRequest("http://127.0.0.1:8095/robots.txt")
	fmt.Println(str)

	HTTPD.Stop()
	HTTPD.WG.Wait()

	// Output:
	// User-agent: *
	// Disallow: /
	// Allow: /humans.txt
}
