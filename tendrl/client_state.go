package tendrl

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// StateCallback handles state table changes detected by polling.
type StateCallback func(map[string]interface{}) error

func stateSnapshot(state map[string]interface{}) string {
	data, err := json.Marshal(state)
	if err != nil {
		return ""
	}
	return string(data)
}

func (c *Client) hasStateHandlers() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stateHandler != nil || c.stateCallback != nil
}

func (c *Client) hasInboundHandlers() bool {
	return c.hasMessageHandlers() || c.hasStateHandlers()
}

func (c *Client) dispatchState(state map[string]interface{}) error {
	// Snapshot under the lock, then call the handler with it released; see
	// dispatchMessage for why.
	c.mu.RLock()
	handler := c.stateHandler
	callback := c.stateCallback
	c.mu.RUnlock()

	if handler != nil {
		return handler(state)
	}
	if callback != nil {
		return callback(state)
	}
	return nil
}

// OnState registers a handler for remote state table changes (polled).
func (c *Client) OnState(handler StateCallback) {
	c.mu.Lock()
	c.stateHandler = handler
	c.mu.Unlock()
}

// SetStateCallback sets a catch-all state handler when OnState is not used.
func (c *Client) SetStateCallback(callback StateCallback) {
	c.mu.Lock()
	c.stateCallback = callback
	c.mu.Unlock()
}

// CheckState polls the state table and invokes handlers when it changes.
func (c *Client) CheckState() error {
	if !c.hasStateHandlers() {
		return nil
	}

	endpoint, err := url.JoinPath(c.baseURL, "/entities/status-table")
	if err != nil {
		return fmt.Errorf("failed to construct status-table endpoint URL: %w", err)
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create status-table request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", BuildUserAgent())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("status-table request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status-table failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read status-table response: %w", err)
	}

	var payload struct {
		StatusTable map[string]interface{} `json:"statusTable"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("failed to decode status-table response: %w", err)
	}

	state := payload.StatusTable
	if state == nil {
		state = map[string]interface{}{}
	}

	// Record the new table before dispatching so a manual CheckState racing the
	// background poller cannot see a stale snapshot and fire the handler twice.
	c.mu.Lock()
	seenBefore := c.lastStateInit
	previous := stateSnapshot(c.lastState)
	c.lastState = cloneStateMap(state)
	c.lastStateInit = true
	c.mu.Unlock()

	// The first poll establishes the baseline; only later changes are dispatched.
	if seenBefore && stateSnapshot(state) != previous {
		return c.dispatchState(state)
	}
	return nil
}

func cloneStateMap(state map[string]interface{}) map[string]interface{} {
	if state == nil {
		return map[string]interface{}{}
	}
	data, err := json.Marshal(state)
	if err != nil {
		return state
	}
	var copy map[string]interface{}
	if err := json.Unmarshal(data, &copy); err != nil {
		return state
	}
	return copy
}
