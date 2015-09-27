// +build local

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"simonwaldherr.de/go/golibs/as"
	"simonwaldherr.de/go/golibs/cachedfile"
	"simonwaldherr.de/go/golibs/gopath"
	"simonwaldherr.de/go/gwv"
	"strings"
)

func absExePath() (name string, err error) {
	name = os.Args[0]

	if name[0] == '.' {
		name, err = filepath.Abs(name)
		if err == nil {
			name = filepath.Clean(name)
		}
	} else {
		name, err = exec.LookPath(filepath.Clean(name))
	}
	return
}

const (
	PathVar = "PATH"
)

func CommandPath(cmdName string) (string, error) {
	switch cmdName[0] {
	case '.', os.PathSeparator:
		wd, err := os.Getwd()
		return path.Join(wd, cmdName), err
	}
	directories := strings.Split(os.Getenv(PathVar), string(os.PathListSeparator))
	for _, directory := range directories {
		fi, err := os.Stat(path.Join(directory, cmdName))
		if err == nil && fi.Mode().IsRegular() {
			return path.Join(directory, cmdName), nil
		}
	}
	return "", fmt.Errorf("Can't find the right path for %s", cmdName)
}

func main() {
	dir := gopath.Dir()
	fmt.Println("DIR 1:", gopath.WD())
	fmt.Println("DIR 2:", dir)
	HTTPD := gwv.NewWebServer(8080, 60)
	HTTPD.ConfigSSL(4443, filepath.Join(dir, "..", "ssl.key"), filepath.Join(dir, "..", "ssl.cert"), true)

	HTTPD.URLhandler(
		gwv.Robots(as.String(cachedfile.Read(filepath.Join(dir, "..", "static", "robots.txt")))),
		gwv.Favicon(filepath.Join(dir, "..", "static", "favicon.ico")),
		gwv.StaticFiles("/", dir),
	)

	log.Print("starting")
	HTTPD.Start()
	HTTPD.WG.Add(1)
	log.Print("started")
	HTTPD.WG.Wait()
	log.Print("stopped")
}
