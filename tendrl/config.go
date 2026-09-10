package tendrl

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// ConfigFile represents the structure of the JSON configuration file
type ConfigFile struct {
	// HTTP settings
	Timeout    int `json:"timeout_seconds,omitempty"` // Timeout in seconds (JSON friendly)
	MaxRetries int `json:"max_retries,omitempty"`

	// Mode settings.
	//
	// These four are pointers for the same reason SendHeartbeat is: a plain
	// bool has no "unset" state, so an absent config file read as an explicit
	// false and overwrote the defaults below with zero values. A managed client
	// on a host with no config file therefore ran with offline storage, offline
	// retry, connectivity checking and heartbeats all off, which is the opposite
	// of what managed mode is documented to do.
	Managed *bool `json:"managed,omitempty"`

	// Batch processing settings
	MinBatchSize     int     `json:"min_batch_size,omitempty"`
	MaxBatchSize     int     `json:"max_batch_size,omitempty"`
	MaxQueueSize     int     `json:"max_queue_size,omitempty"`
	TargetCPUPercent float64 `json:"target_cpu_percent,omitempty"`
	TargetMemPercent float64 `json:"target_mem_percent,omitempty"`

	// Batch timing settings (in milliseconds for JSON)
	MinBatchIntervalMs int `json:"min_batch_interval_ms,omitempty"`
	MaxBatchIntervalMs int `json:"max_batch_interval_ms,omitempty"`

	// Storage settings
	OfflineStorage *bool  `json:"offline_storage,omitempty"`
	StoragePath    string `json:"storage_path,omitempty"`

	// Offline retry settings
	OfflineRetryEnabled  *bool `json:"offline_retry_enabled,omitempty"`
	OfflineRetryInterval int   `json:"offline_retry_interval_seconds,omitempty"`
	OfflineRetryLimit    int   `json:"offline_retry_limit,omitempty"`

	// Connectivity monitoring settings
	ConnectivityCheckEnabled  *bool `json:"connectivity_check_enabled,omitempty"`
	ConnectivityCheckInterval int   `json:"connectivity_check_interval_seconds,omitempty"`

	// Heartbeat settings
	SendHeartbeat     *bool `json:"send_heartbeat,omitempty"`             // Enable automatic heartbeat messages (nil = use default true)
	HeartbeatInterval int   `json:"heartbeat_interval_seconds,omitempty"` // Interval between heartbeats in seconds

	// Debug settings
	Debug bool `json:"debug,omitempty"` // Enable debug logging
}

// LoadConfigFile loads configuration from a JSON file
// It looks for config files in the following order:
// 1. ~/.tendrl/config.json (user home directory)
// 2. /etc/tendrl/config.json (system directory, Unix only)
func LoadConfigFile() (*ConfigFile, error) {
	// Define potential config file paths (no current directory)
	var configPaths []string

	// Add user home directory path
	if homeDir, err := os.UserHomeDir(); err == nil {
		configPaths = append(configPaths, filepath.Join(homeDir, ".tendrl", "config.json"))
	}

	// Add system directory for Unix systems
	if runtime.GOOS != "windows" {
		configPaths = append(configPaths, "/etc/tendrl/config.json")
	}

	// Try each path until we find a readable config file
	for _, path := range configPaths {
		if config, err := loadConfigFromPath(path); err == nil {
			return config, nil
		}
	}

	// Return empty config if no file found (will use defaults and env vars)
	return &ConfigFile{}, nil
}

// loadConfigFromPath loads configuration from a specific file path
func loadConfigFromPath(path string) (*ConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config ConfigFile
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("invalid JSON in config file %s: %w", path, err)
	}

	return &config, nil
}

