package parser

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ar13101085/go-m3u8-parser/m3u8/linestream"
	"github.com/ar13101085/go-m3u8-parser/m3u8/parsestream"
	"github.com/ar13101085/go-m3u8-parser/m3u8/stream"
)

// Segment represents an HLS segment
type Segment struct {
	URI             string
	Duration        float64
	Title           string
	Byterange       *parsestream.Byterange
	Map             *Map
	Key             *Key
	Timeline        int
	Discontinuity   bool
	DateTimeString  string
	DateTimeObject  time.Time
	ProgramDateTime int64
	CueOut          string
	CueOutCont      string
	CueIn           string
	Parts           []map[string]interface{}
	PreloadHints    []map[string]interface{}
	Attributes      map[string]string
}

// Map represents initialization segment information
type Map struct {
	URI       string
	Byterange *parsestream.Byterange
	Key       *Key
}

// Key represents encryption information
type Key struct {
	Method string
	URI    string
	IV     string
}

// Start represents a start point for the manifest
type Start struct {
	TimeOffset float64
	Precise    bool
}

// DateRange represents a date range
type DateRange struct {
	ID               string
	Class            string
	StartDate        time.Time
	EndDate          time.Time
	Duration         float64
	PlannedDuration  float64
	EndOnNext        bool
	SCTE35CMD        string
	SCTE35OUT        string
	SCTE35IN         string
	CustomAttributes map[string]interface{}
}

// IFramePlaylist represents an iFrame playlist
type IFramePlaylist struct {
	URI        string
	Attributes map[string]string
	Timeline   int
}

// MediaGroup represents a media group
type MediaGroup struct {
	Default         bool
	Autoselect      bool
	Language        string
	URI             string
	InstreamID      string
	Characteristics string
	Forced          bool
}

// Manifest represents a parsed M3U8 manifest
type Manifest struct {
	AllowCache            bool
	Version               int
	TargetDuration        int
	MediaSequence         int
	DiscontinuitySequence int
	EndList               bool
	PlaylistType          string
	DateTimeString        string
	DateTimeObject        time.Time
	Segments              []*Segment
	PreloadSegment        *Segment

	IFramesOnly         bool
	IndependentSegments bool
	Start               *Start
	DtsCptOffset        float64

	Skip                map[string]interface{}
	ServerControl       map[string]interface{}
	PartInf             map[string]interface{}
	PartTargetDuration  float64
	RenditionReports    []map[string]interface{}
	Playlists           []*Segment
	IFramePlaylists     []*IFramePlaylist
	DiscontinuityStarts []int
	DateRanges          []*DateRange
	ContentProtection   map[string]interface{}
	ContentSteering     map[string]interface{}
	MediaGroups         map[string]map[string]map[string]*MediaGroup
	Custom              map[string]interface{}
	Definitions         map[string]string
}

// Parser represents an M3U8 parser
type Parser struct {
	*stream.Stream
	LineStream          *linestream.LineStream
	ParseStream         *parsestream.ParseStream
	Manifest            *Manifest
	URI                 string
	MainDefinitions     map[string]string
	Params              url.Values
	LastProgramDateTime int64
}

