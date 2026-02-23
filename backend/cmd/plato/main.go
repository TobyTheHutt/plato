package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"plato/backend/internal/httpapi"
)

var (
	runServer          = run
	makeRouter         = httpapi.NewRouter
	logPrintf          = log.Printf
	logFatalf          = log.Fatalf
	signalNotify       = signal.Notify
	signalStop         = signal.Stop
	newShutdownContext = context.WithTimeout
)

const shutdownTimeout = 30 * time.Second

func main() {
	addr := getenv("PLATO_ADDR", ":8070")

	if err := runServer(addr, makeRouter(), func(server *http.Server, listener net.Listener) error {
		return server.Serve(listener)
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

func run(addr string, handler http.Handler, start func(*http.Server, net.Listener) error, logger func(string, ...any)) error {
	if start == nil {
		return fmt.Errorf("start function is required")
	}

	server := &http.Server{
		Addr:    addr,
		Handler: handler,
		// Limits time to read request headers and reduces slowloris risk.
		ReadHeaderTimeout: 10 * time.Second,
		// Limits time to read the entire request including body to prevent slow-read attacks.
		ReadTimeout: 15 * time.Second,
		// Limits time to write responses to prevent slow clients from tying up workers.
		WriteTimeout: 15 * time.Second,
		// Limits idle keep-alive duration to prevent connection exhaustion.
		IdleTimeout: 60 * time.Second,
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer func() {
		_ = listener.Close()
	}()

	if logger != nil {
		logger("plato backend listening on %s", addr)
	}

	serveErr := make(chan error, 1)
	go func() {
		if startErr := start(server, listener); startErr != nil && !errors.Is(startErr, http.ErrServerClosed) {
			serveErr <- startErr
			return
		}
		serveErr <- nil
	}()

	quit := make(chan os.Signal, 1)
	signalNotify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signalStop(quit)

	select {
	case err := <-serveErr:
		return err
	case shutdownSignal := <-quit:
		if logger != nil {
			logger("shutdown signal received (%s), draining in-flight requests", shutdownSignal)
		}
	}

	ctx, cancel := newShutdownContext(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		if logger != nil {
			logger("server forced to shutdown: %v", err)
		}
	} else if logger != nil {
		logger("server exited gracefully")
	}

	if err := closeResources(handler); err != nil {
		if logger != nil {
			logger("resource cleanup failed: %v", err)
		}
	} else if logger != nil {
		logger("resource cleanup completed")
	}

	select {
	case err := <-serveErr:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		if logger != nil {
			logger("timed out waiting for server goroutine to exit: %v", ctx.Err())
		}
	}

	return nil
}

type closer interface {
	Close() error
}

func closeResources(handler http.Handler) error {
	if handler == nil {
		return nil
	}

	resourceCloser, ok := handler.(closer)
	if !ok {
		return nil
	}

	return resourceCloser.Close()
}
