package server

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestServeStartsAndStopsWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	ready := make(chan struct{})
	handler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("ok"))
	})

	resultCh := make(chan struct {
		result ServeResult
		err    error
	}, 1)
	go func() {
		close(ready)
		result, err := Serve(ctx, ServeOptions{
			Address:         "127.0.0.1:0",
			Handler:         handler,
			ReadTimeout:     time.Second,
			WriteTimeout:    time.Second,
			IdleTimeout:     time.Second,
			ShutdownTimeout: time.Second,
		})
		resultCh <- struct {
			result ServeResult
			err    error
		}{result: result, err: err}
	}()

	<-ready
	cancel()

	select {
	case output := <-resultCh:
		if output.err != nil {
			t.Fatalf("expected graceful stop, got %v", output.err)
		}
		if !output.result.Stopped {
			t.Fatalf("expected stopped result")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop after context cancellation")
	}
}

func TestServeRejectsInvalidOptions(t *testing.T) {
	ctx := context.Background()
	cases := []ServeOptions{
		{Address: "", Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), ShutdownTimeout: time.Second},
		{Address: "127.0.0.1:0", Handler: nil, ShutdownTimeout: time.Second},
		{Address: "127.0.0.1:0", Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), ShutdownTimeout: 0},
	}

	for _, options := range cases {
		_, err := Serve(ctx, options)
		if err == nil {
			t.Fatalf("expected invalid options to fail: %+v", options)
		}
	}
}
