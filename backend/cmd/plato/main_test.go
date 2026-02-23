package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"plato/backend/internal/httpapi"
)

func TestGetenv(t *testing.T) {
	t.Setenv("PLATO_ADDR", ":9000")
	if got := getenv("PLATO_ADDR", ":8070"); got != ":9000" {
		t.Fatalf("expected :9000 got %s", got)
	}

	t.Setenv("PLATO_ADDR", "")
	if got := getenv("PLATO_ADDR", ":8070"); got != ":8070" {
		t.Fatalf("expected fallback got %s", got)
	}
}

func TestRun(t *testing.T) {
	handler := http.NewServeMux()
	loggerCalled := false
	addr := "127.0.0.1:0"

	if err := run(addr, handler, func(server *http.Server, _ net.Listener) error {
		if server.Addr != addr {
			t.Fatalf("unexpected server addr %s", server.Addr)
		}
		if server.Handler != handler {
			t.Fatalf("unexpected server handler")
		}
		if server.ReadHeaderTimeout != 10*time.Second {
			t.Fatalf("expected read header timeout 10s, got %v", server.ReadHeaderTimeout)
		}
		if server.ReadTimeout != 15*time.Second {
			t.Fatalf("expected read timeout 15s, got %v", server.ReadTimeout)
		}
		if server.WriteTimeout != 15*time.Second {
			t.Fatalf("expected write timeout 15s, got %v", server.WriteTimeout)
		}
		if server.IdleTimeout != 60*time.Second {
			t.Fatalf("expected idle timeout 60s, got %v", server.IdleTimeout)
		}
		return nil
	}, func(_ string, _ ...any) {
		loggerCalled = true
	}); err != nil {
		t.Fatalf("expected run success, got %v", err)
	}
	if !loggerCalled {
		t.Fatal("expected logger callback to be called")
	}

	if err := run(addr, handler, func(_ *http.Server, _ net.Listener) error {
		return http.ErrServerClosed
	}, nil); err != nil {
		t.Fatalf("expected nil on server closed, got %v", err)
	}

	expected := errors.New("boom")
	if err := run(addr, handler, func(_ *http.Server, _ net.Listener) error {
		return expected
	}, nil); !errors.Is(err, expected) {
		t.Fatalf("expected propagated error, got %v", err)
	}

	if err := run(addr, handler, nil, nil); err == nil {
		t.Fatal("expected error for nil start function")
	}
}

func TestRunGracefulShutdownCallsCleanup(t *testing.T) {
	previousSignalNotify := signalNotify
	previousSignalStop := signalStop
	t.Cleanup(func() {
		signalNotify = previousSignalNotify
		signalStop = previousSignalStop
	})

	registeredSignalChannel := make(chan chan<- os.Signal, 1)
	signalNotify = func(c chan<- os.Signal, _ ...os.Signal) {
		registeredSignalChannel <- c
	}
	signalStop = func(chan<- os.Signal) {}

	startRelease := make(chan struct{})
	handler := &testClosableHandler{Handler: http.NewServeMux()}
	logs := make(chan string, 16)

	runErrors := make(chan error, 1)
	go func() {
		runErrors <- run("127.0.0.1:0", handler, func(_ *http.Server, _ net.Listener) error {
			<-startRelease
			return http.ErrServerClosed
		}, func(format string, args ...any) {
			logs <- fmt.Sprintf(format, args...)
		})
	}()

	signalChannel := <-registeredSignalChannel
	signalChannel <- syscall.SIGTERM
	close(startRelease)

	if err := <-runErrors; err != nil {
		t.Fatalf("expected graceful shutdown to return nil, got %v", err)
	}
	if !handler.closed {
		t.Fatal("expected handler cleanup to run on shutdown")
	}
	logEntries := drainLogChannel(logs)
	if !logsContain(logEntries, "shutdown signal received") {
		t.Fatalf("expected shutdown log message, got %v", logEntries)
	}
	if !logsContain(logEntries, "resource cleanup completed") {
		t.Fatalf("expected cleanup completion log message, got %v", logEntries)
	}
}

