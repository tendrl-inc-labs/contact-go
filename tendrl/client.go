package tendrl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

var version = "0.1.0"

// debugLog logs a debug message if debug mode is enabled
func (c *Client) debugLog(format string, args ...interface{}) {
	if c.config != nil && c.config.Debug {
		log.Printf("[DEBUG] "+format, args...)
	}
}

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	config     *Config

	// Managed mode components (only initialized if config.Managed=true)
	queue        chan Message
	metrics      *SystemMetrics
	storage      *Storage
	connectivity *ConnectivityState
	wg           sync.WaitGroup
	done         chan struct{}

	// Message callback functionality.
	// Handlers may be registered at any time from any goroutine while the
	// background poller is running, so everything below is guarded by mu.
	mu             sync.RWMutex
	callback       MessageCallback
	defaultHandler MessageCallback
	routes         []MessageRoute
	stateHandler   StateCallback
	stateCallback  StateCallback
	lastState      map[string]interface{}
	lastStateInit  bool
	checkMsgRate   time.Duration // How often to check for messages
	checkMsgLimit  int           // Maximum number of messages to retrieve per check

	// rateChanged wakes the background poller so a new check rate takes effect
	// immediately instead of after the previous interval elapses.
	rateChanged chan struct{}

	// Heartbeat functionality
	lastHeartbeat time.Time // Last time heartbeat was sent
}

// NewClient creates a new Tendrl client
// managed: true for managed mode (full features: queuing, batching, offline storage), false for headless mode (direct API calls)
// apiKey: Optional API key for authentication. If not provided or empty, will use TENDRL_KEY environment variable
func NewClient(managed bool, apiKey ...string) (*Client, error) {
	key := ""
	if len(apiKey) > 0 {
		key = apiKey[0]
	}
	return NewClientWithModeAndAPIKey(managed, key)
}

// NewClientWithMode creates a new Tendrl client using the TENDRL_KEY environment variable
// managed: true for managed mode (full features: queuing, batching, offline storage), false for headless mode (direct API calls)
func NewClientWithMode(managed bool) (*Client, error) {
	return NewClientWithModeAndAPIKey(managed, "")
}

// NewClientWithModeAndAPIKey creates a new Tendrl client with the specified mode and API key
func NewClientWithModeAndAPIKey(managed bool, apiKey string) (*Client, error) {
	// Load configuration from file if available
	configFile, err := LoadConfigFile()
	if err != nil {
		return nil, fmt.Errorf("failed to load config file: %w", err)
	}

	// Convert to internal config structure with defaults
	config := configFile.toConfig()

	// Override managed mode if explicitly specified
	config.Managed = managed

	// Disable managed-mode features
	if !config.Managed {
		config.OfflineStorage = false
		config.OfflineRetryEnabled = false
		config.ConnectivityCheckEnabled = false
	}

	client := &Client{
		config:        config,
		apiKey:        apiKey,          // Set API key if provided
		checkMsgRate:  3 * time.Second, // Default: check every 3 seconds
		checkMsgLimit: 1,               // Default: get 1 message per check
		rateChanged:   make(chan struct{}, 1),
		lastHeartbeat: time.Now(), // Initialize heartbeat timer
	}

	// Initialize managed mode components only if needed
	if managed {
		client.metrics = &SystemMetrics{}
		client.done = make(chan struct{})
	}

	// Initialize HTTP client with timeout from config
	client.httpClient = &http.Client{
		Timeout: client.config.Timeout,
	}

	// Set up API configuration
	if err := client.setupAPIConfig(); err != nil {
		return nil, err
	}

	// Initialize managed mode components
	if client.config.Managed {
		// Initialize connectivity state
		client.connectivity = &ConnectivityState{
			Online:       true, // Assume online initially
			LastCheck:    time.Now(),
			CheckEnabled: client.config.ConnectivityCheckEnabled,
		}

		// Initialize offline storage if enabled
		if client.config.OfflineStorage {
			storage, err := NewStorage(client.config.StoragePath)
			if err != nil {
				return nil, err
			}
			client.storage = storage
		}

		client.queue = make(chan Message, client.config.MaxQueueSize)

		// Start background goroutines
		goroutineCount := 2
		if client.config.OfflineStorage && client.config.OfflineRetryEnabled {
			goroutineCount++ // Add offline retry goroutine
		}
		if client.config.ConnectivityCheckEnabled {
			goroutineCount++ // Add connectivity check goroutine
		}

		client.wg.Add(goroutineCount)
		go func() {
			defer client.wg.Done()
			client.processQueue()
		}()
		go func() {
			defer client.wg.Done()
			client.updateMetrics()
		}()

		// Start offline retry goroutine if enabled
		if client.config.OfflineStorage && client.config.OfflineRetryEnabled {
			go func() {
				defer client.wg.Done()
				client.processOfflineRetries()
			}()
		}

		// Start connectivity check goroutine if enabled
		if client.config.ConnectivityCheckEnabled {
			go func() {
				defer client.wg.Done()
				client.monitorConnectivity()
			}()
		}

		// Start the inbound poller. It runs for the life of a managed client and
		// no-ops per tick until a handler is registered, because handlers can only
		// be registered after this constructor returns.
		client.startMessageChecking()

		// Update entity status to online
		client.updateEntityStatus(true)
	}

	return client, nil
}

