package redis

type EventCarrier struct {
	headers map[string]string
}

func (e *EventCarrier) Set(key string, value string) {
	e.headers[key] = value
}

func (e *EventCarrier) Get(key string) string {
	return e.headers[key]
}

func (e *EventCarrier) Keys() []string {
	keys := make([]string, 0, len(e.headers))
	for k := range e.headers {
		keys = append(keys, k)
	}
	return keys
}
func NewEventCarrier(headers *map[string]string) *EventCarrier {
	if headers != nil {
		return &EventCarrier{
			headers: *headers,
		}
	}
	return &EventCarrier{
		headers: make(map[string]string),
	}
}

type RedisMessage[T any] interface {
	Headers() map[string]string
	SetHeaders(map[string]string)
	GetMsg() T
}