// SaveConfigFile saves configuration to a JSON file
func SaveConfigFile(config *ConfigFile, path string) error {
	// Create directory if it doesn't exist
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	// Pretty print JSON for better readability
	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetDefaultConfigPath returns the default config file path for the current user
func GetDefaultConfigPath() string {
	if homeDir, err := os.UserHomeDir(); err == nil {
		return filepath.Join(homeDir, ".tendrl", "config.json")
	}
	return "./tendrl.json" // Fallback to current directory
}

// GenerateExampleConfig creates an example configuration file with all options
func GenerateExampleConfig() *ConfigFile {
	enabled := true
	sendHeartbeat := &enabled
	config := &ConfigFile{
		// HTTP settings
		Timeout:    10,
		MaxRetries: 3,

		// Mode settings
		Managed: &enabled,

		// Batch processing settings
		MinBatchSize:     10,
		MaxBatchSize:     500,
		MaxQueueSize:     1000,
		TargetCPUPercent: 70.0,
		TargetMemPercent: 80.0,

		// Batch timing settings (in milliseconds)
		MinBatchIntervalMs: 100,
		MaxBatchIntervalMs: 1000,

		// Storage settings
		OfflineStorage: &enabled,
		StoragePath:    "tendrl_storage.db",

		// Offline retry settings
		OfflineRetryEnabled:  &enabled,
		OfflineRetryInterval: 30,
		OfflineRetryLimit:    5,

		// Connectivity monitoring settings
		ConnectivityCheckEnabled:  &enabled,
		ConnectivityCheckInterval: 30,

		// Heartbeat settings
		SendHeartbeat:     sendHeartbeat,
		HeartbeatInterval: 30,

		// Debug settings
		Debug: false,
	}
	return config
}

// toConfig converts ConfigFile to internal Config struct
func (cf *ConfigFile) toConfig() *Config {
	config := &Config{
		// Core configuration defaults
		Managed:    true,
		Timeout:    10 * time.Second,
		MaxRetries: 3,

		// Managed mode configuration defaults
		MinBatchSize:         10,
		MaxBatchSize:         500,
		MaxQueueSize:         1000,
		TargetCPUPercent:     70.0,
		TargetMemPercent:     80.0,
		MinBatchInterval:     100 * time.Millisecond,
		MaxBatchInterval:     1 * time.Second,
		OfflineStorage:       true,
		StoragePath:          "tendrl_storage.db",
		OfflineRetryEnabled:  true,
		OfflineRetryInterval: 30 * time.Second,
		OfflineRetryLimit:    5,

		// Connectivity monitoring defaults
		ConnectivityCheckEnabled:  true,
		ConnectivityCheckInterval: 30 * time.Second,

		// Heartbeat defaults
		SendHeartbeat:     true,             // Enable by default in managed mode
		HeartbeatInterval: 30 * time.Second, // Send heartbeat every 30 seconds

		// Debug defaults
		Debug: false, // Disable debug logging by default
	}

	// Override with values from config file (if specified)
	if cf.Timeout > 0 {
		config.Timeout = time.Duration(cf.Timeout) * time.Second
	}
	if cf.MaxRetries > 0 {
		config.MaxRetries = cf.MaxRetries
	}
	if cf.MinBatchSize > 0 {
		config.MinBatchSize = cf.MinBatchSize
	}
	if cf.MaxBatchSize > 0 {
		config.MaxBatchSize = cf.MaxBatchSize
	}
	if cf.MaxQueueSize > 0 {
		config.MaxQueueSize = cf.MaxQueueSize
	}
	if cf.TargetCPUPercent > 0 {
		config.TargetCPUPercent = cf.TargetCPUPercent
	}
	if cf.TargetMemPercent > 0 {
		config.TargetMemPercent = cf.TargetMemPercent
	}
	if cf.MinBatchIntervalMs > 0 {
		config.MinBatchInterval = time.Duration(cf.MinBatchIntervalMs) * time.Millisecond
	}
	if cf.MaxBatchIntervalMs > 0 {
		config.MaxBatchInterval = time.Duration(cf.MaxBatchIntervalMs) * time.Millisecond
	}
	if cf.StoragePath != "" {
		config.StoragePath = cf.StoragePath
	}
	if cf.OfflineRetryInterval > 0 {
		config.OfflineRetryInterval = time.Duration(cf.OfflineRetryInterval) * time.Second
	}
	if cf.OfflineRetryLimit > 0 {
		config.OfflineRetryLimit = cf.OfflineRetryLimit
	}
	if cf.ConnectivityCheckInterval > 0 {
		config.ConnectivityCheckInterval = time.Duration(cf.ConnectivityCheckInterval) * time.Second
	}
	if cf.HeartbeatInterval > 0 {
		config.HeartbeatInterval = time.Duration(cf.HeartbeatInterval) * time.Second
	}

	// Booleans: nil means the file did not mention the key, so the default set
	// above stands. Only an explicit value overrides it.
	if cf.Managed != nil {
		config.Managed = *cf.Managed
	}
	if cf.OfflineStorage != nil {
		config.OfflineStorage = *cf.OfflineStorage
	}
	if cf.OfflineRetryEnabled != nil {
		config.OfflineRetryEnabled = *cf.OfflineRetryEnabled
	}
	if cf.ConnectivityCheckEnabled != nil {
		config.ConnectivityCheckEnabled = *cf.ConnectivityCheckEnabled
	}
	if cf.SendHeartbeat != nil {
		config.SendHeartbeat = *cf.SendHeartbeat
	}
	config.Debug = cf.Debug

	// Gating deliberately does NOT happen here. The constructors override
	// Managed from their own argument afterwards, so a teardown run at this
	// point would key off the file's flag rather than the mode the caller
	// actually asked for. Both constructors call ApplyManagedGating once the
	// effective mode is settled.
	return config
}

// ApplyManagedGating switches off everything that only exists to serve managed
// mode. Call it once Managed holds the value the client will really run with.
func (c *Config) ApplyManagedGating() {
	if c.Managed {
		return
	}
	c.OfflineStorage = false
	c.OfflineRetryEnabled = false
	c.ConnectivityCheckEnabled = false
	c.SendHeartbeat = false
}

// GetPlatformInfo returns platform information for User-Agent string
func GetPlatformInfo() string {
	goVersion := runtime.Version()
	osArch := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)

	// Try to get more detailed OS information
	osInfo := getDetailedOSInfo()
	if osInfo != "" {
		return fmt.Sprintf("Go/%s; %s", goVersion[2:], osInfo) // Remove "go" prefix
	}

	return fmt.Sprintf("Go/%s; %s", goVersion[2:], osArch)
}

