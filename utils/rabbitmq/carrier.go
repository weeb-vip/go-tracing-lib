package rabbitmq

// EventCarrier carries trace context in message headers
type EventCarrier struct {
	headers map[string]interface{}
}

// Get retrieves a value from the headers
func (c *EventCarrier) Get(key string) string {
	if val, ok := c.headers[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// Set sets a value in the headers
func (c *EventCarrier) Set(key, value string) {
	c.headers[key] = value
}

// Keys returns all header keys
func (c *EventCarrier) Keys() []string {
	keys := make([]string, 0, len(c.headers))
	for key := range c.headers {
		keys = append(keys, key)
	}
	return keys
}