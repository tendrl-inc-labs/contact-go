package tendrl

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Stop() must send what is still queued.
//
// The regression: the shutdown branch flushed only the batch already assembled
// in local memory and returned, abandoning anything still sitting in the queue
// channel. A select picks at random when more than one case is ready, so the
// loss was intermittent rather than total -- which is exactly why it survived.
// Nothing raised and nothing was logged.

type shutdownServer struct {
	*httptest.Server
	mu     sync.Mutex
	seen   map[string]bool
	claims int
}

// newSlowShutdownServer delays message posts, which is what puts the processor
// inside a send when Stop() arrives -- the condition under which the loss
// actually happens. Any real network does this; a local httptest server that
// answers instantly does not, which is why the bug survived.
func newSlowShutdownServer(t *testing.T, d time.Duration) *shutdownServer {
	t.Helper()
	s := newShutdownServer(t)
	base := s.Server.Config.Handler
	s.Server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/messages") {
			time.Sleep(d)
		}
		base.ServeHTTP(w, r)
	})
	return s
}

func newShutdownServer(t *testing.T) *shutdownServer {
	t.Helper()
	s := &shutdownServer{seen: map[string]bool{}}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/claims"):
			s.mu.Lock()
			s.claims++
			s.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"code":200,"content":{"type":"entity"}}`)
			return
		case strings.HasSuffix(r.URL.Path, "/messages"):
			body, _ := io.ReadAll(r.Body)
			var msgs []map[string]any
			if err := json.Unmarshal(body, &msgs); err == nil {
				s.mu.Lock()
				for _, m := range msgs {
					if d, ok := m["data"].(map[string]any); ok {
						if marker, ok := d["marker"].(string); ok {
							s.seen[marker] = true
						}
					}
				}
				s.mu.Unlock()
			}
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"code":200,"content":[]}`)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *shutdownServer) got(marker string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seen[marker]
}

// A first publish puts the processor into a slow send; the rest pile up in the
// channel behind it. That is the shape of the loss: against the previous commit
// this drops 72 of 96.
func TestStopSendsWhatIsStillQueued(t *testing.T) {
	const trials, perTrial = 6, 8
	srv := newSlowShutdownServer(t, 400*time.Millisecond)
	t.Setenv("TENDRL_APP_URL", srv.URL)

	var lost []string
	for i := 0; i < trials; i++ {
		c, err := NewClientWithModeAndAPIKey(true, "test-key")
		if err != nil {
			t.Fatalf("NewClient: %v", err)
		}

		if _, err := c.Publish(map[string]any{"marker": fmt.Sprintf("warm-%d", i)},
			[]string{"sensor"}, "", false, 5); err != nil {
			t.Fatalf("warm-up Publish: %v", err)
		}
		time.Sleep(120 * time.Millisecond) // the send is now in flight

		for j := 0; j < perTrial; j++ {
			marker := fmt.Sprintf("queued-%d-%d", i, j)
			if _, err := c.Publish(map[string]any{"marker": marker},
				[]string{"sensor"}, "", false, 5); err != nil {
				t.Fatalf("Publish: %v", err)
			}
		}
		c.Stop()

		for j := 0; j < perTrial; j++ {
			if marker := fmt.Sprintf("queued-%d-%d", i, j); !srv.got(marker) {
				lost = append(lost, marker)
			}
		}
	}

	if len(lost) > 0 {
		t.Errorf("Stop() discarded %d of %d queued messages; first few: %v",
			len(lost), trials*perTrial, lost[:min(5, len(lost))])
	}
}

// Stop() must also wait for the drain before closing storage, or a final batch
// that fails to send has nowhere to go.
func TestStopWaitsForTheDrainBeforeClosingStorage(t *testing.T) {
	srv := newSlowShutdownServer(t, 200*time.Millisecond)
	t.Setenv("TENDRL_APP_URL", srv.URL)

	c, err := NewClientWithModeAndAPIKey(true, "test-key")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	for i := 0; i < 4; i++ {
		if _, err := c.Publish(map[string]any{"marker": fmt.Sprintf("ordered-%d", i)},
			[]string{"sensor"}, "", false, 5); err != nil {
			t.Fatalf("Publish: %v", err)
		}
	}
	c.Stop() // must not panic on a closed store, and must not hang

	for i := 0; i < 4; i++ {
		if marker := fmt.Sprintf("ordered-%d", i); !srv.got(marker) {
			t.Errorf("%s never arrived", marker)
		}
	}
}