func TestRunLogsTimeoutWaitingForServerGoroutine(t *testing.T) {
	previousSignalNotify := signalNotify
	previousSignalStop := signalStop
	previousNewShutdownContext := newShutdownContext
	t.Cleanup(func() {
		signalNotify = previousSignalNotify
		signalStop = previousSignalStop
		newShutdownContext = previousNewShutdownContext
	})

	registeredSignalChannel := make(chan chan<- os.Signal, 1)
	signalNotify = func(c chan<- os.Signal, _ ...os.Signal) {
		registeredSignalChannel <- c
	}
	signalStop = func(chan<- os.Signal) {}
	newShutdownContext = func(parent context.Context, _ time.Duration) (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(parent)
		cancel()
		return ctx, func() {}
	}

	startRelease := make(chan struct{})
	logs := make(chan string, 16)
	runErrors := make(chan error, 1)
	go func() {
		runErrors <- run("127.0.0.1:0", http.NewServeMux(), func(_ *http.Server, _ net.Listener) error {
			<-startRelease
			return nil
		}, func(format string, args ...any) {
			logs <- fmt.Sprintf(format, args...)
		})
	}()

	signalChannel := <-registeredSignalChannel
	signalChannel <- syscall.SIGTERM

	if err := <-runErrors; err != nil {
		t.Fatalf("expected shutdown path to return nil, got %v", err)
	}
	close(startRelease)

	logEntries := drainLogChannel(logs)
	if !logsContain(logEntries, "timed out waiting for server goroutine to exit") {
		t.Fatalf("expected timeout waiting for server goroutine log message, got %v", logEntries)
	}
}

func TestRunLogsForcedShutdownWhenSlowRequestExceedsGracePeriod(t *testing.T) {
	previousSignalNotify := signalNotify
	previousSignalStop := signalStop
	previousNewShutdownContext := newShutdownContext
	t.Cleanup(func() {
		signalNotify = previousSignalNotify
		signalStop = previousSignalStop
		newShutdownContext = previousNewShutdownContext
	})

	registeredSignalChannel := make(chan chan<- os.Signal, 1)
	signalNotify = func(c chan<- os.Signal, _ ...os.Signal) {
		registeredSignalChannel <- c
	}
	signalStop = func(chan<- os.Signal) {}
	newShutdownContext = func(parent context.Context, _ time.Duration) (context.Context, context.CancelFunc) {
		return context.WithTimeout(parent, 10*time.Millisecond)
	}

	slowRequestStarted := make(chan struct{})
	releaseSlowRequest := make(chan struct{})
	router := http.NewServeMux()
	router.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	router.HandleFunc("/slow", func(w http.ResponseWriter, _ *http.Request) {
		close(slowRequestStarted)
		<-releaseSlowRequest
		w.WriteHeader(http.StatusOK)
	})

	logs := make(chan string, 16)
	runErrors := make(chan error, 1)
	listenAddr := make(chan string, 1)
	go func() {
		runErrors <- run("127.0.0.1:0", router, func(server *http.Server, listener net.Listener) error {
			listenAddr <- listener.Addr().String()
			return server.Serve(listener)
		}, func(format string, args ...any) {
			logs <- fmt.Sprintf(format, args...)
		})
	}()

	signalChannel := <-registeredSignalChannel
	baseURL := "http://" + <-listenAddr
	waitForReady(t, baseURL+"/ready")
	go func() {
		client := &http.Client{Timeout: 2 * time.Second}
		resp, requestErr := doGetRequest(client, baseURL+"/slow")
		if requestErr == nil {
			_ = resp.Body.Close()
		}
	}()

	<-slowRequestStarted
	signalChannel <- syscall.SIGTERM
	waitForLogMessage(t, logs, "server forced to shutdown")

	if err := <-runErrors; err != nil {
		t.Fatalf("expected shutdown flow to return nil after timeout, got %v", err)
	}
	close(releaseSlowRequest)
}

func TestRunReturnsServeErrorAfterShutdownSignal(t *testing.T) {
	previousSignalNotify := signalNotify
	previousSignalStop := signalStop
	t.Cleanup(func() {
		signalNotify = previousSignalNotify
		signalStop = previousSignalStop
	})

	registeredSignalChannel := make(chan chan<- os.Signal, 1)
	signalNotify = func(c chan<- os.Signal, _ ...os.Signal) {
		registeredSignalChannel <- c
	}
	signalStop = func(chan<- os.Signal) {}

	startRelease := make(chan struct{})
	expected := errors.New("serve failure")
	runErrors := make(chan error, 1)
	go func() {
		runErrors <- run("127.0.0.1:0", http.NewServeMux(), func(_ *http.Server, _ net.Listener) error {
			<-startRelease
			return expected
		}, nil)
	}()

	signalChannel := <-registeredSignalChannel
	signalChannel <- syscall.SIGINT
	close(startRelease)

	if err := <-runErrors; !errors.Is(err, expected) {
		t.Fatalf("expected serve error %v after shutdown, got %v", expected, err)
	}
}