// NewClientWithConfig creates a client with a specific configuration file
func NewClientWithConfig(configPath string) (*Client, error) {
	return NewClientWithConfigAndAPIKey(configPath, "")
}

// NewClientWithConfigAndAPIKey creates a client with a specific configuration file and API key
func NewClientWithConfigAndAPIKey(configPath string, apiKey string) (*Client, error) {
	config, err := loadConfigFromPath(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from %s: %w", configPath, err)
	}

	client := &Client{
		config:        config.toConfig(),
		apiKey:        apiKey,          // Set API key if provided
		checkMsgRate:  3 * time.Second, // Default: check every 3 seconds
		checkMsgLimit: 1,               // Default: get 1 message per check
		rateChanged:   make(chan struct{}, 1),
		lastHeartbeat: time.Now(), // Initialize heartbeat timer
	}

	// Initialize managed mode components only if needed
	if client.config.Managed {
		client.metrics = &SystemMetrics{}
		client.done = make(chan struct{})
	}

	// Initialize HTTP client with timeout from config
	client.httpClient = &http.Client{
		Timeout: client.config.Timeout,
	}

	// Set up API configuration
	if err := client.setupAPIConfig(); err != nil {
		return nil, err
	}

	// Initialize managed mode components
	if client.config.Managed {
		// Initialize connectivity state
		client.connectivity = &ConnectivityState{
			Online:       true, // Assume online initially
			LastCheck:    time.Now(),
			CheckEnabled: client.config.ConnectivityCheckEnabled,
		}

		// Initialize offline storage if enabled
		if client.config.OfflineStorage {
			storage, err := NewStorage(client.config.StoragePath)
			if err != nil {
				return nil, err
			}
			client.storage = storage
		}

		client.queue = make(chan Message, client.config.MaxQueueSize)

		// Start background goroutines
		goroutineCount := 2
		if client.config.OfflineStorage && client.config.OfflineRetryEnabled {
			goroutineCount++ // Add offline retry goroutine
		}
		if client.config.ConnectivityCheckEnabled {
			goroutineCount++ // Add connectivity check goroutine
		}

		client.wg.Add(goroutineCount)
		go func() {
			defer client.wg.Done()
			client.processQueue()
		}()
		go func() {
			defer client.wg.Done()
			client.updateMetrics()
		}()

		// Start offline retry goroutine if enabled
		if client.config.OfflineStorage && client.config.OfflineRetryEnabled {
			go func() {
				defer client.wg.Done()
				client.processOfflineRetries()
			}()
		}

		// Start connectivity check goroutine if enabled
		if client.config.ConnectivityCheckEnabled {
			go func() {
				defer client.wg.Done()
				client.monitorConnectivity()
			}()
		}

		// Start the inbound poller. It runs for the life of a managed client and
		// no-ops per tick until a handler is registered, because handlers can only
		// be registered after this constructor returns.
		client.startMessageChecking()

		// Update entity status to online
		client.updateEntityStatus(true)
	}

	return client, nil
}

func (c *Client) setupAPIConfig() error {
	// Get API key from parameter, then environment, then error
	if c.apiKey == "" {
		if envKey := os.Getenv("TENDRL_KEY"); envKey != "" {
			c.apiKey = envKey
		} else {
			return fmt.Errorf("no API key provided - set TENDRL_KEY environment variable or pass API key to client constructor")
		}
	}

	// Base URL: TENDRL_APP_URL env or production. Accepts either a bare origin
	// ("http://192.168.1.50" — the SDK appends /api) or a full base URL already
	// ending in /api (nano-agent style), so the same env var works everywhere.
	// Without this the SDK could only ever talk to production, which made it
	// impossible to test against a staging environment or a local stack.
	if envURL := os.Getenv("TENDRL_APP_URL"); envURL != "" {
		trimmed := strings.TrimRight(envURL, "/")
		if !strings.HasSuffix(trimmed, "/api") {
			trimmed += "/api"
		}
		c.baseURL = trimmed
	} else {
		c.baseURL = "https://app.tendrl.com/api"
	}
	c.debugLog("API base URL: %s", c.baseURL)

	// Validate API key in managed mode
	if c.config.Managed {
		c.debugLog("Validating API key")
		if err := c.validateAPIKey(); err != nil {
			return fmt.Errorf("API key validation failed: %w", err)
		}
		c.debugLog("API key validated successfully")
	}

	return nil
}

