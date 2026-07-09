package download

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRedirectEngineRecordsFollowedRedirect(t *testing.T) {
	engine, err := NewRedirectEngine(RedirectOptions{Follow: true, MaxRedirects: 3})
	if err != nil {
		t.Fatalf("new redirect engine: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/start":
			http.Redirect(writer, request, "/final", http.StatusFound)
		case "/final":
			writer.WriteHeader(http.StatusNoContent)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := &http.Client{CheckRedirect: engine.CheckRedirect}
	response, err := client.Get(server.URL + "/start")
	if err != nil {
		t.Fatalf("get with redirect: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d", response.StatusCode)
	}

	events := engine.Snapshot()
	if len(events) != 1 {
		t.Fatalf("events = %d", len(events))
	}
	if events[0].Method != http.MethodGet || events[0].To != server.URL+"/final" {
		t.Fatalf("unexpected event: %+v", events[0])
	}
}

func TestRedirectEngineCanDisableRedirectFollowing(t *testing.T) {
	engine, err := NewRedirectEngine(RedirectOptions{Follow: false, MaxRedirects: 0})
	if err != nil {
		t.Fatalf("new redirect engine: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, "/next", http.StatusTemporaryRedirect)
	}))
	defer server.Close()

	client := &http.Client{CheckRedirect: engine.CheckRedirect}
	response, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("get without following redirect: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if len(engine.Snapshot()) != 1 {
		t.Fatalf("expected attempted redirect to be recorded")
	}
}

func TestRedirectEngineEnforcesMaximumRedirects(t *testing.T) {
	engine, err := NewRedirectEngine(RedirectOptions{Follow: true, MaxRedirects: 1})
	if err != nil {
		t.Fatalf("new redirect engine: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/one" {
			http.Redirect(writer, request, "/two", http.StatusFound)
			return
		}
		http.Redirect(writer, request, "/three", http.StatusFound)
	}))
	defer server.Close()

	client := &http.Client{CheckRedirect: engine.CheckRedirect}
	_, err = client.Get(server.URL + "/one")
	if err == nil {
		t.Fatalf("expected redirect error")
	}
	if errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("expected max redirect error, got ErrUseLastResponse")
	}
	if got := len(engine.Snapshot()); got != 2 {
		t.Fatalf("events = %d", got)
	}
}

func TestRedirectEngineRejectsNegativeMax(t *testing.T) {
	if _, err := NewRedirectEngine(RedirectOptions{Follow: true, MaxRedirects: -1}); err == nil {
		t.Fatalf("expected error")
	}
}