func TestRunAllowsInFlightRequestAndRejectsNewRequestsOnShutdown(t *testing.T) {
	previousSignalNotify := signalNotify
	previousSignalStop := signalStop
	t.Cleanup(func() {
		signalNotify = previousSignalNotify
		signalStop = previousSignalStop
	})

	registeredSignalChannel := make(chan chan<- os.Signal, 1)
	signalNotify = func(c chan<- os.Signal, _ ...os.Signal) {
		registeredSignalChannel <- c
	}
	signalStop = func(chan<- os.Signal) {}

	slowRequestStarted := make(chan struct{})
	releaseSlowRequest := make(chan struct{})
	router := http.NewServeMux()
	router.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	router.HandleFunc("/slow", func(w http.ResponseWriter, _ *http.Request) {
		close(slowRequestStarted)
		<-releaseSlowRequest
		w.WriteHeader(http.StatusOK)
		if _, writeErr := w.Write([]byte("ok")); writeErr != nil {
			return
		}
	})

	logs := make(chan string, 16)
	runErrors := make(chan error, 1)
	listenAddr := make(chan string, 1)
	go func() {
		runErrors <- run("127.0.0.1:0", router, func(server *http.Server, listener net.Listener) error {
			listenAddr <- listener.Addr().String()
			return server.Serve(listener)
		}, func(format string, args ...any) {
			logs <- fmt.Sprintf(format, args...)
		})
	}()

	signalChannel := <-registeredSignalChannel
	baseURL := "http://" + <-listenAddr
	waitForReady(t, baseURL+"/ready")

	slowResponseErrors := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 2 * time.Second}
		resp, requestErr := doGetRequest(client, baseURL+"/slow")
		if requestErr != nil {
			slowResponseErrors <- requestErr
			return
		}
		defer resp.Body.Close()
		if _, readErr := io.ReadAll(resp.Body); readErr != nil {
			slowResponseErrors <- fmt.Errorf("read slow response body: %w", readErr)
			return
		}
		if resp.StatusCode != http.StatusOK {
			slowResponseErrors <- fmt.Errorf("expected slow response 200, got %d", resp.StatusCode)
			return
		}
		slowResponseErrors <- nil
	}()

	<-slowRequestStarted
	signalChannel <- syscall.SIGINT
	waitForLogMessage(t, logs, "shutdown signal received")

	if err := waitForRequestRejection(baseURL + "/ready"); err != nil {
		t.Fatalf("expected new requests to be rejected after shutdown signal: %v", err)
	}

	close(releaseSlowRequest)
	if err := <-slowResponseErrors; err != nil {
		t.Fatalf("expected in-flight request to complete successfully: %v", err)
	}
	if err := <-runErrors; err != nil {
		t.Fatalf("expected graceful shutdown to complete without error, got %v", err)
	}
}

func TestCloseResources(t *testing.T) {
	if err := closeResources(nil); err != nil {
		t.Fatalf("expected nil for nil handler, got %v", err)
	}

	if err := closeResources(http.NewServeMux()); err != nil {
		t.Fatalf("expected nil for non-closable handler, got %v", err)
	}

	expected := errors.New("close failed")
	handler := &testClosableHandler{
		Handler:  http.NewServeMux(),
		closeErr: expected,
	}
	if err := closeResources(handler); !errors.Is(err, expected) {
		t.Fatalf("expected close error %v, got %v", expected, err)
	}
	if !handler.closed {
		t.Fatal("expected close to be called on closable handler")
	}
}

func TestRunLogsCleanupFailure(t *testing.T) {
	previousSignalNotify := signalNotify
	previousSignalStop := signalStop
	t.Cleanup(func() {
		signalNotify = previousSignalNotify
		signalStop = previousSignalStop
	})

	registeredSignalChannel := make(chan chan<- os.Signal, 1)
	signalNotify = func(c chan<- os.Signal, _ ...os.Signal) {
		registeredSignalChannel <- c
	}
	signalStop = func(chan<- os.Signal) {}

	startRelease := make(chan struct{})
	logs := make(chan string, 16)
	handler := &testClosableHandler{
		Handler:  http.NewServeMux(),
		closeErr: errors.New("cleanup failed"),
	}
	runErrors := make(chan error, 1)
	go func() {
		runErrors <- run("127.0.0.1:0", handler, func(_ *http.Server, _ net.Listener) error {
			<-startRelease
			return nil
		}, func(format string, args ...any) {
			logs <- fmt.Sprintf(format, args...)
		})
	}()

	signalChannel := <-registeredSignalChannel
	signalChannel <- syscall.SIGTERM
	close(startRelease)

	if err := <-runErrors; err != nil {
		t.Fatalf("expected shutdown to continue when cleanup fails, got %v", err)
	}
	logEntries := drainLogChannel(logs)
	if !logsContain(logEntries, "resource cleanup failed") {
		t.Fatalf("expected cleanup failure to be logged, got %v", logEntries)
	}
}

