package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"plato/backend/internal/httpapi"
)

var (
	runServer  = run
	makeRouter = httpapi.NewRouter
	logPrintf  = log.Printf
	logFatalf  = log.Fatalf
)

func main() {
	addr := getenv("PLATO_ADDR", ":8070")

	if err := runServer(addr, makeRouter(), func(server *http.Server) error {
		return server.ListenAndServe()
	}, logPrintf); err != nil {
		logFatalf("server failed: %v", err)
	}
}

func getenv(key, fallback string) string {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}

	return value
}

func run(addr string, handler http.Handler, start func(*http.Server) error, logger func(string, ...any)) error {
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if start == nil {
		return fmt.Errorf("start function is required")
	}

	if logger != nil {
		logger("plato backend listening on %s", addr)
	}

	if err := start(server); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
