# M3U8 Parser Documentation

## Overview

This project implements an M3U8 parser in Go for HLS (HTTP Live Streaming) playlists. It provides a robust API for parsing both master playlists (containing variant streams) and media playlists (containing segments).

## Installation

```bash
# Install the package
go get github.com/ar13101085/go-m3u8-parser

# Or clone the repository
git clone https://github.com/ar13101085/go-m3u8-parser.git
cd go-m3u8-parser
go build
```

## Import

```go
import "github.com/ar13101085/go-m3u8-parser/m3u8/parser"
```

## Basic Usage

```go
import (
    "fmt"
    "io/ioutil"
    
    "github.com/ar13101085/go-m3u8-parser/m3u8/parser"
)

func main() {
    // Read M3U8 content
    data, err := ioutil.ReadFile("playlist.m3u8")
    if err != nil {
        panic(err)
    }
    
    // Create parser
    p := parser.NewParser(map[string]interface{}{
        "uri": "playlist.m3u8",
    })
    
    // Parse content
    p.Push(string(data))
    p.End()
    
    // Access parsed data
    if p.IsMasterPlaylist() {
        // Handle master playlist...
    } else {
        // Handle media playlist...
    }
}
```

## Core Components

The parser consists of four main components:

1. `stream`: Base event emitter implementation
2. `linestream`: Converts input strings into line-by-line events
3. `parsestream`: Parses lines into M3U8 tag events
4. `parser`: Builds a complete manifest representation

## API Reference

### Parser

#### Creation

```go
p := parser.NewParser(map[string]interface{}{
    "uri": "playlist.m3u8",  // Optional URI for the playlist
    "mainDefinitions": map[string]string{}, // Optional variable definitions
})
```

#### Methods

- `Push(chunk string)`: Process a chunk of M3U8 content
- `End()`: Finalize parsing and trigger end events
- `IsMasterPlaylist() bool`: Returns true if the manifest is a master playlist
- `AddParser(options map[string]interface{})`: Add support for custom tags
- `AddTagMapper(options map[string]interface{})`: Add custom tag mappings

### Data Structures

#### Manifest

The `Manifest` struct contains all parsed information:

```go
type Manifest struct {
    // Basic information
    AllowCache            bool
    Version               int
    TargetDuration        int
    MediaSequence         int
    DiscontinuitySequence int
    EndList               bool
    PlaylistType          string
    DateTimeString        string
    DateTimeObject        time.Time
    
    // Segments (for media playlists)
    Segments              []*Segment
    PreloadSegment        *Segment
    
    // Feature flags
    IFramesOnly           bool
    IndependentSegments   bool
    
    // Additional information
    Start                 *Start
    DtsCptOffset          float64
    Skip                  map[string]interface{}
    ServerControl         map[string]interface{}
    PartInf               map[string]interface{}
    PartTargetDuration    float64
    
    // HLS features
    RenditionReports      []map[string]interface{}
    Playlists             []*Segment // Variant streams for master playlist
    IFramePlaylists       []*IFramePlaylist
    DiscontinuityStarts   []int
    DateRanges            []*DateRange
    ContentProtection     map[string]interface{}
    ContentSteering       map[string]interface{}
    MediaGroups           map[string]map[string]map[string]*MediaGroup
    Custom                map[string]interface{}
    Definitions           map[string]string
}
```

#### Segment

Represents an HLS segment or variant stream:

```go
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
    Attributes      map[string]string  // For variant streams
}
```

#### Other Structures

- `Map`: Initialization segment information (URI, Byterange, Key)
- `Key`: Encryption details (Method, URI, IV)
- `Start`: Playlist start information (TimeOffset, Precise)
- `DateRange`: Date range information for timed metadata
- `IFramePlaylist`: I-Frame playlist information
- `MediaGroup`: Audio/video/subtitle rendition information

## Working with Master Playlists

```go
if p.IsMasterPlaylist() {
    // Access variant streams
    for _, variant := range p.Manifest.Playlists {
        fmt.Printf("URI: %s\n", variant.URI)
        
        // Access variant attributes
        if bandwidth, ok := variant.Attributes["BANDWIDTH"]; ok {
            fmt.Printf("Bandwidth: %s\n", bandwidth)
        }
        if resolution, ok := variant.Attributes["RESOLUTION"]; ok {
            fmt.Printf("Resolution: %s\n", resolution)
        }
    }
    
    // Access iframe playlists
    for _, iframe := range p.Manifest.IFramePlaylists {
        fmt.Printf("I-Frame URI: %s\n", iframe.URI)
        fmt.Printf("Bandwidth: %s\n", iframe.Attributes["BANDWIDTH"])
    }
    
    // Access media groups (audio, video, subtitles)
    for mediaType, groups := range p.Manifest.MediaGroups {
        for groupId, renditions := range groups {
            for name, rendition := range renditions {
                fmt.Printf("Media: %s, Group: %s, Name: %s\n", 
                          mediaType, groupId, name)
                fmt.Printf("URI: %s\n", rendition.URI)
            }
        }
    }
}
```

## Working with Media Playlists