// NewParser creates a new Parser instance
func NewParser(opts map[string]interface{}) *Parser {
	p := &Parser{
		Stream:              stream.NewStream(),
		LineStream:          linestream.NewLineStream(),
		ParseStream:         parsestream.NewParseStream(),
		MainDefinitions:     make(map[string]string),
		LastProgramDateTime: 0,
	}

	if uri, ok := opts["uri"].(string); ok {
		p.URI = uri
		parsedURL, err := url.Parse(uri)
		if err == nil {
			p.Params = parsedURL.Query()
		}
	}

	if mainDefs, ok := opts["mainDefinitions"].(map[string]string); ok {
		p.MainDefinitions = mainDefs
	}

	p.Manifest = &Manifest{
		AllowCache:          true,
		DiscontinuityStarts: []int{},
		DateRanges:          []*DateRange{},
		IFramePlaylists:     []*IFramePlaylist{},
		Segments:            []*Segment{},
		MediaGroups:         make(map[string]map[string]map[string]*MediaGroup),
	}

	// Initialize default media groups
	defaultMediaGroups := []string{"AUDIO", "VIDEO", "CLOSED-CAPTIONS", "SUBTITLES"}
	for _, groupType := range defaultMediaGroups {
		p.Manifest.MediaGroups[groupType] = make(map[string]map[string]*MediaGroup)
	}

	// State variables for parsing
	uris := []*Segment{}
	var currentUri *Segment = &Segment{}
	var currentMap *Map
	var key *Key
	currentTimeline := 0
	lastByterangeEnd := 0
	lastPartByterangeEnd := 0

	// Variables for tracking variant playlists
	expectPlaylistURI := false
	var pendingPlaylist *Segment

	p.On("end", func(data interface{}) {
		// only add preloadSegment if we don't yet have a uri for it
		// and we actually have parts/preloadHints
		if currentUri.URI != "" || (len(currentUri.Parts) == 0 && len(currentUri.PreloadHints) == 0) {
			return
		}

		if currentUri.Map == nil && currentMap != nil {
			currentUri.Map = currentMap
		}

		if currentUri.Key == nil && key != nil {
			currentUri.Key = key
		}

		if currentUri.Timeline == 0 && currentTimeline != 0 {
			currentUri.Timeline = currentTimeline
		}

		p.Manifest.PreloadSegment = currentUri
	})

	p.ParseStream.Stream.On("data", func(data interface{}) {
		entry, ok := data.(map[string]interface{})
		if !ok {
			// Handle error case when data is not a map[string]interface{}
			p.Trigger("error", map[string]interface{}{
				"message": "Invalid data format received",
			})
			return
		}

		entryType, ok := entry["type"].(string)
		if !ok {
			p.Trigger("error", map[string]interface{}{
				"message": "Missing type in data",
			})
			return
		}

		// Replace variables in uris and attributes as defined in #EXT-X-DEFINE tags
		if p.Manifest.Definitions != nil {
			if entryType == "uri" && entry["uri"] != nil {
				uri := entry["uri"].(string)
				for def, val := range p.Manifest.Definitions {
					uri = strings.Replace(uri, "{$"+def+"}", val, -1)
				}
				entry["uri"] = uri
			}
			if attrs, ok := entry["attributes"].(map[string]string); ok {
				for attrKey, attrVal := range attrs {
					for def, val := range p.Manifest.Definitions {
						attrs[attrKey] = strings.Replace(attrVal, "{$"+def+"}", val, -1)
					}
				}
			}
		}

		switch entryType {
		case "tag":
			tagType, _ := entry["tagType"].(string)
			switch tagType {
			case "m3u":
				// Nothing to do

			case "version":
				if version, ok := entry["version"].(int); ok {
					p.Manifest.Version = version
				}

			case "allow-cache":
				if allowed, ok := entry["allowed"].(bool); ok {
					p.Manifest.AllowCache = allowed
				} else {
					p.Manifest.AllowCache = true
					p.Trigger("info", map[string]interface{}{
						"message": "defaulting allowCache to YES",
					})
				}

			case "byterange":
				byterange := &parsestream.Byterange{}
				if length, ok := entry["length"].(int); ok {
					byterange.Length = length
					currentUri.Byterange = byterange

					if _, ok := entry["offset"]; !ok {
						entry["offset"] = lastByterangeEnd
					}
				}
				if offset, ok := entry["offset"].(int); ok {
					byterange.Offset = offset
					currentUri.Byterange = byterange
				}
				lastByterangeEnd = byterange.Offset + byterange.Length

			case "endlist":
				p.Manifest.EndList = true

			case "inf":
				if p.Manifest.MediaSequence == 0 {
					p.Manifest.MediaSequence = 0
					p.Trigger("info", map[string]interface{}{
						"message": "defaulting media sequence to zero",
					})
				}
				if p.Manifest.DiscontinuitySequence == 0 {
					p.Manifest.DiscontinuitySequence = 0
					p.Trigger("info", map[string]interface{}{
						"message": "defaulting discontinuity sequence to zero",
					})
				}

				if title, ok := entry["title"].(string); ok {
					currentUri.Title = title
				}

				if duration, ok := entry["duration"].(float64); ok && duration > 0 {
					currentUri.Duration = duration
				}

				if duration, ok := entry["duration"].(float64); ok && duration == 0 {
					currentUri.Duration = 0.01
					p.Trigger("info", map[string]interface{}{
						"message": "updating zero segment duration to a small value",
					})
				}

				p.Manifest.Segments = uris

			case "key":
				attrs, ok := entry["attributes"].(map[string]string)
				if !ok {
					p.Trigger("warn", map[string]interface{}{
						"message": "ignoring key declaration without attribute list",
					})
					return
				}

				// clear the active encryption key
				if method, ok := attrs["METHOD"]; ok && method == "NONE" {
					key = nil
					return
				}

				if _, ok := attrs["URI"]; !ok {
					p.Trigger("warn", map[string]interface{}{
						"message": "ignoring key declaration without URI",
					})
					return
				}

				// setup an encryption key for upcoming segments
				key = &Key{
					Method: "AES-128",
					URI:    attrs["URI"],
				}

				if method, ok := attrs["METHOD"]; ok {
					key.Method = method
				}

				if iv, ok := attrs["IV"]; ok {
					key.IV = iv
				}

			case "media-sequence":
				if number, ok := entry["number"].(int); ok {
					p.Manifest.MediaSequence = number
				}

			case "discontinuity-sequence":
				if number, ok := entry["number"].(int); ok {
					p.Manifest.DiscontinuitySequence = number
					currentTimeline = number
				}

			case "playlist-type":
				if playlistType, ok := entry["playlistType"].(string); ok {
					p.Manifest.PlaylistType = playlistType
				}

			case "map":
				currentMap = &Map{}
				if uri, ok := entry["uri"].(string); ok {
					currentMap.URI = uri
				}
				if byterange, ok := entry["byterange"].(parsestream.Byterange); ok {
					currentMap.Byterange = &byterange
				}
				if key != nil {
					currentMap.Key = key
				}

			case "stream-inf":
				attrsMap, ok := entry["attributes"].(map[string]string)
				if !ok {
					p.Trigger("warn", map[string]interface{}{
						"message": "ignoring empty stream-inf attributes",
					})
					return
				}

				// Create a new variant playlist entry with attributes
				pendingPlaylist = &Segment{
					Attributes: make(map[string]string),
				}

				// Copy all attributes to the playlist segment
				for k, v := range attrsMap {
					pendingPlaylist.Attributes[k] = v
				}

				// Initialize playlists array if needed
				if p.Manifest.Playlists == nil {
					p.Manifest.Playlists = []*Segment{}
				}

				// Signal that the next URI belongs to this variant
				expectPlaylistURI = true

			case "discontinuity":
				currentTimeline++
				currentUri.Discontinuity = true
				p.Manifest.DiscontinuityStarts = append(p.Manifest.DiscontinuityStarts, len(uris))

			case "program-date-time":
				if p.Manifest.DateTimeString == "" {
					if dateTimeStr, ok := entry["dateTimeString"].(string); ok {
						p.Manifest.DateTimeString = dateTimeStr
					}
					if dateTimeObj, ok := entry["dateTimeObject"].(time.Time); ok {
						p.Manifest.DateTimeObject = dateTimeObj
					}
				}

				if dateTimeStr, ok := entry["dateTimeString"].(string); ok {
					currentUri.DateTimeString = dateTimeStr
				}

				if dateTimeObj, ok := entry["dateTimeObject"].(time.Time); ok {
					currentUri.DateTimeObject = dateTimeObj
				}

				// Handle program date time logic similar to the JS version
				lastDateTime := p.LastProgramDateTime

				// Parse the current date time value
				if dateTimeStr, ok := entry["dateTimeString"].(string); ok {
					t, err := time.Parse(time.RFC3339, dateTimeStr)
					if err == nil {
						p.LastProgramDateTime = t.UnixNano() / int64(time.Millisecond)
					}
				}

				// Extrapolate backwards if this is the first PDT
				if lastDateTime == 0 {
					// We need to go through segments in reverse and set program date time
					prevTime := p.LastProgramDateTime
					for i := len(p.Manifest.Segments) - 1; i >= 0; i-- {
						segment := p.Manifest.Segments[i]
						// Calculate segment PDT based on duration
						segment.ProgramDateTime = prevTime - int64(segment.Duration*1000)
						prevTime = segment.ProgramDateTime
					}
				}

			case "targetduration":
				if duration, ok := entry["duration"].(int); ok {
					p.Manifest.TargetDuration = duration
				}

			case "start":
				attrs, ok := entry["attributes"].(map[string]string)
				if !ok {
					p.Trigger("warn", map[string]interface{}{
						"message": "ignoring start declaration without appropriate attribute list",
					})
					return
				}

				p.Manifest.Start = &Start{}

				if timeOffset, ok := attrs["TIME-OFFSET"]; ok {
					if offset, err := strconv.ParseFloat(timeOffset, 64); err == nil {
						p.Manifest.Start.TimeOffset = offset
					}
				}

				if precise, ok := attrs["PRECISE"]; ok {
					p.Manifest.Start.Precise = regexp.MustCompile(`YES`).MatchString(precise)
				}

			case "cue-out":
				if data, ok := entry["data"].(string); ok {
					currentUri.CueOut = data
				}

			case "cue-out-cont":
				if data, ok := entry["data"].(string); ok {
					currentUri.CueOutCont = data
				}

			case "cue-in":
				if data, ok := entry["data"].(string); ok {
					currentUri.CueIn = data
				}

			case "independent-segments":
				p.Manifest.IndependentSegments = true

			case "i-frames-only":
				p.Manifest.IFramesOnly = true

			case "part":
				attrs, ok := entry["attributes"].(map[string]string)
				if ok {
					part := make(map[string]interface{})
					for k, v := range attrs {
						part[camelCase(k)] = v
					}

					if byterangeObj, ok := entry["byterange"].(parsestream.Byterange); ok {
						byterange := parsestream.Byterange{
							Length: byterangeObj.Length,
							Offset: byterangeObj.Offset,
						}

						if byterange.Offset == 0 {
							byterange.Offset = lastPartByterangeEnd
						}

						lastPartByterangeEnd = byterange.Offset + byterange.Length
						part["byterange"] = byterange
					}

					currentUri.Parts = append(currentUri.Parts, part)
				}

			case "i-frame-playlist":
				attrs, ok := entry["attributes"].(map[string]string)
				if !ok {
					p.Trigger("warn", map[string]interface{}{
						"message": "ignoring empty i-frame-playlist attributes",
					})
					return
				}

				uri, ok := entry["uri"].(string)
				if !ok {
					p.Trigger("warn", map[string]interface{}{
						"message": "ignoring i-frame-playlist without URI",
					})
					return
				}

				p.Manifest.IFramePlaylists = append(p.Manifest.IFramePlaylists, &IFramePlaylist{
					URI:        uri,
					Attributes: attrs,
					Timeline:   currentTimeline,
				})

			case "media":
				attrs, ok := entry["attributes"].(map[string]string)
				if !ok {
					p.Trigger("warn", map[string]interface{}{
						"message": "ignoring media without attributes",
					})
					return
				}

				mediaType, ok := attrs["TYPE"]
				if !ok {
					p.Trigger("warn", map[string]interface{}{
						"message": "ignoring media without TYPE",
					})
					return
				}

				groupID, ok := attrs["GROUP-ID"]
				if !ok {
					p.Trigger("warn", map[string]interface{}{
						"message": "ignoring media without GROUP-ID",
					})
					return
				}

				name, ok := attrs["NAME"]
				if !ok {
					p.Trigger("warn", map[string]interface{}{
						"message": "ignoring media without NAME",
					})
					return
				}

				// Initialize the group if needed
				if p.Manifest.MediaGroups[mediaType][groupID] == nil {
					p.Manifest.MediaGroups[mediaType][groupID] = make(map[string]*MediaGroup)
				}

				// Create the media group rendition
				rendition := &MediaGroup{
					Default:    regexp.MustCompile(`yes`).MatchString(attrs["DEFAULT"]),
					Autoselect: regexp.MustCompile(`yes`).MatchString(attrs["AUTOSELECT"]),
				}

				if language, ok := attrs["LANGUAGE"]; ok {
					rendition.Language = language
				}

				if uri, ok := attrs["URI"]; ok {
					rendition.URI = uri
				}

				if instreamID, ok := attrs["INSTREAM-ID"]; ok {
					rendition.InstreamID = instreamID
				}

				if characteristics, ok := attrs["CHARACTERISTICS"]; ok {
					rendition.Characteristics = characteristics
				}

				if forced, ok := attrs["FORCED"]; ok {
					rendition.Forced = regexp.MustCompile(`yes`).MatchString(forced)
				}

				// Add the rendition to the media groups
				p.Manifest.MediaGroups[mediaType][groupID][name] = rendition

			case "daterange":
				attrs, ok := entry["attributes"].(map[string]string)
				if !ok {
					p.Trigger("warn", map[string]interface{}{
						"message": "ignoring daterange without attributes",
					})
					return
				}

				id, ok := attrs["ID"]
				if !ok {
					p.Trigger("warn", map[string]interface{}{
						"message": "ignoring daterange without ID",
					})
					return
				}

				startDate, ok := attrs["START-DATE"]
				if !ok {
					p.Trigger("warn", map[string]interface{}{
						"message": "ignoring daterange without START-DATE",
					})
					return
				}

				dateRange := &DateRange{
					ID: id,
				}

				if class, ok := attrs["CLASS"]; ok {
					dateRange.Class = class
				}

				if startDateObj, err := time.Parse(time.RFC3339, startDate); err == nil {
					dateRange.StartDate = startDateObj
				}

				if endDate, ok := attrs["END-DATE"]; ok {
					if endDateObj, err := time.Parse(time.RFC3339, endDate); err == nil {
						dateRange.EndDate = endDateObj
					}
				}

				if durationStr, ok := attrs["DURATION"]; ok {
					if duration, err := strconv.ParseFloat(durationStr, 64); err == nil {
						dateRange.Duration = duration
					}
				}

				if plannedDurationStr, ok := attrs["PLANNED-DURATION"]; ok {
					if plannedDuration, err := strconv.ParseFloat(plannedDurationStr, 64); err == nil {
						dateRange.PlannedDuration = plannedDuration
					}
				}

				if endOnNext, ok := attrs["END-ON-NEXT"]; ok {
					dateRange.EndOnNext = regexp.MustCompile(`yes`).MatchString(endOnNext)
				}

				if scte35CMD, ok := attrs["SCTE35-CMD"]; ok {
					dateRange.SCTE35CMD = scte35CMD
				}

				if scte35OUT, ok := attrs["SCTE35-OUT"]; ok {
					dateRange.SCTE35OUT = scte35OUT
				}

				if scte35IN, ok := attrs["SCTE35-IN"]; ok {
					dateRange.SCTE35IN = scte35IN
				}

				dateRange.CustomAttributes = make(map[string]interface{})
				// Process client-defined attributes (X-*)
				for key, value := range attrs {
					if strings.HasPrefix(key, "X-") {
						// Try to determine the type
						if regexp.MustCompile(`^0x[0-9A-Fa-f]+$`).MatchString(value) {
							// Hex value
							dateRange.CustomAttributes[key] = value
						} else if f, err := strconv.ParseFloat(value, 64); err == nil {
							// Numeric value
							dateRange.CustomAttributes[key] = f
						} else {
							// String value
							dateRange.CustomAttributes[key] = value
						}
					}
				}

				p.Manifest.DateRanges = append(p.Manifest.DateRanges, dateRange)

			case "part-inf":
				attrs, ok := entry["attributes"].(map[string]string)
				if !ok {
					p.Trigger("warn", map[string]interface{}{
						"message": "ignoring part-inf without attributes",
					})
					return
				}

				p.Manifest.PartInf = camelCaseKeys(attrs)
				if partTarget, ok := p.Manifest.PartInf["partTarget"].(float64); ok {
					p.Manifest.PartTargetDuration = partTarget
				}

			case "server-control":
				attrs, ok := entry["attributes"].(map[string]string)
				if !ok {
					p.Trigger("warn", map[string]interface{}{
						"message": "ignoring server-control without attributes",
					})
					return
				}

				p.Manifest.ServerControl = camelCaseKeys(attrs)

				// Add can-block-reload default if not specified
				if _, ok := p.Manifest.ServerControl["canBlockReload"]; !ok {
					p.Manifest.ServerControl["canBlockReload"] = false
					p.Trigger("info", map[string]interface{}{
						"message": "#EXT-X-SERVER-CONTROL defaulting CAN-BLOCK-RELOAD to false",
					})
				}

				// Validate CAN-SKIP-DATERANGES requires CAN-SKIP-UNTIL
				if canSkipDateranges, ok := p.Manifest.ServerControl["canSkipDateranges"].(bool); ok && canSkipDateranges {
					if _, ok := p.Manifest.ServerControl["canSkipUntil"]; !ok {
						p.Trigger("warn", map[string]interface{}{
							"message": "#EXT-X-SERVER-CONTROL lacks required attribute CAN-SKIP-UNTIL which is required when CAN-SKIP-DATERANGES is set",
						})
					}
				}
			}

		case "uri":
			uri, _ := entry["uri"].(string)

			if expectPlaylistURI {
				// This URI is for a variant playlist
				pendingPlaylist.URI = uri
				p.Manifest.Playlists = append(p.Manifest.Playlists, pendingPlaylist)
				expectPlaylistURI = false
			} else {
				// This is a normal segment URI
				currentUri.URI = uri
				uris = append(uris, currentUri)

				// if no explicit duration was declared, use the target duration
				if p.Manifest.TargetDuration != 0 && currentUri.Duration == 0 {
					p.Trigger("warn", map[string]interface{}{
						"message": "defaulting segment duration to the target duration",
					})
					currentUri.Duration = float64(p.Manifest.TargetDuration)
				}

				// annotate with encryption information, if necessary
				if key != nil {
					currentUri.Key = key
				}

				currentUri.Timeline = currentTimeline

				// annotate with initialization segment information, if necessary
				if currentMap != nil {
					currentUri.Map = currentMap
				}

				// reset the last byterange end as it needs to be 0 between parts
				lastPartByterangeEnd = 0

				// Once we have at least one program date time we can always extrapolate it forward
				if p.LastProgramDateTime != 0 {
					currentUri.ProgramDateTime = p.LastProgramDateTime
					p.LastProgramDateTime += int64(currentUri.Duration * 1000)
				}
			}

			// prepare for the next URI
			currentUri = &Segment{}

		case "comment":
			// comments are not important for playback

		case "custom":
			customType, _ := entry["customType"].(string)
			data, _ := entry["data"].(string)
			segment, _ := entry["segment"].(bool)

			// if this is segment-level data attach the output to the segment
			if segment {
				// Custom segment data handling would go here
			} else {
				// if this is manifest-level data attach to the top level manifest object
				if p.Manifest.Custom == nil {
					p.Manifest.Custom = make(map[string]interface{})
				}
				p.Manifest.Custom[customType] = data
			}
		}
	})

	// Connect LineStream to ParseStream
	p.LineStream.Stream.On("data", func(data interface{}) {
		p.ParseStream.HandleData(data)
	})

	p.LineStream.Stream.On("end", func(data interface{}) {
		p.ParseStream.Stream.Trigger("end", nil)
	})

	return p
}

