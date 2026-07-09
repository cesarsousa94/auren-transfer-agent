package download

import (
	"net/http"
	"testing"
)

func TestCookieEngineStoresAndSnapshotsCookies(t *testing.T) {
	engine, err := NewCookieEngine()
	if err != nil {
		t.Fatalf("new cookie engine: %v", err)
	}
	if err := engine.SetCookies("https://example.com/media", []*http.Cookie{{Name: "session", Value: "abc"}}); err != nil {
		t.Fatalf("set cookies: %v", err)
	}

	cookies, err := engine.Cookies("https://example.com/other")
	if err != nil {
		t.Fatalf("cookies: %v", err)
	}
	if len(cookies) != 1 || cookies[0].Name != "session" || cookies[0].Value != "abc" {
		t.Fatalf("unexpected cookies: %+v", cookies)
	}

	cookies[0].Value = "mutated"
	again, err := engine.Cookies("https://example.com/other")
	if err != nil {
		t.Fatalf("cookies again: %v", err)
	}
	if again[0].Value != "abc" {
		t.Fatalf("cookies not defensively copied")
	}

	snapshot, err := engine.Snapshot("https://example.com/other")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snapshot) != 1 || snapshot[0].Name != "session" {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
}

func TestCookieEngineRejectsInvalidURL(t *testing.T) {
	engine, err := NewCookieEngine()
	if err != nil {
		t.Fatalf("new cookie engine: %v", err)
	}
	if err := engine.SetCookies("ftp://example.com/file", []*http.Cookie{{Name: "x", Value: "1"}}); err == nil {
		t.Fatalf("expected scheme error")
	}
	if _, err := engine.Cookies("/relative"); err == nil {
		t.Fatalf("expected absolute url error")
	}
}
