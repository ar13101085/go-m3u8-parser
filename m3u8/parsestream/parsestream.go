// Package parsestream provides M3U8 tag parsing functionality
package parsestream

import (
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ar13101085/go-m3u8-parser/m3u8/stream"
)

const TAB = string('\t')

// Resolution represents a resolution object
type Resolution struct {
	Width  int
	Height int
}

// Byterange represents a byte range
type Byterange struct {
	Length int
	Offset int
}

// CustomParser represents a custom parser
type CustomParser struct {
	Expression *regexp.Regexp
	CustomType string
	DataParser func(string) string
	Segment    bool
}

// TagMapper represents a tag mapper
type TagMapper struct {
	Expression *regexp.Regexp
	Map        func(string) string
}

// ParseStream is a line-level M3U8 parser event stream
type ParseStream struct {
	*stream.Stream
	CustomParsers []CustomParser
	TagMappers    []TagMapper
}

// NewParseStream creates a new ParseStream instance
func NewParseStream() *ParseStream {
	return &ParseStream{
		Stream:        stream.NewStream(),
		CustomParsers: []CustomParser{},
		TagMappers:    []TagMapper{},
	}
}

// Push parses an additional line of input
func (ps *ParseStream) Push(line string) {
	var match []string
	var event map[string]interface{}

	// strip whitespace
	line = strings.TrimSpace(line)

	if len(line) == 0 {
		// ignore empty lines
		return
	}

	// URIs
	if line[0] != '#' {
		ps.Trigger("data", map[string]interface{}{
			"type": "uri",
			"uri":  line,
		})
		return
	}

	// map tags
	newLines := []string{line}
	for _, mapper := range ps.TagMappers {
		var mappedLines []string
		for _, l := range newLines {
			mappedLine := mapper.Map(l)
			// skip if unchanged
			if mappedLine == l {
				mappedLines = append(mappedLines, l)
			} else {
				mappedLines = append(mappedLines, mappedLine)
			}
		}
		newLines = mappedLines
	}

	for _, newLine := range newLines {
		// Try custom parsers first
		customParsed := false
		for _, parser := range ps.CustomParsers {
			if parser.Expression.MatchString(newLine) {
				var data string
				if parser.DataParser != nil {
					data = parser.DataParser(newLine)
				} else {
					data = newLine
				}

				ps.Trigger("data", map[string]interface{}{
					"type":       "custom",
					"data":       data,
					"customType": parser.CustomType,
					"segment":    parser.Segment,
				})
				customParsed = true
				break
			}
		}

		if customParsed {
			continue
		}

		// Comments
		if !strings.HasPrefix(newLine, "#EXT") {
			ps.Trigger("data", map[string]interface{}{
				"type": "comment",
				"text": newLine[1:],
			})
			continue
		}

		// strip off any carriage returns
		newLine = strings.Replace(newLine, "\r", "", -1)

		// Tags
		re := regexp.MustCompile(`^#EXTM3U`)
		if re.MatchString(newLine) {
			ps.Trigger("data", map[string]interface{}{
				"type":    "tag",
				"tagType": "m3u",
			})
			continue
		}

		re = regexp.MustCompile(`^#EXTINF:([0-9\.]*)?(?:,(.*))?$`)
		match = re.FindStringSubmatch(newLine)
		if len(match) > 0 {
			event = map[string]interface{}{
				"type":    "tag",
				"tagType": "inf",
			}
			if match[1] != "" {
				duration, _ := strconv.ParseFloat(match[1], 64)
				event["duration"] = duration
			}
			if len(match) > 2 && match[2] != "" {
				event["title"] = match[2]
			}
			ps.Trigger("data", event)
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-TARGETDURATION:([0-9\.]*)?`)
		match = re.FindStringSubmatch(newLine)
		if len(match) > 0 {
			event = map[string]interface{}{
				"type":    "tag",
				"tagType": "targetduration",
			}
			if match[1] != "" {
				duration, _ := strconv.Atoi(match[1])
				event["duration"] = duration
			}
			ps.Trigger("data", event)
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-VERSION:([0-9\.]*)?`)
		match = re.FindStringSubmatch(newLine)
		if len(match) > 0 {
			event = map[string]interface{}{
				"type":    "tag",
				"tagType": "version",
			}
			if match[1] != "" {
				version, _ := strconv.Atoi(match[1])
				event["version"] = version
			}
			ps.Trigger("data", event)
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-MEDIA-SEQUENCE:(\-?[0-9\.]*)?`)
		match = re.FindStringSubmatch(newLine)
		if len(match) > 0 {
			event = map[string]interface{}{
				"type":    "tag",
				"tagType": "media-sequence",
			}
			if match[1] != "" {
				number, _ := strconv.Atoi(match[1])
				event["number"] = number
			}
			ps.Trigger("data", event)
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-DISCONTINUITY-SEQUENCE:(\-?[0-9\.]*)?`)
		match = re.FindStringSubmatch(newLine)
		if len(match) > 0 {
			event = map[string]interface{}{
				"type":    "tag",
				"tagType": "discontinuity-sequence",
			}
			if match[1] != "" {
				number, _ := strconv.Atoi(match[1])
				event["number"] = number
			}
			ps.Trigger("data", event)
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-PLAYLIST-TYPE:(.*)?$`)
		match = re.FindStringSubmatch(newLine)
		if len(match) > 0 {
			event = map[string]interface{}{
				"type":    "tag",
				"tagType": "playlist-type",
			}
			if match[1] != "" {
				event["playlistType"] = match[1]
			}
			ps.Trigger("data", event)
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-BYTERANGE:(.*)?$`)
		match = re.FindStringSubmatch(newLine)
		if len(match) > 0 {
			event = map[string]interface{}{
				"type":    "tag",
				"tagType": "byterange",
			}
			if match[1] != "" {
				byterange := parseByterange(match[1])
				if byterange.Length != 0 {
					event["length"] = byterange.Length
				}
				if byterange.Offset != 0 {
					event["offset"] = byterange.Offset
				}
			}
			ps.Trigger("data", event)
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-ALLOW-CACHE:(YES|NO)?`)
		match = re.FindStringSubmatch(newLine)
		if len(match) > 0 {
			event = map[string]interface{}{
				"type":    "tag",
				"tagType": "allow-cache",
			}
			if match[1] != "" {
				event["allowed"] = !regexp.MustCompile(`NO`).MatchString(match[1])
			}
			ps.Trigger("data", event)
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-MAP:(.*)$`)
		match = re.FindStringSubmatch(newLine)
		if len(match) > 0 {
			event = map[string]interface{}{
				"type":    "tag",
				"tagType": "map",
			}
			if match[1] != "" {
				attributes := parseAttributes(match[1])
				if uri, ok := attributes["URI"]; ok {
					event["uri"] = uri
				}
				if byterange, ok := attributes["BYTERANGE"]; ok {
					event["byterange"] = parseByterange(byterange)
				}
			}
			ps.Trigger("data", event)
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-STREAM-INF:(.*)$`)
		match = re.FindStringSubmatch(newLine)
		if len(match) > 0 {
			event = map[string]interface{}{
				"type":    "tag",
				"tagType": "stream-inf",
			}
			if match[1] != "" {
				attributes := parseAttributes(match[1])
				event["attributes"] = attributes

				if resolution, ok := attributes["RESOLUTION"]; ok {
					event["attributes"].(map[string]string)["RESOLUTION"] = resolution
					resolutionObj := parseResolution(resolution)
					if resolutionObj.Width != 0 {
						attributes["RESOLUTION_WIDTH"] = strconv.Itoa(resolutionObj.Width)
					}
					if resolutionObj.Height != 0 {
						attributes["RESOLUTION_HEIGHT"] = strconv.Itoa(resolutionObj.Height)
					}
				}
				if bandwidth, ok := attributes["BANDWIDTH"]; ok {
					bandwidthInt, _ := strconv.Atoi(bandwidth)
					attributes["BANDWIDTH"] = strconv.Itoa(bandwidthInt)
				}
				if framerate, ok := attributes["FRAME-RATE"]; ok {
					framerateFloat, _ := strconv.ParseFloat(framerate, 64)
					attributes["FRAME-RATE"] = strconv.FormatFloat(framerateFloat, 'f', -1, 64)
				}
				if programId, ok := attributes["PROGRAM-ID"]; ok {
					programIdInt, _ := strconv.Atoi(programId)
					attributes["PROGRAM-ID"] = strconv.Itoa(programIdInt)
				}
			}
			ps.Trigger("data", event)
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-MEDIA:(.*)$`)
		match = re.FindStringSubmatch(newLine)
		if len(match) > 0 {
			event = map[string]interface{}{
				"type":    "tag",
				"tagType": "media",
			}
			if match[1] != "" {
				attributes := parseAttributes(match[1])
				event["attributes"] = attributes
			}
			ps.Trigger("data", event)
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-ENDLIST`)
		if re.MatchString(newLine) {
			ps.Trigger("data", map[string]interface{}{
				"type":    "tag",
				"tagType": "endlist",
			})
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-DISCONTINUITY`)
		if re.MatchString(newLine) {
			ps.Trigger("data", map[string]interface{}{
				"type":    "tag",
				"tagType": "discontinuity",
			})
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-PROGRAM-DATE-TIME:(.*)$`)
		match = re.FindStringSubmatch(newLine)
		if len(match) > 0 {
			event = map[string]interface{}{
				"type":    "tag",
				"tagType": "program-date-time",
			}
			if match[1] != "" {
				event["dateTimeString"] = match[1]
				dateTimeObject, _ := time.Parse(time.RFC3339, match[1])
				event["dateTimeObject"] = dateTimeObject
			}
			ps.Trigger("data", event)
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-KEY:(.*)$`)
		match = re.FindStringSubmatch(newLine)
		if len(match) > 0 {
			event = map[string]interface{}{
				"type":    "tag",
				"tagType": "key",
			}
			if match[1] != "" {
				attributes := parseAttributes(match[1])
				event["attributes"] = attributes

				// parse the IV string into a []byte
				if iv, ok := attributes["IV"]; ok {
					if strings.HasPrefix(strings.ToLower(iv), "0x") {
						iv = iv[2:]
					}

					ivBytes, _ := hex.DecodeString(iv)
					attributes["IV"] = hex.EncodeToString(ivBytes)
				}
			}
			ps.Trigger("data", event)
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-START:(.*)$`)
		match = re.FindStringSubmatch(newLine)
		if len(match) > 0 {
			event = map[string]interface{}{
				"type":    "tag",
				"tagType": "start",
			}
			if match[1] != "" {
				attributes := parseAttributes(match[1])
				event["attributes"] = attributes

				if timeOffset, ok := attributes["TIME-OFFSET"]; ok {
					timeOffsetFloat, _ := strconv.ParseFloat(timeOffset, 64)
					attributes["TIME-OFFSET"] = strconv.FormatFloat(timeOffsetFloat, 'f', -1, 64)
				}
				if precise, ok := attributes["PRECISE"]; ok {
					attributes["PRECISE"] = strconv.FormatBool(regexp.MustCompile(`YES`).MatchString(precise))
				}
			}
			ps.Trigger("data", event)
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-CUE-OUT-CONT:(.*)?$`)
		match = re.FindStringSubmatch(newLine)
		if len(match) > 0 {
			event = map[string]interface{}{
				"type":    "tag",
				"tagType": "cue-out-cont",
			}
			if len(match) > 1 {
				event["data"] = match[1]
			} else {
				event["data"] = ""
			}
			ps.Trigger("data", event)
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-CUE-OUT:(.*)?$`)
		match = re.FindStringSubmatch(newLine)
		if len(match) > 0 {
			event = map[string]interface{}{
				"type":    "tag",
				"tagType": "cue-out",
			}
			if len(match) > 1 {
				event["data"] = match[1]
			} else {
				event["data"] = ""
			}
			ps.Trigger("data", event)
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-CUE-IN:?(.*)?$`)
		match = re.FindStringSubmatch(newLine)
		if len(match) > 0 {
			event = map[string]interface{}{
				"type":    "tag",
				"tagType": "cue-in",
			}
			if len(match) > 1 {
				event["data"] = match[1]
			} else {
				event["data"] = ""
			}
			ps.Trigger("data", event)
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-SKIP:(.*)$`)
		match = re.FindStringSubmatch(newLine)
		if len(match) > 0 && match[1] != "" {
			event = map[string]interface{}{
				"type":    "tag",
				"tagType": "skip",
			}
			attributes := parseAttributes(match[1])
			event["attributes"] = attributes

			if skippedSegments, ok := attributes["SKIPPED-SEGMENTS"]; ok {
				skippedSegmentsInt, _ := strconv.Atoi(skippedSegments)
				attributes["SKIPPED-SEGMENTS"] = strconv.Itoa(skippedSegmentsInt)
			}

			if removedDateranges, ok := attributes["RECENTLY-REMOVED-DATERANGES"]; ok {
				attributes["RECENTLY-REMOVED-DATERANGES"] = removedDateranges
				// We actually need to process this into an array on the parser side when needed
			}

			ps.Trigger("data", event)
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-PART:(.*)$`)
		match = re.FindStringSubmatch(newLine)
		if len(match) > 0 && match[1] != "" {
			event = map[string]interface{}{
				"type":    "tag",
				"tagType": "part",
			}
			attributes := parseAttributes(match[1])
			event["attributes"] = attributes

			if duration, ok := attributes["DURATION"]; ok {
				durationFloat, _ := strconv.ParseFloat(duration, 64)
				attributes["DURATION"] = strconv.FormatFloat(durationFloat, 'f', -1, 64)
			}

			for _, key := range []string{"INDEPENDENT", "GAP"} {
				if val, ok := attributes[key]; ok {
					attributes[key] = strconv.FormatBool(regexp.MustCompile(`YES`).MatchString(val))
				}
			}

			if byterange, ok := attributes["BYTERANGE"]; ok {
				event["byterange"] = parseByterange(byterange)
			}

			ps.Trigger("data", event)
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-SERVER-CONTROL:(.*)$`)
		match = re.FindStringSubmatch(newLine)
		if len(match) > 0 && match[1] != "" {
			event = map[string]interface{}{
				"type":    "tag",
				"tagType": "server-control",
			}
			attributes := parseAttributes(match[1])
			event["attributes"] = attributes

			for _, key := range []string{"CAN-SKIP-UNTIL", "PART-HOLD-BACK", "HOLD-BACK"} {
				if val, ok := attributes[key]; ok {
					floatVal, _ := strconv.ParseFloat(val, 64)
					attributes[key] = strconv.FormatFloat(floatVal, 'f', -1, 64)
				}
			}

			for _, key := range []string{"CAN-SKIP-DATERANGES", "CAN-BLOCK-RELOAD"} {
				if val, ok := attributes[key]; ok {
					attributes[key] = strconv.FormatBool(regexp.MustCompile(`YES`).MatchString(val))
				}
			}

			ps.Trigger("data", event)
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-PART-INF:(.*)$`)
		match = re.FindStringSubmatch(newLine)
		if len(match) > 0 && match[1] != "" {
			event = map[string]interface{}{
				"type":    "tag",
				"tagType": "part-inf",
			}
			attributes := parseAttributes(match[1])
			event["attributes"] = attributes

			if val, ok := attributes["PART-TARGET"]; ok {
				floatVal, _ := strconv.ParseFloat(val, 64)
				attributes["PART-TARGET"] = strconv.FormatFloat(floatVal, 'f', -1, 64)
			}

			ps.Trigger("data", event)
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-INDEPENDENT-SEGMENTS`)
		if re.MatchString(newLine) {
			ps.Trigger("data", map[string]interface{}{
				"type":    "tag",
				"tagType": "independent-segments",
			})
			continue
		}

		re = regexp.MustCompile(`^#EXT-X-I-FRAMES-ONLY`)
		if re.MatchString(newLine) {
			ps.Trigger("data", map[string]interface{}{
				"type":    "tag",
				"tagType": "i-frames-only",
			})
			continue
		}

		// Find and check the I-FRAME-STREAM-INF handler in the Push method
		re = regexp.MustCompile(`^#EXT-X-I-FRAME-STREAM-INF:(.*)$`)
		match = re.FindStringSubmatch(newLine)
		if len(match) > 0 {
			event = map[string]interface{}{
				"type":    "tag",
				"tagType": "i-frame-playlist",
			}

			if match[1] != "" {
				attributes := parseAttributes(match[1])
				event["attributes"] = attributes

				if uri, ok := attributes["URI"]; ok {
					event["uri"] = uri
				}

				if resolution, ok := attributes["RESOLUTION"]; ok {
					resolutionObj := parseResolution(resolution)
					if resolutionObj.Width != 0 {
						attributes["RESOLUTION_WIDTH"] = strconv.Itoa(resolutionObj.Width)
					}
					if resolutionObj.Height != 0 {
						attributes["RESOLUTION_HEIGHT"] = strconv.Itoa(resolutionObj.Height)
					}
				}

				if bandwidth, ok := attributes["BANDWIDTH"]; ok {
					bandwidthInt, _ := strconv.Atoi(bandwidth)
					attributes["BANDWIDTH"] = strconv.Itoa(bandwidthInt)
				}

				if framerate, ok := attributes["FRAME-RATE"]; ok {
					framerateFloat, _ := strconv.ParseFloat(framerate, 64)
					attributes["FRAME-RATE"] = strconv.FormatFloat(framerateFloat, 'f', -1, 64)
				}
			}

			ps.Trigger("data", event)
			continue
		}

		// unknown tag type
		ps.Trigger("data", map[string]interface{}{
			"type": "tag",
			"data": newLine[4:],
		})
	}
}

// HandleData handles data from a line stream
func (ps *ParseStream) HandleData(data interface{}) {
	if dataMap, ok := data.(map[string]interface{}); ok {
		if lineData, ok := dataMap["data"].(string); ok {
			ps.Push(lineData)
		}
	}
}

// AddParser adds a parser for custom headers
func (ps *ParseStream) AddParser(options CustomParser) {
	if options.DataParser == nil {
		options.DataParser = func(line string) string { return line }
	}
	ps.CustomParsers = append(ps.CustomParsers, options)
}

// AddTagMapper adds a custom header mapper
func (ps *ParseStream) AddTagMapper(options TagMapper) {
	ps.TagMappers = append(ps.TagMappers, options)
}

// Helper functions

// parseByterange parses a byterange string
func parseByterange(byterangeString string) Byterange {
	re := regexp.MustCompile(`([0-9.]*)?@?([0-9.]*)?`)
	match := re.FindStringSubmatch(byterangeString)
	result := Byterange{}

	if len(match) > 1 && match[1] != "" {
		length, _ := strconv.Atoi(match[1])
		result.Length = length
	}

	if len(match) > 2 && match[2] != "" {
		offset, _ := strconv.Atoi(match[2])
		result.Offset = offset
	}

	return result
}

// attributeSeparator returns a regexp for splitting attributes
func attributeSeparator() *regexp.Regexp {
	key := `[^=]*`
	value := `"[^"]*"|[^,]*`
	keyvalue := `(?:` + key + `)=(?:` + value + `)`

	return regexp.MustCompile(`(?:^|,)(` + keyvalue + `)`)
}

// parseAttributes parses attributes from a line
func parseAttributes(attributesString string) map[string]string {
	result := make(map[string]string)

	if attributesString == "" {
		return result
	}

	separator := attributeSeparator()
	attrs := separator.FindAllString(attributesString, -1)

	for _, attr := range attrs {
		if attr == "" {
			continue
		}

		// Remove leading comma if present
		if attr[0] == ',' {
			attr = attr[1:]
		}

		re := regexp.MustCompile(`([^=]*)=(.*)`)
		match := re.FindStringSubmatch(attr)

		if len(match) >= 3 {
			key := strings.TrimSpace(match[1])
			value := strings.TrimSpace(match[2])

			// Remove quotes if present
			re = regexp.MustCompile(`^['"](.*)['"]$`)
			if quotedMatch := re.FindStringSubmatch(value); len(quotedMatch) > 1 {
				value = quotedMatch[1]
			}

			result[key] = value
		}
	}

	return result
}

// parseResolution converts a string into a resolution object
func parseResolution(resolutionStr string) Resolution {
	result := Resolution{}
	parts := strings.Split(resolutionStr, "x")

	if len(parts) > 0 && parts[0] != "" {
		width, _ := strconv.Atoi(parts[0])
		result.Width = width
	}

	if len(parts) > 1 && parts[1] != "" {
		height, _ := strconv.Atoi(parts[1])
		result.Height = height
	}

	return result
}
