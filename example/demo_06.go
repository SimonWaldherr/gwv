// +build local

package main

import (
	gwv "../../gwv"
	"fmt"
	"net/http"
	"path/filepath"
	"simonwaldherr.de/go/golibs/as"
	"simonwaldherr.de/go/golibs/cache"
	"simonwaldherr.de/go/golibs/cachedfile"
	"simonwaldherr.de/go/golibs/gopath"
	"sync/atomic"
	"time"
)

type CookieStruct struct {
	Id    uint64
	Text  string
	Login time.Time
}

func main() {
	var i int
	var ops uint64

	cache := cache.New(24*time.Hour, 15*time.Minute)

	cookieKey := make([]byte, 16)
	//rand.Read(cookieKey)
	cookieKey = []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 42}
	cookieCrypt := gwv.NewSimpleCryptor(cookieKey, "cookieName")

	dir := gopath.Dir()

	HTTPD := gwv.NewWebServer(8080, 60)
	HTTPD.ConfigSSL(4443, filepath.Join(dir, "..", "ssl.key"), filepath.Join(dir, "..", "ssl.cert"), true)

	HTTPD.URLhandler(
		gwv.Robots(as.String(cachedfile.Read(filepath.Join(dir, "..", "static", "robots.txt")))),
		gwv.Favicon(filepath.Join(dir, "..", "static", "favicon.ico")),
		gwv.URL("^.*$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			cookie := &CookieStruct{}
			cookieCrypt.Read(cookie, req)

			i++

			if cookie.Text == "" {
				uniqueId := atomic.LoadUint64(&ops)
				cookieCrypt.Write(&CookieStruct{
					Id:    uniqueId,
					Text:  fmt.Sprintf("your first Request was %d", i),
					Login: time.Now(),
				}, rw, req)

				cache.Set(fmt.Sprint(uniqueId), "some information (not saved in the cookie, but linked via a unique id)")
			}

			return fmt.Sprintf("Request #%d<br>\n%s<br>\nat %s", i, cookie.Text, cookie.Login), http.StatusOK
		}, gwv.HTML),
	)

	HTTPD.Start()
	HTTPD.WG.Wait()
}
