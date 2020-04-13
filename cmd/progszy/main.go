package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jimsmart/progszy"
)

// TODO Review/improve error handling.

func main() {

	var err error

	portParam := flag.Int("port", 5595, "Port number to listen on")
	cacheParam := flag.String("cache", "./cache", "Cache location")
	proxyParam := flag.String("proxy", "", `Upstream HTTP(S) proxy URL (e.g. "http://10.0.0.1:8080")`)
	flag.Parse()

	listenAddr := ":" + strconv.Itoa(*portParam)

	cachePath := *cacheParam
	// if !filepath.IsAbs(cachePath) {
	cachePath, err = filepath.Abs(cachePath)
	if err != nil {
		fmt.Printf("Error: %s", err)
		os.Exit(1)
	}
	// }

	var proxy *url.URL
	if len(*proxyParam) > 0 {
		proxy, err = url.Parse(*proxyParam)
		if err != nil {
			fmt.Printf("Error: %s", err)
			os.Exit(1)
		}
	}

	err = progszy.Run(listenAddr, cachePath, proxy)
	if err != nil {
		fmt.Printf("Error: %s", err)
		os.Exit(1)
	}
}
