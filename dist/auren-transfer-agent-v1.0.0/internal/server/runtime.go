package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	// RuntimeName identifies the production HTTP server lifecycle contract.
	RuntimeName = "http_server_runtime"
)

// ServeOptions defines the minimal production server runtime settings.
type ServeOptions struct {
	Address         string
	Handler         http.Handler
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// ServeResult describes a completed server lifecycle.
type ServeResult struct {
	Address string
	Stopped bool
}

// Serve starts an HTTP server and blocks until the context is cancelled or the
// server fails. It is intentionally transport-only: it never decides which jobs
// to execute or how the Media Hub distributes work.
func Serve(ctx context.Context, options ServeOptions) (ServeResult, error) {
	if ctx == nil {
		return ServeResult{}, fmt.Errorf("server context cannot be nil")
	}
	if options.Handler == nil {
		return ServeResult{}, fmt.Errorf("server handler cannot be nil")
	}
	if options.Address == "" {
		return ServeResult{}, fmt.Errorf("server address cannot be empty")
	}
	if options.ShutdownTimeout <= 0 {
		return ServeResult{}, fmt.Errorf("server shutdown timeout must be greater than zero")
	}

	httpServer := &http.Server{
		Addr:              options.Address,
		Handler:           options.Handler,
		ReadHeaderTimeout: options.ReadTimeout,
		ReadTimeout:       options.ReadTimeout,
		WriteTimeout:      options.WriteTimeout,
		IdleTimeout:       options.IdleTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		err := httpServer.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			errCh <- nil
			return
		}
		errCh <- err
	}()

	select {
	case err := <-errCh:
		return ServeResult{Address: options.Address, Stopped: err == nil}, err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), options.ShutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return ServeResult{Address: options.Address}, err
		}
		err := <-errCh
		return ServeResult{Address: options.Address, Stopped: true}, err
	}
}
