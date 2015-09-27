# GWV β
Golang Web Valve - to be connected to your series of tubes

[![Build Status](https://travis-ci.org/SimonWaldherr/gwv.svg?branch=master)](https://travis-ci.org/SimonWaldherr/gwv) 
[![Build status](https://img.shields.io/appveyor/ci/SimonWaldherr/gwv.svg)](https://ci.appveyor.com/project/SimonWaldherr/gwv/branch/master) 
[![Coverage Status](https://coveralls.io/repos/SimonWaldherr/gwv/badge.svg?branch=master&service=github)](https://coveralls.io/github/SimonWaldherr/gwv?branch=master) 
[![codecov.io](http://codecov.io/github/SimonWaldherr/gwv/coverage.svg?branch=master)](https://codecov.io/github/SimonWaldherr/gwv?branch=master) 
[![Coverage Status](https://img.shields.io/coveralls/SimonWaldherr/gwv.svg?style=flat-square)](https://simonwaldherr.de/gocover/gwv/) 
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/SimonWaldherr/gwv/)  

## install

```go get -u -t simonwaldherr.de/go/gwv```

## features

* HTTP Server
* HTTPS Server
* SPDY/HTTP2 Server
* Static File Server
* Automatic SSL cert generator
* Realtime Webserver (SSE)
* gracefully stoppable
* channelised log

## license

[MIT (see LICENSE file)](https://github.com/SimonWaldherr/gwv/blob/master/LICENSE)

it depends on:

* [Golang Standard library](https://golang.org/pkg/#stdlib) ([BSD LICENSE](https://golang.org/LICENSE))
* [SimonWaldherr/golibs](https://github.com/SimonWaldherr/golibs) ([MIT LICENSE](https://github.com/SimonWaldherr/golibs/blob/master/LICENSE))
* [bradfitz/http2](https://github.com/bradfitz/http2) ([BSD LICENSE](https://github.com/bradfitz/http2/blob/master/LICENSE))

