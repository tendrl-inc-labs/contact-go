# Tendrl Go SDK

[![Version](https://img.shields.io/badge/version-0.1.0-blue.svg)](https://github.com/tendrl-inc-labs/contact-go)
[![Go Version](https://img.shields.io/badge/go-1.25+-00ADD8.svg)](https://golang.org/doc/devel/release.html)
[![License](https://img.shields.io/badge/license-MIT%20%2B%20Commons%20Clause-yellow.svg)](LICENSE)

A simple, flexible SDK for high-performance messaging.

## License Notice

This software is distributed under the **MIT License with Commons Clause and Client Use Restriction**. The full terms are in [LICENSE](LICENSE); the summary below is not a substitute for them.

### Allowed

- Use, copy, modify, merge, publish, distribute and sublicense the software, subject to the conditions below
- Use the software with services operated by or on behalf of Tendrl, Inc.
- Inspect and study the code to understand its design

### Not allowed

- Selling the software, or using it to provide or host a commercial product or service (SaaS, PaaS, resold or white-labeled) where it is a substantial component
- Using this client, or any derivative, to access or interoperate with any service other than Tendrl's
- Reusing its design patterns, protocol logic or architecture in another product without prior written permission
- Any use that competes with Tendrl, Inc., directly or indirectly

For licensing questions, contact: `support@tendrl.com`

## Features

- **Flexibility**: Works with any JSON-serializable type - strings (automatically wrapped), maps (any key/value types), structs, arrays, primitives - no complex formatting
- **Offline Message Storage**: BoltDB-based persistence with TTL (opt-in, see [Configuration File](#configuration-file))
- **Automatic Retry**: Background retry process for stored offline messages
- **Automatic Heartbeats**: System resource monitoring with automatic heartbeat messages (managed mode)
- **Inbound Routing**: Tag- and type-based routing for messages the backend has queued for this entity
- **File Transfer**: Send and receive scanned files between entities and accounts
- **Dynamic Batch Processing**: CPU/memory-aware batching (10-500 messages)
- **Thread-Safe Operations**: Concurrent-safe message handling
- **Focused API**: `Publish()` covers ordinary messaging, sync and async; heartbeats, cross-account sends and file transfer are separate methods

**Benefits:**

- Simple setup with no additional components
- Talks to the Tendrl API over HTTP directly - HTTPS in production, or plain HTTP against a local stack via `TENDRL_APP_URL`
- Retries transient failures (5xx and 429) up to `max_retries` times
- Dynamic batching based on system resources
- Configurable request timeout and retry count
- Clean code structure with separated models and client logic

## Installation

```bash
go get github.com/tendrl-inc-labs/contact-go@latest
```

## Configuration

The Go SDK supports multiple ways to configure the client.

### 1. Environment Variables

```bash
export TENDRL_KEY="your_api_key_here"
```

### 2. API Key Parameters

For programmatic use, you can pass the API key directly:

```go
// Managed mode with API key parameter (recommended)
client, err := tendrl.NewClient(true, "your_api_key_here")

// Managed mode using environment variable (apiKey is optional)
client, err = tendrl.NewClient(true)

// Headless mode with API key parameter
client, err = tendrl.NewClient(false, "your_api_key_here")

// Headless mode using environment variable
client, err = tendrl.NewClient(false)
if err != nil {
    log.Fatal(err)
}
defer client.Stop()
```

### 3. Constructors

| Constructor | Description |
|-------------|-------------|
| `NewClient(managed bool, apiKey ...string)` | Primary constructor. Mode is explicit; the API key is optional and falls back to `TENDRL_KEY` |
| `NewClientWithMode(managed bool)` | Same, always using `TENDRL_KEY` |
| `NewClientWithModeAndAPIKey(managed bool, apiKey string)` | Same, with an explicit (possibly empty) key |
| `NewClientWithConfig(configPath string)` | Loads a specific config file. **Mode comes from the file's `managed` key**, not a parameter |
| `NewClientWithConfigAndAPIKey(configPath, apiKey string)` | Same, with an explicit key |

The `NewClientWithConfig*` constructors read only the file at the path given; they do not fall back to the default search paths, and a missing or invalid file is an error.

### 4. Configuration Priority

**API Key Sources (highest to lowest priority):**

1. **API Key Parameter** (passed to constructor)
2. **Environment Variable** (`TENDRL_KEY`)

**Mode Parameter:**

- `true` = Managed mode (background queuing, batching, metrics) - **Recommended**
- `false` = Headless mode (immediate API calls only)

### Configuration File

You can also configure the client using a JSON configuration file. With the `NewClient*` constructors the SDK searches, in order, and uses the first file it can read:

1. `~/.tendrl/config.json`
2. `/etc/tendrl/config.json` (non-Windows only)

If neither exists, built-in defaults apply. To load a file from anywhere else, pass its path to `NewClientWithConfig` or `NewClientWithConfigAndAPIKey`.

```json
{
  "managed": true,
  "timeout_seconds": 10,
  "max_retries": 3,
  "debug": false,
  "send_heartbeat": true,
  "heartbeat_interval_seconds": 30,
  "offline_storage": true,
  "storage_path": "tendrl_storage.db",
  "offline_retry_enabled": true,
  "offline_retry_interval_seconds": 30,
  "connectivity_check_enabled": true,
  "connectivity_check_interval_seconds": 30
}
```

#### All configuration keys

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `managed` | bool | `false` | Managed mode. Ignored by `NewClient*` (the mode parameter wins); **required** for managed mode with `NewClientWithConfig*` |
| `timeout_seconds` | int | `10` | HTTP request timeout |
| `max_retries` | int | `3` | Retry attempts for a failed send. Backoff is a fixed 1s, 2s, 3s... and is not configurable |
| `debug` | bool | `false` | Enable debug logging to stderr |
| `min_batch_size` | int | `10` | Lower clamp on the dynamic batch size |
| `max_batch_size` | int | `500` | Upper clamp on the dynamic batch size |
| `max_queue_size` | int | `1000` | Capacity of the in-memory publish queue |
| `target_cpu_percent` | float | `70.0` | CPU load at which batching backs off |
| `target_mem_percent` | float | `80.0` | Memory load at which batching backs off |
| `min_batch_interval_ms` | int | `100` | Flush interval for a partial batch |
| `max_batch_interval_ms` | int | `1000` | Reserved for batch timing; currently only `min_batch_interval_ms` drives the flush ticker |
| `offline_storage` | bool | `false` | Persist messages to BoltDB when a send fails |
| `storage_path` | string | `tendrl_storage.db` | BoltDB file path |
| `offline_retry_enabled` | bool | `false` | Run the background retry loop for stored messages |
| `offline_retry_interval_seconds` | int | `30` | How often that loop runs |
| `offline_retry_limit` | int | `5` | **Currently inert.** Parsed and stored, but no code reads it; a stored message is retried until it succeeds or its 1-hour TTL expires |
| `connectivity_check_enabled` | bool | `false` | Run background connectivity probes against `/health` |
| `connectivity_check_interval_seconds` | int | `30` | How often those probes run |
| `send_heartbeat` | bool | `true` when `managed` is true | Send automatic heartbeats |
| `heartbeat_interval_seconds` | int | `30` | Interval between heartbeats |

**Important - booleans default to off.** Every boolean above except `send_heartbeat` defaults to `false` when the key is absent, and the managed-mode features keyed off them are disabled along with it. In particular, a client built with `tendrl.NewClient(true)` and **no config file** gets queuing, batching and metrics, but **not** offline storage, offline retry, connectivity checks or heartbeats. To get those, write a config file that sets `"managed": true` along with the features you want.

To generate an example configuration file with every feature enabled:

```go
// Generate an example config at the default location (~/.tendrl/config.json)
if err := tendrl.InitializeConfig(tendrl.GetDefaultConfigPath()); err != nil {
    log.Fatal(err)
}
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `TENDRL_KEY` | API key for authentication | "" |
| `TENDRL_APP_URL` | Base URL of the Tendrl API | `https://app.tendrl.com/api` |

`TENDRL_APP_URL` accepts either a bare origin or a full base URL. Trailing slashes are trimmed, and `/api` is appended if it is not already the last path segment - so `http://192.168.1.50:8000`, `http://192.168.1.50:8000/`, and `http://192.168.1.50:8000/api` all resolve to `http://192.168.1.50:8000/api`. Point it at a staging or local stack to test against something other than production; plain HTTP is accepted for local use.

**API Key Priority:**

1. **API Key Parameter** (passed to constructor)
2. **Environment Variable** (`TENDRL_KEY`)

**Examples:**

```bash
export TENDRL_KEY="your_api_key_here"

# Optional: talk to a local stack instead of production
export TENDRL_APP_URL="http://192.168.1.50:8000"
```

**Note:** Storage path, debug and the other settings have no environment variables; use the config file. See [Configuration File](#configuration-file).

## Operating Modes

The Go SDK supports two operating modes. Both speak HTTP to the Tendrl API directly - unlike the Python SDK there is no agent-socket mode, because the Go SDK is intended to be the agent.

### Managed Mode (Recommended)

#### Background processing

- **Automatic** API key validation on startup
- **Automatic** message queuing and dynamic batching
- **Automatic** background system monitoring and resource-aware batch sizing
- **Automatic** inbound message and state polling, once a handler is registered
- **Optional** offline storage with retry (`offline_storage`, `offline_retry_enabled`)
- **Optional** connectivity monitoring (`connectivity_check_enabled`)
- **Optional** heartbeat messages with system resource information (`send_heartbeat`)

Note that the optional features are only active when a config file turns them on; see [Configuration File](#configuration-file).

### Headless Mode

#### Lightweight immediate API calls

- Direct HTTP requests only
- No background processes and no API key validation on startup
- Minimal resource usage
- Synchronous and asynchronous publishing
- No queuing, batching, offline storage or inbound polling (`CheckMessages` and `CheckState` still work when called directly)

## Quick Start

### Method 1: Using Environment Variables

```go
package main

import (
    "log"

    tendrl "github.com/tendrl-inc-labs/contact-go/tendrl"
)

func main() {
    // Create managed client (reads TENDRL_KEY environment variable)
    client, err := tendrl.NewClient(true) // true = managed mode, apiKey optional - uses TENDRL_KEY env var
    if err != nil {
        log.Fatal(err)
    }
    defer client.Stop()

    if err := client.PublishAsync("hello from Go", []string{"demo"}); err != nil {
        log.Printf("publish failed: %v", err)
    }
}
```

### Method 2: Using API Key Parameters (Recommended for Programmatic Use)

```go
package main

import (
    "log"

    tendrl "github.com/tendrl-inc-labs/contact-go/tendrl"
)

func main() {
    // Create managed client with API key parameter (no environment variable needed)
    client, err := tendrl.NewClient(true, "your_api_key_here") // true = managed mode
    if err != nil {
        log.Fatal(err)
    }
    defer client.Stop()

    if err := client.PublishAsync("hello from Go", []string{"demo"}); err != nil {
        log.Printf("publish failed: %v", err)
    }
}
```

## API Reference

### Publishing Messages

```go
// Publish with optional response waiting
messageID, err := client.Publish(
    data,                     // Any JSON-serializable data
    []string{"tag1", "tag2"}, // Tags
    "entity_name",            // Target entity (empty string for default)
    true,                     // waitResponse
    10,                       // response timeout in seconds
)
if err != nil {
    log.Fatal(err)
}
fmt.Println("message ID:", messageID)

// Async publishing (fire-and-forget): equivalent to
// Publish(data, tags, "", false, 5) with the message ID discarded
if err := client.PublishAsync(data, []string{"tag1", "tag2"}); err != nil {
    log.Printf("publish failed: %v", err)
}
```

`Publish` returns a non-empty message ID only when `waitResponse` is `true`. With `waitResponse` false in managed mode the message is queued for batching and the returned ID is empty; in headless mode it is sent immediately. If the queue is full, the message is written to offline storage when that is enabled, and otherwise an error is returned.

### Helper Methods

```go
// Publish heartbeat with system resource information (manual)
heartbeatData := tendrl.HeartbeatData{
    MemFree:  15728640,   // Available RAM in bytes
    MemTotal: 8388608,    // Total RAM in bytes
    DiskFree: 536870912,  // Available filesystem space in bytes
    DiskSize: 1073741824, // Total filesystem size in bytes
}
if err := client.PublishHeartbeat(heartbeatData); err != nil {
    log.Printf("heartbeat failed: %v", err)
}

// Publish sensor data (convenience wrapper over Publish)
sensorData := map[string]interface{}{
    "temperature": 23.5,
    "humidity":    60.2,
    "pressure":    1013.25,
}
if err := client.PublishSensorData(sensorData, []string{"sensor", "environment"}); err != nil {
    log.Printf("sensor publish failed: %v", err)
}

// Cross-account messaging (send to another entity)
destination := "123:us-1:entity:target-entity"
if err := client.PublishCrossAccount(
    map[string]interface{}{"command": "restart"},
    destination,
    []string{"urgent", "command"},
); err != nil {
    log.Printf("cross-account publish failed: %v", err)
}
```

`PublishHeartbeat` and `PublishCrossAccount` bypass the queue and send immediately in both modes.

Automatic heartbeats are covered in [Automatic Heartbeats](#automatic-heartbeats).

### File Transfer

Files are uploaded to the Tendrl API, scanned by Surface, and only become downloadable once they are clean. These methods work in either mode.

```go
// Send a file from disk to another entity in the same account
result, err := client.SendFile("/var/spool/clip.mp4", "camera-hub", nil)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("transfer %s: %s (%d bytes, %s)\n",
    result.TransferID, result.Status, result.Size, result.ThreatLevel)

// Route by tags instead of a destination - matching Strand automations receive it
if _, err := client.SendFile("/var/spool/clip.mp4", "", []string{"clips", "driveway"}); err != nil {
    log.Printf("tag-routed send failed: %v", err)
}

// Attach custom metadata to the transfer (optional trailing argument)
if _, err := client.SendFile("/var/spool/clip.mp4", "camera-hub", nil, map[string]any{
    "zone":    "driveway",
    "trigger": "PIR",
}); err != nil {
    log.Printf("send with metadata failed: %v", err)
}

// Send bytes you already have in memory
if _, err := client.SendFileBytes("reading.csv", []byte("t,v\n1,2\n"), "analytics", nil); err != nil {
    log.Printf("byte send failed: %v", err)
}

// List clean files waiting for this entity
files, err := client.CheckFiles(50) // limit <= 0 defaults to 50
if err != nil {
    log.Fatal(err)
}
for _, f := range files {
    fmt.Println(f["transfer_id"], f["file_name"])
}

// Download one. For delete_on_download files (the default) this consumes it.
contents, err := client.DownloadFile("transfer-id-here")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("downloaded %d bytes\n", len(contents))

// Re-scan a received cross-account file with this account's own Surface profile.
// Billed to the recipient; only the recipient may call it.
verdict, err := client.RescanFile("transfer-id-here")
if err == nil && verdict.Blocked {
    fmt.Println("blocked by our profile:", verdict.Threat)
}
```

Pass exactly one of `dest` or `tags`. A `dest` may be a bare entity name or a full `account:region:entity:name` resource path; an entity-group destination broadcasts to its members, and a destination in another account is a cross-account transfer (which the recipient must have opted into and allowlisted). A non-2xx response - 402 credits, 403 not accepted, 415 unsupported type, 422 blocked - comes back as an error.

**`FileResult`**

| Field | Type | Description |
|-------|------|-------------|
| `TransferID` | `string` | Identifier used to download or re-scan the file |
| `Status` | `string` | Terminal scan state (`clean`, or `awaiting_fetch` for tag-routed uploads) |
| `Mode` | `string` | `direct`, `tag`, `group` or `cross_account` |
| `FileName` | `string` | Stored file name |
| `Size` | `int64` | Size in bytes |
| `SHA256` | `string` | Content hash |
| `ThreatLevel` | `string` | Scan verdict |
| `Delivered` | `int` | Group broadcast only: number of recipients |
| `Skipped` | `[]string` | Group broadcast only: recipients skipped |

**`RescanResult`**

| Field | Type | Description |
|-------|------|-------------|
| `TransferID` | `string` | The re-scanned transfer |
| `RecipientThreatLevel` | `string` | Verdict under the recipient's profile |
| `Threat` | `string` | Threat name, if any |
| `Blocked` | `bool` | True when the recipient's profile flagged it; it is no longer downloadable on this side |

### Inbound Message Routing

Route incoming messages with `client.On()`:

```go
client.On(tendrl.MessageRoute{
    Tag: "ai-response",
    Handler: func(message tendrl.IncomingMessage) error {
        fmt.Printf("AI: %v\n", message.Data)
        return nil
    },
})

client.On(tendrl.MessageRoute{
    Tags: []string{"alert", "anomaly"}, // ANY of these tags
    Handler: func(message tendrl.IncomingMessage) error {
        fmt.Printf("Alert: %v\n", message.Data)
        return nil
    },
})

client.On(tendrl.MessageRoute{
    MsgType: "cmd",                          // and/or match on message type
    TagsAll: []string{"urgent", "restart"},  // ALL of these tags
    Handler: func(message tendrl.IncomingMessage) error {
        fmt.Printf("Command: %v\n", message.Data)
        return nil
    },
})

client.OnDefault(func(message tendrl.IncomingMessage) error {
    fmt.Printf("Unhandled: %s\n", message.MsgType)
    return nil
})

client.SetMessageCheckRate(5 * time.Second)
client.SetMessageCheckLimit(10)
```

`SetMessageCallback` remains available as a catch-all fallback for unmatched messages.

**`MessageRoute` fields**

| Field | Type | Description |
|-------|------|-------------|
| `MsgType` | `string` | Match this `msg_type` exactly |
| `Tag` | `string` | Message must carry this tag |
| `Tags` | `[]string` | Message must carry **any** of these tags |
| `TagsAll` | `[]string` | Message must carry **all** of these tags |
| `Handler` | `MessageCallback` | Called on a match. A route with no handler is ignored |

All non-empty criteria must match (AND semantics between fields).

### Inbound state (`OnState`)

Poll `GET /entities/status-table` at the same interval as messages. The handler fires when the table changes (not on the first poll, which establishes the baseline):

```go
client.OnState(func(state map[string]interface{}) error {
    fmt.Println("State:", state)
    return nil
})
```

`SetStateCallback` remains available as a catch-all fallback. `client.CheckState()` polls once on demand in either mode.

### IncomingMessage Structure

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `msg_type` | `string` | Message type identifier (e.g. "publish", "cmd", "notification") | Yes |
| `source` | `string` | Sender's resource path (set by server) | Yes |
| `dest` | `string` | Destination entity identifier | Optional |
| `timestamp` | `string` | RFC3339 timestamp (set by server) | Yes |
| `data` | `interface{}` | The actual message payload (can be any JSON type) | Yes |
| `context` | `IncomingMessageContext` | Message metadata | Optional |
| `request_id` | `string` | Request identifier (if message was a request) | Optional |

### Message Context Structure

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `tags` | `[]string` | Message tags for categorization | Optional |
| `dynamicActions` | `map[string]interface{}` | Server-side validation results | Optional |

#### How It Works

1. **Background Checking**: In managed mode the SDK polls for messages every 3 seconds by default, but only once at least one handler is registered - a client with no handlers issues no polling requests
2. **Check Rate**: `SetMessageCheckRate` takes effect immediately, including on an already-running poller
3. **Manual Checking**: You can call `CheckMessages()` and `CheckState()` yourself in any mode
4. **Route Matching**: Registered `On()` routes are checked in registration order; first match wins
5. **Fallback**: Unmatched messages go to `OnDefault`, then `SetMessageCallback`, if set
6. **Error Handling**: A handler returning an error does not stop the remaining messages from being processed

### Tethering Functions to the Cloud

**Note**: Core background processing (queuing, batching, metrics, retry) starts automatically in managed mode. `Tether` is for **additional** user-defined periodic data collection.

```go
// Only available in managed mode - returns a no-op stop function in headless mode

// Method 1: Tether a heartbeat function
stopHeartbeat := client.Tether("heartbeat", func() (interface{}, error) {
    return "heartbeat", nil
}, []string{"health"}, 1*time.Minute)
defer stopHeartbeat() // Clean up when done

// Method 2: Tether custom metrics collection
stopMetrics := client.Tether("custom_metrics", func() (interface{}, error) {
    return map[string]interface{}{
        "custom_value": 42.5,
        "app_status":   "running",
    }, nil
}, []string{"custom"}, 30*time.Second)
defer stopMetrics()

// Method 3: Tether an existing function
getAppMetrics := func() (interface{}, error) {
    return map[string]interface{}{
        "active_users":     150,
        "requests_per_sec": 45.2,
    }, nil
}

stopAppMetrics := client.Tether("app_metrics", getAppMetrics, []string{"application"}, 1*time.Minute)
defer stopAppMetrics()
```

Tethered results are published fire-and-forget; a collection function returning an error is skipped for that tick.

### Flexible Data Types

The SDK works with any JSON-serializable data type. `PublishAsync` is used below for brevity; its error return is omitted in these snippets but should be checked in real code.

```go
// Strings are automatically wrapped in {"data": "..."} to match backend expectations
client.PublishAsync("Simple log message", []string{"logs"})
// This sends: {"msg_type": "publish", "data": {"data": "Simple log message"}, ...}

// Any map type works (not just map[string]interface{})
client.PublishAsync(map[string]interface{}{
    "user_id": 12345,
    "action":  "login",
}, []string{"user", "events"})

client.PublishAsync(map[string]string{
    "name":  "John Doe",
    "email": "john@example.com",
    "city":  "New York",
}, []string{"user", "profile"})

client.PublishAsync(map[string]int{
    "age":   30,
    "score": 95,
    "level": 5,
}, []string{"user", "stats"})

client.PublishAsync(map[int]string{
    1: "first",
    2: "second",
    3: "third",
}, []string{"rankings"}) // Keys become strings in JSON

// Structs get JSON serialized
type Event struct {
    Type string `json:"type"`
    Data string `json:"data"`
}
client.PublishAsync(Event{Type: "error", Data: "Something went wrong"}, []string{"errors"})

// Arrays and slices work
client.PublishAsync([]string{"item1", "item2", "item3"}, []string{"arrays"})
client.PublishAsync([]int{1, 2, 3, 4, 5}, []string{"numbers"})

// Numbers work too
client.PublishAsync(42, []string{"numbers"})
client.PublishAsync(3.14159, []string{"numbers"})
client.PublishAsync(true, []string{"booleans"})

// Complex nested structures
client.PublishAsync([]interface{}{
    "string",
    map[string]interface{}{"key": "value"},
    42,
    true,
    []string{"nested", "array"},
}, []string{"mixed"})

// Maps with different value types
client.PublishAsync(map[string]interface{}{
    "name":     "Product A",
    "price":    29.99,
    "in_stock": true,
    "tags":     []string{"electronics", "gadget"},
    "metadata": map[string]string{
        "color": "black",
        "size":  "medium",
    },
}, []string{"products"})
```

**Important**: The data must be JSON-serializable. These compile, but fail at send time because `encoding/json` cannot marshal them:

```go
// Not JSON-serializable - these return an error:
client.PublishAsync(map[string]func(){"callback": myFunc}, []string{"invalid"})
client.PublishAsync(map[string]chan int{"ch": myChan}, []string{"invalid"})

// But these work perfectly:
client.PublishAsync(map[string][]int{"scores": {95, 87, 92}}, []string{"valid"})
client.PublishAsync(map[int]bool{1: true, 2: false}, []string{"valid"})
```

### Status and Diagnostics

```go
// Get current system metrics (managed mode; zero values otherwise)
metrics := client.GetSystemMetrics()
fmt.Printf("CPU: %.1f%%, Memory: %.1f%%, Queue: %.1f%%\n",
    metrics.CPUUsage, metrics.MemoryUsage, metrics.QueueLoad)

// Get offline storage stats
stats := client.GetOfflineStorageStats()
fmt.Printf("Offline messages: %d, Retry enabled: %v\n",
    stats.MessageCount, stats.RetryEnabled)

// Get connectivity state (managed mode; zero values otherwise)
connectivity := client.GetConnectivityState()
fmt.Printf("Online: %v, Last Check: %v\n",
    connectivity.Online, connectivity.LastCheck.Format("15:04:05"))

// Last known connectivity, without the struct. Always true in headless mode.
if !client.IsOnline() {
    fmt.Println("offline - publishes will be stored if offline storage is on")
}

// SDK version and the platform string used in the User-Agent
fmt.Println(tendrl.GetVersion())      // e.g. 0.1.0
fmt.Println(tendrl.GetPlatformInfo()) // e.g. Go/1.25.0; macOS/arm64
```

## Automatic Heartbeats

In managed mode with heartbeats enabled, the SDK sends heartbeat messages carrying system resource information:

```go
// Heartbeats are driven by the config file, not by constructor arguments.
client, err := tendrl.NewClientWithConfigAndAPIKey("config.json", "api_key")
if err != nil {
    log.Fatal(err)
}
defer client.Stop()

// In config.json:
//   "managed": true                        -> heartbeats default to on
//   "send_heartbeat": false                -> turn them off
//   "heartbeat_interval_seconds": 60       -> change the interval (default 30)
```

**Heartbeat Features:**

- On by default once a config file enables managed mode; see the boolean caveat under [Configuration File](#configuration-file)
- Uses real system metrics via gopsutil (available/total memory, free/total filesystem space)
- Sends every 30 seconds by default, on the queue processor's tick
- Only sent while the client believes it is online
- Sent immediately rather than queued, and bypasses batching
- Disabled in headless mode
- Errors are swallowed so a failing heartbeat cannot break the client

## Offline Storage & Retry

When `offline_storage` is enabled, the SDK stores messages in BoltDB if a send fails or the queue is full, and retries them when `offline_retry_enabled` is on.

### Message TTL (Time To Live)

**All offline messages have a TTL of 1 hour (3600 seconds).**

- Messages stored offline are automatically assigned a 1-hour expiration time
- Expired messages are automatically cleaned up and will not be retried
- TTL cleanup happens:
  - Periodically during queue processing
  - During retry operations (expired messages are skipped and deleted)
- This prevents indefinite storage of old messages that may no longer be relevant

**TTL Behavior:**

- Messages stored when offline: TTL starts from storage timestamp
- If a message expires before it can be sent, it's automatically deleted
- No manual cleanup required - the SDK handles expiration automatically

**Offline Retry Flow:**

```sh
Network Down → Store Messages in BoltDB (with 1-hour TTL)
                        ↓
    Background Retry Process (every 30s by default,
    offline_retry_interval_seconds; no jitter)
                        ↓
                Network Available? ──No──→ Continue Checking
                        ↓ Yes
        Retrieve Stored Messages (50 per batch, 5 batches per cycle)
                        ↓
            Check TTL ──Expired──→ Delete & Skip
                        ↓ Valid
                  Send in Batches
                        ↓
                   Success? ──No──→ Keep for Next Retry
                        ↓ Yes
               Delete from Storage
                        ↓
            Continue Normal Operation
```

## Debug Mode

The Go SDK includes a debug mode that provides detailed logging of SDK operations. This is useful for troubleshooting, development, and understanding SDK behavior.

### Enabling Debug Mode

Debug mode is set in the configuration file; there is no environment variable or constructor argument for it:

```json
{
  "debug": true
}
```

### What Gets Logged

When debug mode is enabled, the SDK logs:

- **Client Initialization**: Resolved base URL and API key validation
- **Message Publishing**: All publish operations with details (tags, destination, wait response)
- **API Requests**: HTTP endpoints being called, request/response status
- **Message Callbacks**: Incoming messages received and processed
- **Queue Processing**: Queue processor startup
- **Connectivity**: Network state changes (online/offline transitions)
- **Heartbeats**: Automatic heartbeat sending with system resource data
- **Entity Status**: Online/offline status updates on start and stop

### Example Debug Output

```bash
[DEBUG] API base URL: https://app.tendrl.com/api
[DEBUG] Validating API key
[DEBUG] API key validated successfully
[DEBUG] Publishing message (tags=[sensor temperature], entity=, waitResponse=false)
[DEBUG] Sending 1 message(s) (waitResponse=false)
[DEBUG] Sending request to: https://app.tendrl.com/api/entities/messages
[DEBUG] Message(s) sent successfully (status=200)
[DEBUG] Checking for incoming messages (limit=1)
[DEBUG] Received 2 message(s)
[DEBUG] Processing message: type=cmd, source=123:us-1:entity:sender
[DEBUG] Sending heartbeat
[DEBUG] Heartbeat data: mem_free=8589934592, mem_total=17179869184, disk_free=536870912000, disk_size=1073741824000
[DEBUG] Connectivity changed: online -> offline
[DEBUG] Connectivity changed: offline -> online
[DEBUG] TendrlClient stopping
[DEBUG] TendrlClient stopped
```

### Disabling Debug Mode

Debug mode is disabled by default. To disable it explicitly:

```json
{
  "debug": false
}
```

**Note**: Debug logging uses Go's standard `log` package and outputs to `stderr`. In production environments, you may want to redirect or filter debug output.

## Error Handling

```go
// Basic error handling
if err := client.PublishAsync(data, tags); err != nil {
    log.Printf("Failed to publish: %v", err)
}

// With retries for critical data
for attempts := 0; attempts < 3; attempts++ {
    if err := client.PublishAsync(criticalData, tags); err == nil {
        break // Success
    }
    time.Sleep(time.Duration(attempts+1) * time.Second)
}
```

The SDK already retries a failed send `max_retries` times internally, on transient failures only: network errors, 5xx responses and 429. A 4xx is returned immediately.

## Security Best Practices

Keep API keys in the environment or pass them as parameters:

```bash
export TENDRL_KEY=your_api_key_here
```

```go
client, _ := tendrl.NewClient(true) // Reads TENDRL_KEY automatically
defer client.Stop()

explicit, _ := tendrl.NewClient(true, "your_api_key") // Or pass it explicitly
defer explicit.Stop()
```

API keys are never read from config files - config files hold only non-sensitive settings.

## Compatibility

- Go 1.25+ (`go.mod` declares `go 1.25.0`; dependencies require it, so older toolchains cannot build the module)
- All major operating systems (Linux, macOS, Windows)
- Both amd64 and arm64 architectures

## License

Copyright (c) Tendrl, Inc. 2025-2026.

Licensed under the MIT License with Commons Clause and Client Use Restriction. See [LICENSE](LICENSE) for the full terms and [License Notice](#license-notice) above for a summary.
