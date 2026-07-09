// Package download contains foundation HTTP download primitives.
package download

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

const (
	// RedirectEngineName is the canonical name of the foundation redirect engine.
	RedirectEngineName = "redirect"
)

// RedirectOptions configures the mechanical redirect policy used by the HTTP client.
type RedirectOptions struct {
	Follow       bool
	MaxRedirects int
}

// RedirectEvent records one redirect transition observed by the client.
type RedirectEvent struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Method string `json:"method"`
}

// RedirectEngine applies and records redirect behavior without making business decisions.
type RedirectEngine struct {
	follow       bool
	maxRedirects int
	mu           sync.Mutex
	events       []RedirectEvent
}

// NewRedirectEngine creates a redirect policy with validation.
func NewRedirectEngine(options RedirectOptions) (*RedirectEngine, error) {
	if options.MaxRedirects < 0 {
		return nil, fmt.Errorf("max redirects must be zero or greater")
	}
	return &RedirectEngine{follow: options.Follow, maxRedirects: options.MaxRedirects}, nil
}

// CheckRedirect implements http.Client redirect policy.
func (engine *RedirectEngine) CheckRedirect(request *http.Request, via []*http.Request) error {
	if engine == nil {
		return fmt.Errorf("redirect engine cannot be nil")
	}
	if request == nil {
		return fmt.Errorf("redirect request cannot be nil")
	}

	if len(via) > 0 {
		from := ""
		if via[len(via)-1] != nil && via[len(via)-1].URL != nil {
			from = via[len(via)-1].URL.String()
		}
		to := ""
		if request.URL != nil {
			to = request.URL.String()
		}
		engine.record(RedirectEvent{From: from, To: to, Method: request.Method})
	}

	if !engine.follow {
		return http.ErrUseLastResponse
	}
	if len(via) > engine.maxRedirects {
		return fmt.Errorf("stopped after %d redirects", engine.maxRedirects)
	}
	return nil
}

// Follow reports whether redirects are followed.
func (engine *RedirectEngine) Follow() bool {
	if engine == nil {
		return false
	}
	return engine.follow
}

// MaxRedirects returns the configured maximum redirect count.
func (engine *RedirectEngine) MaxRedirects() int {
	if engine == nil {
		return 0
	}
	return engine.maxRedirects
}

// Snapshot returns a defensive copy of observed redirect events.
func (engine *RedirectEngine) Snapshot() []RedirectEvent {
	if engine == nil {
		return nil
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	output := make([]RedirectEvent, len(engine.events))
	copy(output, engine.events)
	return output
}

// Reset clears observed redirect events.
func (engine *RedirectEngine) Reset() {
	if engine == nil {
		return
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.events = nil
}

func (engine *RedirectEngine) record(event RedirectEvent) {
	event.From = strings.TrimSpace(event.From)
	event.To = strings.TrimSpace(event.To)
	event.Method = strings.ToUpper(strings.TrimSpace(event.Method))
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.events = append(engine.events, event)
}
