package main

import (
	"alertHub/initialization"
	"alertHub/internal/global"
	"net/http"
	_ "net/http/pprof"
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