func TestMainUsesRunServerAndExitHandler(t *testing.T) {
	previousRunServer := runServer
	previousMakeRouter := makeRouter
	previousLoadRuntimeConfig := loadRuntimeConfig
	previousLogPrintf := logPrintf
	previousExitProcess := exitProcess
	t.Cleanup(func() {
		runServer = previousRunServer
		makeRouter = previousMakeRouter
		loadRuntimeConfig = previousLoadRuntimeConfig
		logPrintf = previousLogPrintf
		exitProcess = previousExitProcess
	})

	loadRuntimeConfig = func() (httpapi.RuntimeConfig, error) {
		return httpapi.RuntimeConfig{Mode: httpapi.RuntimeModeProduction}, nil
	}
	t.Setenv("PLATO_ADDR", ":8123")
	makeRouter = func(httpapi.RuntimeConfig) (http.Handler, error) {
		return http.NewServeMux(), nil
	}

	runCalled := false
	runServer = func(addr string, handler http.Handler, start func(*http.Server, net.Listener) error, logger func(string, ...any)) error {
		runCalled = true
		if addr != ":8123" {
			t.Fatalf("expected main to pass env addr, got %s", addr)
		}
		if handler == nil || start == nil {
			t.Fatal("expected handler and start function to be set")
		}
		if logger == nil {
			t.Fatal("expected logger function")
		}
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("create listener for main callback: %v", err)
		}
		startErr := make(chan error, 1)
		go func() {
			startErr <- start(&http.Server{}, listener)
		}()
		_ = listener.Close()
		if serveErr := <-startErr; serveErr == nil {
			t.Fatal("expected start callback from main to return serve error when listener closes")
		}
		return nil
	}

	exitCode := -1
	exitProcess = func(code int) {
		exitCode = code
	}
	logPrintf = func(_ string, _ ...any) {}

	main()
	if !runCalled {
		t.Fatal("expected main to call runServer")
	}
	if exitCode != -1 {
		t.Fatalf("expected no exit on success, got %d", exitCode)
	}

	t.Setenv("PLATO_ADDR", "")
	runServer = func(addr string, _ http.Handler, _ func(*http.Server, net.Listener) error, _ func(string, ...any)) error {
		if addr != ":8070" {
			t.Fatalf("expected fallback addr in main, got %s", addr)
		}
		return nil
	}
	main()

	runServer = func(_ string, _ http.Handler, _ func(*http.Server, net.Listener) error, _ func(string, ...any)) error {
		return errors.New("boom")
	}
	var logMessages []string
	logPrintf = func(format string, args ...any) {
		logMessages = append(logMessages, fmt.Sprintf(format, args...))
	}
	main()
	if exitCode != 1 {
		t.Fatalf("expected exit code 1 when runServer fails, got %d", exitCode)
	}
	if !logsContain(logMessages, "server failed") {
		t.Fatalf("expected log message to include server failed, got %v", logMessages)
	}
}

