package progszy

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"
)

// Run a server, blocking until we receive OS interrupt (ctrl-C).
func Run(addr, cachePath string, proxy *url.URL) error {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	// TODO(js) Use this throughout?
	logger := log.New(os.Stderr, "", 0)

	// TODO(js) Create cache folder if missing?

	// TODO Startup messages - to log or fmt.Print/stdout?

	stat, err := os.Stat(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Printf("Cache folder does not exist %s\n", cachePath)
		}
		return err
	}
	if !stat.IsDir() {
		logger.Printf("Cache location must be a folder %s\n", cachePath)
		return fmt.Errorf("Location not a folder %s", cachePath)
	}
	logger.Printf("Cache location %s\n", cachePath)

	if proxy != nil {
		logger.Printf("Upstream proxy %s\n", proxy.String())
	}

	cache := NewSqliteCache(cachePath)
	// s := NewServer(func(s *Server) { s.logger = logger })
	h := &http.Server{
		Addr: "127.0.0.1" + addr,
		// Handler: http.HandlerFunc(ProxyHandlerWith(cache)),
		Handler: ProxyHandlerWith(cache, proxy),
	}

	go func() {
		logger.Printf("Listening on port %s\n", addr[1:])
		if err := h.ListenAndServe(); err != http.ErrServerClosed {
			err2 := cache.CloseAll()
			if err2 != nil {
				logger.Printf("Error closing cache %v\n", err2)
			}
			logger.Fatal(err)
		}
	}()

	// Wait for interrupt signal.
	<-stop

	logger.Println("\nStopping the server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	h.Shutdown(ctx)

	err = cache.CloseAll()
	if err != nil {
		logger.Printf("Error closing cache %v\n", err)
	}

	logger.Println("Server stopped")
	return err
}
