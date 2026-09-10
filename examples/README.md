# Go SDK Examples

This directory contains example code demonstrating how to use the Tendrl Go SDK.

## Prerequisites

1. **Go 1.25 or later** installed
2. **API Key**: Set the `TENDRL_KEY` environment variable with your Tendrl API key:

   ```bash
   export TENDRL_KEY="your_api_key_here"
   ```

## Building

From the `examples` directory:

```bash
go build -o tendrl-example main.go
```

Or build and run directly:

```bash
go run main.go
```

## Running

### Basic Mode

Run the example in normal mode:

```bash
./tendrl-example
# or
go run main.go
```

### Demo Mode

Run with demo mode for more verbose output:

```bash
./tendrl-example demo
# or
go run main.go demo
```

## What This Example Demonstrates

This example showcases the following SDK features:

1. **Managed Client Setup**: Creates a managed client with queuing, batching, and offline storage
2. **Message Callbacks**: Sets up a callback to handle incoming messages from the server
3. **Tether Function**: Uses the `Tether()` method to periodically collect and publish data
4. **Message Publishing**: Demonstrates publishing messages with tags
5. **Command Handling**: Shows how to handle different message types (commands, notifications, requests)
6. **Error Handling**: Includes proper error handling and client cleanup

## Example Features

- **User Signup Metrics**: Simulates collecting user signup data every 5 seconds
- **System Metrics**: Collects system metrics (CPU, memory, disk) every 10 seconds
- **Command Processing**: Handles incoming commands from the server
- **Message Types**: Demonstrates handling commands, notifications, and requests

## Expected Output

When running, you should see:

- Client initialization messages
- Periodic data collection messages
- Incoming message notifications (when messages are received from the server)
- System metrics and status updates

## Troubleshooting

### "no API key provided" error

Make sure you've set the `TENDRL_KEY` environment variable:

```bash
export TENDRL_KEY="your_api_key_here"
```

### Build errors

If you encounter import errors, make sure you're in the `examples` directory and the `go.mod` file includes the replace directive for local development:

```go
replace github.com/tendrl-inc-labs/contact-go => ../
```

Then run:

```bash
go mod tidy
```

### Connection errors

Ensure you have internet connectivity and that your API key is valid.

## Next Steps

- Modify the example to match your use case
- Add your own data collection functions
- Customize message handling logic
- Explore the full SDK API in the main README