```go
if !p.IsMasterPlaylist() {
    // Access basic information
    fmt.Printf("Target Duration: %d\n", p.Manifest.TargetDuration)
    fmt.Printf("Media Sequence: %d\n", p.Manifest.MediaSequence)
    fmt.Printf("End List: %t\n", p.Manifest.EndList)
    
    // Access segments
    for _, segment := range p.Manifest.Segments {
        fmt.Printf("URI: %s\n", segment.URI)
        fmt.Printf("Duration: %.3f\n", segment.Duration)
        
        // Handle encryption
        if segment.Key != nil {
            fmt.Printf("Encryption: %s, URI: %s\n", 
                      segment.Key.Method, segment.Key.URI)
        }
        
        // Handle byterange
        if segment.Byterange != nil {
            fmt.Printf("Byterange: Length=%d, Offset=%d\n",
                      segment.Byterange.Length, segment.Byterange.Offset)
        }
    }
    
    // Access date ranges
    for _, dateRange := range p.Manifest.DateRanges {
        fmt.Printf("Date Range ID: %s\n", dateRange.ID)
        fmt.Printf("Start: %s, Duration: %.1f\n", 
                  dateRange.StartDate, dateRange.Duration)
    }
}
```

## Advanced Features

### Server Control

Access server control information:

```go
if p.Manifest.ServerControl != nil {
    if holdBack, ok := p.Manifest.ServerControl["holdBack"].(float64); ok {
        fmt.Printf("Hold Back: %.1f seconds\n", holdBack)
    }
    if canSkip, ok := p.Manifest.ServerControl["canSkipUntil"].(float64); ok {
        fmt.Printf("Can Skip Until: %.1f seconds\n", canSkip)
    }
}
```

### Low-Latency HLS

Access part information for Low-Latency HLS:

```go
if p.Manifest.PartInf != nil {
    fmt.Printf("Part Target Duration: %.1f\n", p.Manifest.PartTargetDuration)
}

// Access part information for segments
for _, segment := range p.Manifest.Segments {
    if len(segment.Parts) > 0 {
        fmt.Printf("Segment has %d parts\n", len(segment.Parts))
        for i, part := range segment.Parts {
            fmt.Printf("Part %d: %v\n", i, part)
        }
    }
}
```

### Custom Data

Access custom tags:

```go
if p.Manifest.Custom != nil {
    for tagType, data := range p.Manifest.Custom {
        fmt.Printf("Custom tag %s: %v\n", tagType, data)
    }
}
```

## Example Application

The provided `main.go` file demonstrates a complete example of parsing M3U8 files and displaying their contents. Run it with:

```bash
go run main.go [path/to/playlist.m3u8]
```

## Extending the Parser

Add support for custom tags:

```go
p.AddParser(map[string]interface{}{
    "expression": regexp.MustCompile(`^#MY-CUSTOM-TAG:(.*)$`),
    "customType": "myCustomTag",
    "dataParser": func(line string) string {
        return line // Process the line
    },
    "segment": false, // Whether this is segment-level data
})
```

## Publishing to pkg.go.dev

This package is available on pkg.go.dev at:
[github.com/ar13101085/go-m3u8-parser](https://pkg.go.dev/github.com/ar13101085/go-m3u8-parser)

To use it in your project:

```bash
go get github.com/ar13101085/go-m3u8-parser
```

## Supported Features

The parser supports the following HLS features:

- Basic M3U8 parsing (EXT-X-VERSION, EXT-X-TARGETDURATION, etc.)
- Master playlists with variant streams (EXT-X-STREAM-INF)
- I-Frame playlists (EXT-X-I-FRAME-STREAM-INF)
- Media groups for audio/video selection (EXT-X-MEDIA)
- Discontinuities (EXT-X-DISCONTINUITY)
- Program date time (EXT-X-PROGRAM-DATE-TIME)
- Map segments (EXT-X-MAP)
- Byteranges (EXT-X-BYTERANGE)
- Encryption (EXT-X-KEY)
- Date ranges (EXT-X-DATERANGE)
- Start time specification (EXT-X-START)
- Server control (EXT-X-SERVER-CONTROL)
- Part information for low-latency HLS (EXT-X-PART)
- Independent segments (EXT-X-INDEPENDENT-SEGMENTS)
- Variable substitution (EXT-X-DEFINE)

## HLS Tag Support

| HLS Tag                       | Supported |
|-------------------------------|-----------|
| #EXTM3U                       | ✓         |
| #EXT-X-VERSION                | ✓         |
| #EXT-X-TARGETDURATION         | ✓         |
| #EXT-X-MEDIA-SEQUENCE         | ✓         |
| #EXT-X-DISCONTINUITY-SEQUENCE | ✓         |
| #EXTINF                       | ✓         |
| #EXT-X-KEY                    | ✓         |
| #EXT-X-MAP                    | ✓         |
| #EXT-X-PROGRAM-DATE-TIME      | ✓         |
| #EXT-X-BYTERANGE              | ✓         |
| #EXT-X-DISCONTINUITY          | ✓         |
| #EXT-X-ENDLIST                | ✓         |
| #EXT-X-PLAYLIST-TYPE          | ✓         |
| #EXT-X-STREAM-INF             | ✓         |
| #EXT-X-I-FRAME-STREAM-INF     | ✓         |
| #EXT-X-MEDIA                  | ✓         |
| #EXT-X-START                  | ✓         |
| #EXT-X-INDEPENDENT-SEGMENTS   | ✓         |
| #EXT-X-DATERANGE              | ✓         |
| #EXT-X-SERVER-CONTROL         | ✓         |
| #EXT-X-PART                   | ✓         |
| #EXT-X-PART-INF               | ✓         |
| #EXT-X-DEFINE                 | ✓         |
| #EXT-X-I-FRAMES-ONLY          | ✓         |

## License

This project is licensed under the MIT License. See [LICENSE](https://github.com/ar13101085/go-m3u8-parser/blob/main/LICENSE) for details.

## Repository

[https://github.com/ar13101085/go-m3u8-parser](https://github.com/ar13101085/go-m3u8-parser) 