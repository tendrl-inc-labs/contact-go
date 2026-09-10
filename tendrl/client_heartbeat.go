package tendrl

// sendHeartbeat sends a heartbeat message with system resource information
// This is called automatically by the background queue processor
func (c *Client) sendHeartbeat() error {
	c.debugLog("Sending heartbeat")
	// Get system resource information
	resources, err := getSystemResources()
	if err != nil {
		c.debugLog("Failed to get system resources: %v", err)
		// If we can't get system resources, send heartbeat with zero values
		// This is better than not sending a heartbeat at all
		resources = HeartbeatData{
			MemFree:  0,
			MemTotal: 0,
			DiskFree: 0,
			DiskSize: 0,
		}
	} else {
		// HeartbeatData fields are float64 byte counts. %d on a float64 prints
		// %!d(float64=8.589934592e+09), so these are formatted as whole numbers.
		c.debugLog("Heartbeat data: mem_free=%.0f, mem_total=%.0f, disk_free=%.0f, disk_size=%.0f",
			resources.MemFree, resources.MemTotal, resources.DiskFree, resources.DiskSize)
	}

	// Send heartbeat using the helper method
	return c.PublishHeartbeat(resources)
}
