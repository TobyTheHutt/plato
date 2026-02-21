package main

import (
	"errors"
	"net/http"
	"strings"
	"testing"
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

	if err := run(":8070", handler, func(server *http.Server) error {
		if server.Addr != ":8070" {
			t.Fatalf("unexpected server addr %s", server.Addr)
		}
		if server.Handler != handler {
			t.Fatalf("unexpected server handler")
		}
		if server.ReadHeaderTimeout == 0 {
			t.Fatalf("expected read header timeout to be set")
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

	if err := run(":8070", handler, func(_ *http.Server) error {
		return http.ErrServerClosed
	}, nil); err != nil {
		t.Fatalf("expected nil on server closed, got %v", err)
	}

	expected := errors.New("boom")
	if err := run(":8070", handler, func(_ *http.Server) error {
		return expected
	}, nil); !errors.Is(err, expected) {
		t.Fatalf("expected propagated error, got %v", err)
	}

	if err := run(":8070", handler, nil, nil); err == nil {
		t.Fatal("expected error for nil start function")
	}
}

func TestMainUsesRunServerAndFatalHandler(t *testing.T) {
	previousRunServer := runServer
	previousMakeRouter := makeRouter
	previousLogFatalf := logFatalf
	previousLogPrintf := logPrintf
	t.Cleanup(func() {
		runServer = previousRunServer
		makeRouter = previousMakeRouter
		logFatalf = previousLogFatalf
		logPrintf = previousLogPrintf
	})

	t.Setenv("PLATO_ADDR", ":8123")
	makeRouter = func() http.Handler {
		return http.NewServeMux()
	}

	runCalled := false
	runServer = func(addr string, handler http.Handler, start func(*http.Server) error, logger func(string, ...any)) error {
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
		return nil
	}
	logPrintf = func(_ string, _ ...any) {}
	logFatalf = func(_ string, _ ...any) {
		t.Fatal("fatal logger should not be called on success")
	}

	main()
	if !runCalled {
		t.Fatal("expected main to call runServer")
	}

	t.Setenv("PLATO_ADDR", "")
	runServer = func(addr string, _ http.Handler, _ func(*http.Server) error, _ func(string, ...any)) error {
		if addr != ":8070" {
			t.Fatalf("expected fallback addr in main, got %s", addr)
		}
		return nil
	}
	main()

	runServer = func(_ string, _ http.Handler, _ func(*http.Server) error, _ func(string, ...any)) error {
		return errors.New("boom")
	}
	var fatalMessage string
	logFatalf = func(format string, args ...any) {
		fatalMessage = format
	}
	main()
	if !strings.Contains(fatalMessage, "server failed") {
		t.Fatalf("expected fatal message to include server failed, got %q", fatalMessage)
	}
}