func TestMainUsesModeDefaultsAndHandlesBootstrapErrors(t *testing.T) {
	previousRunServer := runServer
	previousMakeRouter := makeRouter
	previousLoadRuntimeConfig := loadRuntimeConfig
	previousLogPrintf := logPrintf
	previousExitProcess := exitProcess
	t.Cleanup(func() {
		runServer = previousRunServer
		makeRouter = previousMakeRouter
		loadRuntimeConfig = previousLoadRuntimeConfig
		logPrintf = previousLogPrintf
		exitProcess = previousExitProcess
	})

	makeRouter = func(httpapi.RuntimeConfig) (http.Handler, error) {
		return http.NewServeMux(), nil
	}
	runServer = func(addr string, _ http.Handler, _ func(*http.Server, net.Listener) error, _ func(string, ...any)) error {
		if addr != "127.0.0.1:8070" {
			t.Fatalf("expected development mode default addr, got %s", addr)
		}
		return nil
	}

	var logMessages []string
	logPrintf = func(format string, args ...any) {
		logMessages = append(logMessages, fmt.Sprintf(format, args...))
	}

	exitCode := -1
	exitProcess = func(code int) {
		exitCode = code
	}

	loadRuntimeConfig = func() (httpapi.RuntimeConfig, error) {
		return httpapi.RuntimeConfig{Mode: httpapi.RuntimeModeDevelopment}, nil
	}
	t.Setenv("PLATO_ADDR", "")
	main()
	if exitCode != -1 {
		t.Fatalf("expected no exit during successful startup, got %d", exitCode)
	}
	if !logsContain(logMessages, "development mode") {
		t.Fatalf("expected development mode warnings, got %v", logMessages)
	}

	loadRuntimeConfig = func() (httpapi.RuntimeConfig, error) {
		return httpapi.RuntimeConfig{}, errors.New("config failed")
	}
	logMessages = []string{}
	exitCode = -1
	main()
	if exitCode != 1 {
		t.Fatalf("expected exit code 1 on runtime config failure, got %d", exitCode)
	}
	if !logsContain(logMessages, "failed to load runtime config") {
		t.Fatalf("expected config failure log, got %v", logMessages)
	}

	loadRuntimeConfig = func() (httpapi.RuntimeConfig, error) {
		return httpapi.RuntimeConfig{Mode: httpapi.RuntimeModeProduction}, nil
	}
	makeRouter = func(httpapi.RuntimeConfig) (http.Handler, error) {
		return nil, errors.New("router failed")
	}
	logMessages = []string{}
	exitCode = -1
	main()
	if exitCode != 1 {
		t.Fatalf("expected exit code 1 on router initialization failure, got %d", exitCode)
	}
	if !logsContain(logMessages, "failed to initialize router") {
		t.Fatalf("expected router failure log, got %v", logMessages)
	}
}

func TestLogStartupWarnings(t *testing.T) {
	logMessages := []string{}
	logger := func(format string, args ...any) {
		logMessages = append(logMessages, fmt.Sprintf(format, args...))
	}

	logStartupWarnings(httpapi.RuntimeConfig{Mode: httpapi.RuntimeModeProduction}, logger)
	for _, message := range logMessages {
		if strings.Contains(strings.ToLower(message), "development mode") {
			t.Fatalf("expected no development warning in production mode, got %v", logMessages)
		}
	}

	logMessages = []string{}
	logStartupWarnings(httpapi.RuntimeConfig{Mode: httpapi.RuntimeModeDevelopment}, logger)
	expectedWarnings := []string{
		"development mode",
		"header-based dev auth",
		"do not expose",
	}
	for _, expectedWarning := range expectedWarnings {
		if !logsContain(logMessages, expectedWarning) {
			t.Fatalf("expected warning containing %q, got %v", expectedWarning, logMessages)
		}
	}
}

type testClosableHandler struct {
	http.Handler
	closeErr error
	closed   bool
}

func (h *testClosableHandler) Close() error {
	h.closed = true
	return h.closeErr
}

func logsContain(logs []string, substring string) bool {
	for _, entry := range logs {
		if strings.Contains(entry, substring) {
			return true
		}
	}

	return false
}

func waitForReady(t *testing.T, url string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	client := &http.Client{Timeout: 200 * time.Millisecond}
	for time.Now().Before(deadline) {
		resp, err := doGetRequest(client, url)
		if err == nil {
			_ = resp.Body.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("server did not become ready: %s", url)
}

func waitForLogMessage(t *testing.T, logs <-chan string, substring string) {
	t.Helper()

	timeout := time.NewTimer(2 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case entry := <-logs:
			if strings.Contains(entry, substring) {
				return
			}
		case <-timeout.C:
			t.Fatalf("did not observe log message containing %q", substring)
		}
	}
}

func waitForRequestRejection(url string) error {
	deadline := time.Now().Add(2 * time.Second)
	client := &http.Client{Timeout: 150 * time.Millisecond}

	for time.Now().Before(deadline) {
		resp, requestErr := doGetRequest(client, url)
		if requestErr == nil {
			if closeErr := resp.Body.Close(); closeErr != nil {
				return fmt.Errorf("close probe response body: %w", closeErr)
			}
		}
		if requestRejected(resp, requestErr) {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}

	return fmt.Errorf("request to %s kept succeeding during shutdown", url)
}

func drainLogChannel(logs <-chan string) []string {
	entries := make([]string, 0, 8)
	for {
		select {
		case entry := <-logs:
			entries = append(entries, entry)
		default:
			return entries
		}
	}
}

func requestRejected(resp *http.Response, requestErr error) bool {
	if requestErr != nil {
		return true
	}

	return resp.StatusCode >= http.StatusInternalServerError
}

func doGetRequest(client *http.Client, url string) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), client.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}

	return client.Do(req)
}
