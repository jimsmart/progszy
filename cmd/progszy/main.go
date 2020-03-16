package main

import (
	"flag"
	"path/filepath"
	"strconv"

	"github.com/jimsmart/progszy"
)

func main() {

	portParam := flag.Int("port", 5595, "Port number to listen on")
	cacheParam := flag.String("cache", "./cache", "Cache location")
	flag.Parse()

	listenAddr := ":" + strconv.Itoa(*portParam)

	cachePath := *cacheParam
	if !filepath.IsAbs(cachePath) {
		var err error
		cachePath, err = filepath.Abs(cachePath)
		if err != nil {
			panic(err)
		}
	}

	progszy.Run(listenAddr, cachePath)
}
