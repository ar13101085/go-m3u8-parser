// Package linestream provides functionality for processing text line by line
package linestream

import (
	"strings"

	"github.com/ar13101085/go-m3u8-parser/m3u8/stream"
)

// LineStream is a stream that buffers string input and generates a data event for each line
type LineStream struct {
	*stream.Stream
	buffer string
}

// NewLineStream creates a new LineStream instance
func NewLineStream() *LineStream {
	return &LineStream{
		Stream: stream.NewStream(),
		buffer: "",
	}
}

// Push adds new data to be parsed
func (ls *LineStream) Push(data string) {
	ls.buffer += data
	nextNewline := strings.Index(ls.buffer, "\n")

	for nextNewline > -1 {
		line := ls.buffer[:nextNewline]
		ls.Trigger("data", map[string]interface{}{
			"data": line,
		})
		ls.buffer = ls.buffer[nextNewline+1:]
		nextNewline = strings.Index(ls.buffer, "\n")
	}
}