// validateAPIKey checks if the API key is valid by calling the /claims endpoint
func (c *Client) validateAPIKey() error {
	endpoint, err := url.JoinPath(c.baseURL, "/claims")
	if err != nil {
		return fmt.Errorf("failed to construct claims endpoint URL: %w", err)
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create claims request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", BuildUserAgent())

	// Use a shorter timeout for validation
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("claims request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check for success status codes
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil // API key is valid
	}

	// Handle specific error cases
	switch resp.StatusCode {
	case 401:
		return fmt.Errorf("invalid API key (unauthorized)")
	case 403:
		return fmt.Errorf("API key lacks required permissions (forbidden)")
	case 429:
		return fmt.Errorf("rate limited - too many requests")
	default:
		return fmt.Errorf("claims validation failed with status %d: %s", resp.StatusCode, resp.Status)
	}
}

// GetSystemMetrics returns current system performance metrics (only available in managed mode)
func (c *Client) GetSystemMetrics() SystemMetrics {
	if !c.config.Managed || c.metrics == nil {
		return SystemMetrics{} // Return empty metrics if not in managed mode
	}
	c.metrics.RLock()
	defer c.metrics.RUnlock()
	return SystemMetrics{
		CPUUsage:    c.metrics.CPUUsage,
		MemoryUsage: c.metrics.MemoryUsage,
		QueueLoad:   c.metrics.QueueLoad,
	}
}

// GetOfflineStorageStats returns offline storage statistics (only available in managed mode)
func (c *Client) GetOfflineStorageStats() OfflineStorageStats {
	stats := OfflineStorageStats{
		Enabled:        c.config.Managed && c.config.OfflineStorage,
		RetryEnabled:   c.config.Managed && c.config.OfflineRetryEnabled,
		BatchSize:      50, // Max messages per batch (from retryOfflineMessages)
		MaxBatchCycles: 5,  // Max batches per retry cycle (from retryOfflineMessages)
	}

	if c.config.Managed && c.storage != nil {
		count, err := c.storage.Count()
		if err == nil {
			stats.MessageCount = count
		}
	}

	return stats
}

// GetConnectivityState returns current connectivity information (only available in managed mode)
func (c *Client) GetConnectivityState() ConnectivityState {
	if !c.config.Managed || c.connectivity == nil {
		return ConnectivityState{} // Return empty state if not in managed mode
	}
	c.connectivity.RLock()
	defer c.connectivity.RUnlock()
	return ConnectivityState{
		Online:       c.connectivity.Online,
		LastCheck:    c.connectivity.LastCheck,
		LastOnline:   c.connectivity.LastOnline,
		LastOffline:  c.connectivity.LastOffline,
		CheckEnabled: c.connectivity.CheckEnabled,
	}
}

func (c *Client) Stop() {
	c.debugLog("TendrlClient stopping")

	// Update entity status to offline
	if c.config.Managed {
		c.updateEntityStatus(false)
	}

	if c.config.Managed {
		if c.done != nil {
			close(c.done)
		}
		if c.storage != nil {
			c.storage.Close()
		}
		c.wg.Wait()
	}
	c.debugLog("TendrlClient stopped")
}

// updateEntityStatus updates the entity's online/offline status on the backend
// This is called automatically on client start (online=true) and stop (online=false)
func (c *Client) updateEntityStatus(online bool) {
	endpoint, err := url.JoinPath(c.baseURL, "/entities/status")
	if err != nil {
		c.debugLog("Failed to construct status endpoint URL: %v", err)
		return
	}

	// Create request body
	requestBody := map[string]bool{"online": online}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		c.debugLog("Failed to marshal status request: %v", err)
		return
	}

	req, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		c.debugLog("Failed to create status update request: %v", err)
		return
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", BuildUserAgent())

	// Execute request with a short timeout
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.debugLog("Error updating entity status: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		status := "offline"
		if online {
			status = "online"
		}
		c.debugLog("Entity status updated to %s", status)
	} else {
		c.debugLog("Failed to update entity status: %d %s", resp.StatusCode, resp.Status)
	}
}

// Tether attaches a function to run periodically and publish results to the cloud.
// Returns a stop function. Only available in managed mode.
func (c *Client) Tether(name string, fn DataFunc, tags []string, interval time.Duration) func() {
	if !c.config.Managed {
		// Return a no-op function for headless mode
		return func() {}
	}

	stopChan := make(chan struct{})

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				data, err := fn()
				if err != nil {
					continue // Skip on error
				}

				// Use the async publish method for background collections
				c.PublishAsync(data, tags)

			case <-stopChan:
				return
			case <-c.done:
				return
			}
		}
	}()

	// Return stop function
	return func() {
		close(stopChan)
	}
}
