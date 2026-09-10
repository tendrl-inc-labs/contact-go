package tendrl

// MessageRoute matches inbound messages by msg_type and/or tags.
// All non-empty criteria must match (AND semantics).
type MessageRoute struct {
	MsgType string
	Tag     string
	Tags    []string // ANY
	TagsAll []string // ALL
	Handler MessageCallback
}

func extractMessageTags(msg IncomingMessage) []string {
	if msg.Context.Tags != nil {
		return msg.Context.Tags
	}
	return nil
}

func routeMatches(route MessageRoute, msg IncomingMessage) bool {
	if route.MsgType != "" && msg.MsgType != route.MsgType {
		return false
	}
	msgTags := extractMessageTags(msg)
	if route.Tag != "" && !containsTag(msgTags, route.Tag) {
		return false
	}
	if len(route.Tags) > 0 && !anyTag(msgTags, route.Tags) {
		return false
	}
	if len(route.TagsAll) > 0 && !allTags(msgTags, route.TagsAll) {
		return false
	}
	return true
}

func containsTag(msgTags []string, tag string) bool {
	for _, t := range msgTags {
		if t == tag {
			return true
		}
	}
	return false
}

func anyTag(msgTags, tags []string) bool {
	for _, tag := range tags {
		if containsTag(msgTags, tag) {
			return true
		}
	}
	return false
}

func allTags(msgTags, tags []string) bool {
	for _, tag := range tags {
		if !containsTag(msgTags, tag) {
			return false
		}
	}
	return true
}

func (c *Client) hasMessageHandlers() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.routes) > 0 || c.defaultHandler != nil || c.callback != nil
}

func (c *Client) dispatchMessage(msg IncomingMessage) error {
	// Snapshot under the lock, then call handlers with it released: a handler is
	// user code and may itself register a route, which would otherwise deadlock.
	c.mu.RLock()
	routes := make([]MessageRoute, len(c.routes))
	copy(routes, c.routes)
	defaultHandler := c.defaultHandler
	callback := c.callback
	c.mu.RUnlock()

	for _, route := range routes {
		if routeMatches(route, msg) {
			return route.Handler(msg)
		}
	}
	if defaultHandler != nil {
		return defaultHandler(msg)
	}
	if callback != nil {
		return callback(msg)
	}
	return nil
}

// On registers a route handler for incoming messages.
// Routes are checked in registration order; first match wins.
func (c *Client) On(route MessageRoute) {
	if route.Handler == nil {
		return
	}
	c.mu.Lock()
	c.routes = append(c.routes, route)
	c.mu.Unlock()
}

// OnDefault registers a catch-all handler when no route matches.
func (c *Client) OnDefault(handler MessageCallback) {
	c.mu.Lock()
	c.defaultHandler = handler
	c.mu.Unlock()
}
