package tendrl

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// startMessageChecking used to return early unless a handler was already
// registered, but its only callers are the constructors — which run before any
// user code can call On/OnDefault/SetMessageCallback/OnState. The poller
// therefore never started, background message checking never happened, and
// SetMessageCheckRate had nothing to affect, all while the README promised a
// check every 3 seconds. These tests hold that behavior to the promise.
//
// Everything below runs against a local httptest server; no network.

// pollingTestServer stands in for the API: it answers the calls a managed
// client makes on startup and hands out one inbound message per check.
type pollingTestServer struct {
	*httptest.Server
	messageChecks *int32
	stateChecks   *int32
}

func newPollingTestServer(t *testing.T) *pollingTestServer {
	t.Helper()
	var messageChecks, stateChecks int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/claims"):
			w.WriteHeader(http.StatusOK)

		case strings.HasSuffix(r.URL.Path, "/entities/check_messages"):
			atomic.AddInt32(&messageChecks, 1)
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"messages":[{"msg_type":"cmd","source":"acct:us-1:entity:sender",`+
				`"timestamp":"2026-01-01T00:00:00Z","data":{"command":"restart"},`+
				`"context":{"tags":["ops","urgent"]}}]}`)

		case strings.HasSuffix(r.URL.Path, "/entities/status-table"):
			// A changing table on every poll, so a state handler is guaranteed
			// something to fire on after the first (baseline) poll.
			atomic.AddInt32(&stateChecks, 1)
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"statusTable":{"tick":`+
				string(rune('0'+atomic.LoadInt32(&stateChecks)%10))+`}}`)

		default:
			// /entities/status (PUT on start/stop) and anything else.
			w.WriteHeader(http.StatusOK)
		}
	}))
	t.Cleanup(srv.Close)

	return &pollingTestServer{Server: srv, messageChecks: &messageChecks, stateChecks: &stateChecks}
}

// newPollingClient builds a managed client pointed at srv, with storage,
// offline retry and heartbeats off so the only background HTTP traffic under
// test is the inbound poller.
func newPollingClient(t *testing.T, srv *pollingTestServer) *Client {
	t.Helper()
	t.Setenv("TENDRL_APP_URL", srv.URL)

	sendHeartbeat := false
	managed := true
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	cfg := &ConfigFile{
		Managed: &managed,
		// Offline storage is on by default in managed mode, and the default
		// path is relative -- without this a test run drops a database in the
		// package directory.
		StoragePath:   filepath.Join(t.TempDir(), "storage.db"),
		Timeout:       5,
		MaxRetries:    1,
		MaxQueueSize:  16,
		MinBatchSize:  1,
		MaxBatchSize:  10,
		SendHeartbeat: &sendHeartbeat,
	}
	if err := SaveConfigFile(cfg, cfgPath); err != nil {
		t.Fatalf("SaveConfigFile: %v", err)
	}

	client, err := NewClientWithConfigAndAPIKey(cfgPath, "test-key-not-real")
	if err != nil {
		t.Fatalf("NewClientWithConfigAndAPIKey: %v", err)
	}
	t.Cleanup(client.Stop)
	return client
}

func TestBackgroundPollingDeliversToAHandlerRegisteredAfterConstruction(t *testing.T) {
	srv := newPollingTestServer(t)
	client := newPollingClient(t, srv)

	got := make(chan IncomingMessage, 1)
	client.On(MessageRoute{
		Tag: "ops",
		Handler: func(msg IncomingMessage) error {
			select {
			case got <- msg:
			default:
			}
			return nil
		},
	})
	// Nothing polls at a useful rate for a test at the 3s default; this also
	// exercises SetMessageCheckRate taking effect on a running poller.
	client.SetMessageCheckRate(25 * time.Millisecond)

	select {
	case msg := <-got:
		if msg.MsgType != "cmd" {
			t.Errorf("MsgType = %q, want %q", msg.MsgType, "cmd")
		}
		if len(msg.Context.Tags) == 0 {
			t.Error("message arrived without its context tags")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no message was delivered: the background poller never ran, " +
			"so a handler registered after the constructor receives nothing")
	}

	if n := atomic.LoadInt32(srv.messageChecks); n < 1 {
		t.Errorf("check_messages was requested %d times, want at least 1", n)
	}
}

func TestBackgroundPollingDeliversStateChanges(t *testing.T) {
	srv := newPollingTestServer(t)
	client := newPollingClient(t, srv)

	got := make(chan map[string]interface{}, 1)
	client.OnState(func(state map[string]interface{}) error {
		select {
		case got <- state:
		default:
		}
		return nil
	})
	client.SetMessageCheckRate(25 * time.Millisecond)

	select {
	case state := <-got:
		if len(state) == 0 {
			t.Error("state handler fired with an empty table")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no state change was delivered; the state poller never ran")
	}
}

// A poller that ran on every tick regardless would call the API forever for a
// client that never registers a handler. It must stay silent until there is
// somewhere to deliver to.
func TestPollerIssuesNoRequestsWithoutHandlers(t *testing.T) {
	srv := newPollingTestServer(t)
	client := newPollingClient(t, srv)

	client.SetMessageCheckRate(10 * time.Millisecond)
	time.Sleep(200 * time.Millisecond)

	if n := atomic.LoadInt32(srv.messageChecks); n != 0 {
		t.Errorf("check_messages was called %d times with no handler registered, want 0", n)
	}
	if n := atomic.LoadInt32(srv.stateChecks); n != 0 {
		t.Errorf("status-table was called %d times with no handler registered, want 0", n)
	}
}