// Push parses the input string and updates the manifest object
func (p *Parser) Push(chunk string) {
	p.LineStream.Push(chunk)
}

// End flushes any remaining input
func (p *Parser) End() {
	// flush any buffered input
	p.LineStream.Push("\n")
	p.LastProgramDateTime = 0
	p.Trigger("end", nil)
}

// AddParser adds an additional parser for non-standard tags
func (p *Parser) AddParser(options map[string]interface{}) {
	// Convert options to the parsestream.CustomParser format
	customParser := parsestream.CustomParser{}

	if expr, ok := options["expression"].(*regexp.Regexp); ok {
		customParser.Expression = expr
	}

	if customType, ok := options["customType"].(string); ok {
		customParser.CustomType = customType
	}

	if dataParser, ok := options["dataParser"].(func(string) string); ok {
		customParser.DataParser = dataParser
	}

	if segment, ok := options["segment"].(bool); ok {
		customParser.Segment = segment
	}

	p.ParseStream.AddParser(customParser)
}

// AddTagMapper adds a custom header mapper
func (p *Parser) AddTagMapper(options map[string]interface{}) {
	// Convert options to the parsestream.TagMapper format
	tagMapper := parsestream.TagMapper{}

	if expr, ok := options["expression"].(*regexp.Regexp); ok {
		tagMapper.Expression = expr
	}

	if mapFunc, ok := options["map"].(func(string) string); ok {
		tagMapper.Map = mapFunc
	}

	p.ParseStream.AddTagMapper(tagMapper)
}

