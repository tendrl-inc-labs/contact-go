package tendrl

import (
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

// updateMetrics updates system metrics for dynamic batching
func (c *Client) updateMetrics() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cpuPercent, _ := cpu.Percent(100*time.Millisecond, false)
			memStats, _ := mem.VirtualMemory()

			c.metrics.Lock()
			if len(cpuPercent) > 0 {
				c.metrics.CPUUsage = cpuPercent[0]
			}
			c.metrics.MemoryUsage = memStats.UsedPercent
			c.metrics.QueueLoad = float64(len(c.queue)) / float64(c.config.MaxQueueSize) * 100
			c.metrics.Unlock()

		case <-c.done:
			return
		}
	}
}

// calculateDynamicBatchSize calculates batch size based on system performance
func (c *Client) calculateDynamicBatchSize() int {
	c.metrics.RLock()
	defer c.metrics.RUnlock()

	cpuFactor := max(0.0, 1-(c.metrics.CPUUsage/c.config.TargetCPUPercent))
	memFactor := max(0.0, 1-(c.metrics.MemoryUsage/c.config.TargetMemPercent))
	queueFactor := min(1.0, c.metrics.QueueLoad/50)

	resourceFactor := (cpuFactor*0.4 + memFactor*0.4 + queueFactor*0.2)
	batchSize := int(float64(c.config.MaxBatchSize) * resourceFactor)

	return max(c.config.MinBatchSize, min(batchSize, c.config.MaxBatchSize))
}

// processQueue processes messages from the queue in dynamic batches
func (c *Client) processQueue() {
	c.debugLog("Starting queue processor")
	batch := make([]Message, 0, c.config.MaxBatchSize)
	ticker := time.NewTicker(c.config.MinBatchInterval)
	defer ticker.Stop()

	for {
		select {
		case msg := <-c.queue:
			batch = append(batch, msg)

			if len(batch) >= c.calculateDynamicBatchSize() {
				c.flushBatch(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				if _, err := c.sendMessages(batch, false); err != nil && c.storage != nil {
					// Store failed messages
					for _, m := range batch {
						if dataStr, err := c.dataAsString(m.Data); err == nil {
							var tags []string
							if m.Context != nil {
								tags = m.Context.Tags
							}
							c.storage.Store(
								time.Now().Format(time.RFC3339),
								dataStr,
								tags,
								3600,
							)
						}
					}
				}
				batch = batch[:0]
			}

			if c.storage != nil {
				c.storage.CleanupExpired()
			}

			// Send heartbeat if enabled and interval has passed
			if c.config.SendHeartbeat && c.IsOnline() {
				currentTime := time.Now()
				if currentTime.Sub(c.lastHeartbeat) >= c.config.HeartbeatInterval {
					if err := c.sendHeartbeat(); err == nil {
						c.lastHeartbeat = currentTime
					}
					// Don't fail on heartbeat errors - they shouldn't break the client
				}
			}

		case <-c.done:
			// Drain the channel before leaving. A select picks at random when
			// more than one case is ready, so messages can still be sitting in
			// c.queue at this point; flushing only the assembled batch
			// abandoned them, losing a final message roughly a quarter of the
			// time. Nothing raised and nothing was logged.
		drain:
			for {
				select {
				case msg := <-c.queue:
					batch = append(batch, msg)
				default:
					break drain
				}
			}
			if len(batch) > 0 {
				c.flushBatch(batch)
			}
			return
		}
	}
}

// flushBatch sends a batch and persists it when the send fails.
//
// The shutdown path used to call sendMessages directly and discard its error,
// so a final batch that failed to send was lost even with offline storage
// enabled. Both paths go through here now.
func (c *Client) flushBatch(batch []Message) {
	if len(batch) == 0 {
		return
	}
	if _, err := c.sendMessages(batch, false); err != nil {
		c.storeFailedBatch(batch)
	}
}

// storeFailedBatch persists messages that could not be sent, if there is
// anywhere to put them.
func (c *Client) storeFailedBatch(batch []Message) {
	if c.storage == nil {
		return
	}
	for _, m := range batch {
		dataStr, err := c.dataAsString(m.Data)
		if err != nil {
			continue
		}
		var tags []string
		if m.Context != nil {
			tags = m.Context.Tags
		}
		c.storage.Store(time.Now().Format(time.RFC3339), dataStr, tags, 3600)
	}
}
