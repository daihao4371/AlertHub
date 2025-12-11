package main

import (
	"net/http"
	_ "net/http/pprof"
	"alertHub/initialization"
	"alertHub/internal/global"
)

var Version string

func main() {
	global.Version = Version

	go func() {
		panic(http.ListenAndServe("localhost:9999", nil))
	}()

	initialization.InitBasic()
	initialization.InitRoute()
}
