// Package stream provides an event emitter implementation
// that serves as the base for the M3U8 parser's event-driven architecture
package stream

// EventListener represents a function that can handle events
type EventListener func(data interface{})

// Stream represents an event emitter
type Stream struct {
	listeners map[string][]EventListener
}

// NewStream creates a new Stream instance
func NewStream() *Stream {
	return &Stream{
		listeners: make(map[string][]EventListener),
	}
}

// On registers a listener for a specific event
func (s *Stream) On(event string, listener EventListener) {
	if s.listeners[event] == nil {
		s.listeners[event] = []EventListener{}
	}
	s.listeners[event] = append(s.listeners[event], listener)
}

// Trigger emits an event with data
func (s *Stream) Trigger(event string, data interface{}) {
	if listeners, ok := s.listeners[event]; ok {
		for _, listener := range listeners {
			listener(data)
		}
	}
}

// Pipe connects this stream to another object that can receive events
func (s *Stream) Pipe(destination interface{}) {
	if dest, ok := destination.(interface {
		Trigger(string, interface{})
	}); ok {
		s.On("data", func(data interface{}) {
			dest.Trigger("data", data)
		})

		s.On("end", func(data interface{}) {
			dest.Trigger("end", data)
		})
	}
}