// Helper function to convert strings to camelCase
func camelCase(str string) string {
	str = strings.ToLower(str)
	re := regexp.MustCompile(`-(\w)`)
	return re.ReplaceAllStringFunc(str, func(s string) string {
		return strings.ToUpper(string(s[1]))
	})
}

// Helper function to convert keys in a map to camelCase
func camelCaseKeys(attributes map[string]string) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range attributes {
		// Convert string values to appropriate types
		switch key {
		case "DURATION", "TIME-OFFSET", "PART-TARGET":
			if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
				result[camelCase(key)] = floatVal
			} else {
				result[camelCase(key)] = value
			}
		case "PRECISE", "CAN-SKIP-DATERANGES", "CAN-BLOCK-RELOAD":
			result[camelCase(key)] = regexp.MustCompile(`YES`).MatchString(value)
		case "SKIPPED-SEGMENTS":
			if intVal, err := strconv.Atoi(value); err == nil {
				result[camelCase(key)] = intVal
			} else {
				result[camelCase(key)] = value
			}
		default:
			result[camelCase(key)] = value
		}
	}

	return result
}

// IsMasterPlaylist returns true if the manifest represents a master playlist
// A master playlist contains variant streams (EXT-X-STREAM-INF) or I-frame playlists (EXT-X-I-FRAME-STREAM-INF)
func (p *Parser) IsMasterPlaylist() bool {
	return len(p.Manifest.Playlists) > 0 || len(p.Manifest.IFramePlaylists) > 0
}