// getDetailedOSInfo tries to get more detailed OS information
func getDetailedOSInfo() string {
	switch runtime.GOOS {
	case "linux":
		return getLinuxInfo()
	case "darwin":
		return getDarwinInfo()
	case "windows":
		return getWindowsInfo()
	default:
		return fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

// getLinuxInfo gets Linux distribution information
func getLinuxInfo() string {
	// Try to read /etc/os-release for distribution info
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		content := string(data)
		// Simple parsing for common fields
		if name := extractField(content, "PRETTY_NAME"); name != "" {
			return fmt.Sprintf("Linux (%s; %s)", name, runtime.GOARCH)
		}
		if id := extractField(content, "ID"); id != "" {
			if version := extractField(content, "VERSION_ID"); version != "" {
				return fmt.Sprintf("Linux (%s %s; %s)", id, version, runtime.GOARCH)
			}
			return fmt.Sprintf("Linux (%s; %s)", id, runtime.GOARCH)
		}
	}
	return fmt.Sprintf("Linux/%s", runtime.GOARCH)
}

// getDarwinInfo gets macOS version information
func getDarwinInfo() string {
	// Try to get macOS version from system
	// This is a simplified version - could be enhanced with system calls
	return fmt.Sprintf("macOS/%s", runtime.GOARCH)
}

// getWindowsInfo gets Windows version information
func getWindowsInfo() string {
	// This could be enhanced with Windows-specific system calls
	return fmt.Sprintf("Windows/%s", runtime.GOARCH)
}

// extractField extracts a field value from os-release format
func extractField(content, field string) string {
	lines := splitLines(content)

	for _, line := range lines {
		if len(line) > len(field)+1 && line[:len(field)] == field && line[len(field)] == '=' {
			value := line[len(field)+1:]
			// Remove quotes if present
			if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
				value = value[1 : len(value)-1]
			}
			return value
		}
	}
	return ""
}

// splitLines splits content by newlines
func splitLines(content string) []string {
	var lines []string
	var current string

	for _, char := range content {
		if char == '\n' {
			lines = append(lines, current)
			current = ""
		} else if char != '\r' {
			current += string(char)
		}
	}

	if current != "" {
		lines = append(lines, current)
	}

	return lines
}

// BuildUserAgent creates a User-Agent string similar to the Python client
func BuildUserAgent() string {
	return fmt.Sprintf("tendrl-go-sdk/%s", version)
}

// GetVersion returns the SDK version (hardcoded in module, not user-configurable)
func GetVersion() string {
	return version
}

// InitializeConfig creates a new configuration file with default values
func InitializeConfig(path string) error {
	config := GenerateExampleConfig()
	return SaveConfigFile(config, path)
}
