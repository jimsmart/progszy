package progszy

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

// Run a server, blocking until we receive OS interrupt (ctrl-C).
func Run(addr, cachePath string) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	// TODO(js) Use this throughout?
	logger := log.New(os.Stdout, "", 0)

	// TODO(js) Create cache folder if missing?

	logger.Printf("Cache location %s\n", cachePath)

	cache := NewSqliteCache(cachePath)
	// s := NewServer(func(s *Server) { s.logger = logger })
	h := &http.Server{
		Addr:    addr,
		Handler: http.HandlerFunc(ProxyHandlerWith(cache)),
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

	err := cache.CloseAll()
	if err != nil {
		logger.Printf("Error closing cache %v\n", err)
	}

	logger.Println("Server stopped")
}
