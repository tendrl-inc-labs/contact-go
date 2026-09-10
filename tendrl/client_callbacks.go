package tendrl

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// SetMessageCallback sets a callback function to handle incoming messages
func (c *Client) SetMessageCallback(callback MessageCallback) {
	c.mu.Lock()
	c.callback = callback
	c.mu.Unlock()
}

// SetMessageCheckRate sets how often to check for messages (only effective in managed mode).
// It takes effect immediately: the background poller is woken so the next check
// is scheduled at the new rate rather than after the previous interval elapses.
func (c *Client) SetMessageCheckRate(rate time.Duration) {
	if rate <= 0 {
		return
	}
	c.mu.Lock()
	c.checkMsgRate = rate
	c.mu.Unlock()
	c.signalRateChanged()
}

// SetMessageCheckLimit sets the maximum number of messages to retrieve per check
func (c *Client) SetMessageCheckLimit(limit int) {
	c.mu.Lock()
	c.checkMsgLimit = limit
	c.mu.Unlock()
}

// messageCheckRate returns the current check interval.
func (c *Client) messageCheckRate() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.checkMsgRate
}

// messageCheckLimit returns the current per-check message limit.
func (c *Client) messageCheckLimit() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.checkMsgLimit
}

// signalRateChanged nudges the poller without blocking if it is busy; the
// channel is buffered so at most one pending wake-up is kept.
func (c *Client) signalRateChanged() {
	if c.rateChanged == nil {
		return
	}
	select {
	case c.rateChanged <- struct{}{}:
	default:
	}
}

// CheckMessages manually checks for incoming messages and calls the callback if set
func (c *Client) CheckMessages() error {
	if !c.hasMessageHandlers() {
		return nil
	}
	limit := c.messageCheckLimit()
	c.debugLog("Checking for incoming messages (limit=%d)", limit)

	// Construct the check messages endpoint (already correct)
	endpoint, err := url.JoinPath(c.baseURL, "/entities/check_messages")
	if err != nil {
		return fmt.Errorf("failed to construct check messages endpoint URL: %w", err)
	}

	// Add query parameter for message limit
	if limit > 0 {
		endpoint += fmt.Sprintf("?limit=%d", limit)
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create check messages request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", BuildUserAgent())

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Update connectivity state if we're in managed mode
		if c.config.Managed && c.connectivity != nil {
			c.updateConnectivityState(false)
		}
		return fmt.Errorf("check messages request failed: %w", err)
	}
	defer resp.Body.Close()

	// 204 means no messages available
	if resp.StatusCode == 204 {
		if c.config.Managed && c.connectivity != nil {
			c.updateConnectivityState(true)
		}
		return nil
	}

	// Check for success status codes
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if c.config.Managed && c.connectivity != nil {
			c.updateConnectivityState(true)
		}

		var response MessageCheckResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return fmt.Errorf("failed to decode check messages response: %w", err)
		}

		// Call callback for each message
		c.debugLog("Received %d message(s)", len(response.Messages))
		for _, message := range response.Messages {
			c.debugLog("Processing message: type=%s, source=%s", message.MsgType, message.Source)
			if err := c.dispatchMessage(message); err != nil {
				c.debugLog("Callback error for message: %v", err)
				// Continue processing other messages even if one callback fails
				// You might want to log this error depending on your needs
				continue
			}
		}

		return nil
	}

	return fmt.Errorf("check messages failed with status %d: %s", resp.StatusCode, resp.Status)
}

// startMessageChecking starts a goroutine to periodically check for messages
// (only in managed mode).
//
// This is called from the constructors, so it cannot gate on whether handlers
// exist: On/OnDefault/SetMessageCallback/OnState/SetStateCallback can only be
// called after the constructor returns. It previously did gate on that, which
// meant the poller never started at all and SetMessageCheckRate had no effect.
// The goroutine now runs for the life of a managed client and simply performs
// no request on ticks where nothing is registered to handle a result.
func (c *Client) startMessageChecking() {
	if !c.config.Managed {
		return
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		timer := time.NewTimer(c.messageCheckRate())
		defer timer.Stop()

		resetTimer := func() {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(c.messageCheckRate())
		}

		for {
			select {
			case <-timer.C:
				if c.hasMessageHandlers() {
					if err := c.CheckMessages(); err != nil {
						c.debugLog("Message check failed: %v", err)
					}
				}
				if c.hasStateHandlers() {
					if err := c.CheckState(); err != nil {
						c.debugLog("State check failed: %v", err)
					}
				}
				timer.Reset(c.messageCheckRate())

			case <-c.rateChanged:
				resetTimer()

			case <-c.done:
				return
			}
		}
	}()
}
