# Go M3U8 Parser

[![Go Reference](https://pkg.go.dev/badge/github.com/ar13101085/go-m3u8-parser.svg)](https://pkg.go.dev/github.com/ar13101085/go-m3u8-parser)
[![GitHub](https://img.shields.io/github/license/ar13101085/go-m3u8-parser)](https://github.com/ar13101085/go-m3u8-parser/blob/main/LICENSE)

A robust M3U8 parser in Go for HLS (HTTP Live Streaming) playlists. This package can parse both master playlists (containing variant streams) and media playlists (containing segments).

## Features

- Parses M3U8 files (both master playlists and media playlists)
- Supports all standard HLS tags
- Handles byterange requests
- Supports encryption information
- Extrapolates program date time information
- Parses custom tags
- Detects master playlists (`IsMasterPlaylist()` function)
- Handles variant streams with bandwidth information
- Supports low-latency HLS
- Full support for media groups (audio, video, subtitles)

## Installation

```bash
go get github.com/ar13101085/go-m3u8-parser
```

## Quick Start

```go
package main

import (
	"fmt"
	"io/ioutil"
	"log"

	"github.com/ar13101085/go-m3u8-parser/m3u8/parser"
)

func main() {
	// Read the file
	data, err := ioutil.ReadFile("playlist.m3u8")
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}

	// Create a parser
	p := parser.NewParser(map[string]interface{}{
		"uri": "playlist.m3u8",
	})
	
	// Parse the data
	p.Push(string(data))
	p.End()

	// Check if it's a master playlist
	if p.IsMasterPlaylist() {
		fmt.Println("This is a master playlist")
		
		// Access variant streams
		for i, playlist := range p.Manifest.Playlists {
			fmt.Printf("Variant %d: %s\n", i+1, playlist.URI)
			if bandwidth, ok := playlist.Attributes["BANDWIDTH"]; ok {
				fmt.Printf("  Bandwidth: %s\n", bandwidth)
			}
		}
	} else {
		// Access the parsed manifest for a media playlist
		fmt.Printf("Target Duration: %v\n", p.Manifest.TargetDuration)
		fmt.Printf("Number of segments: %d\n", len(p.Manifest.Segments))
	}
}
```

## Documentation

For detailed documentation, see [DOCUMENTATION.md](DOCUMENTATION.md).

## Supported HLS Tags

The parser supports all standard HLS tags including:

- `#EXTM3U`
- `#EXT-X-VERSION`
- `#EXT-X-TARGETDURATION`
- `#EXT-X-MEDIA-SEQUENCE`
- `#EXT-X-DISCONTINUITY-SEQUENCE`
- `#EXTINF`
- `#EXT-X-KEY`
- `#EXT-X-MAP`
- `#EXT-X-PROGRAM-DATE-TIME`
- `#EXT-X-BYTERANGE`
- `#EXT-X-DISCONTINUITY`
- `#EXT-X-ENDLIST`
- `#EXT-X-PLAYLIST-TYPE`
- `#EXT-X-STREAM-INF`
- `#EXT-X-I-FRAME-STREAM-INF`
- `#EXT-X-MEDIA`
- `#EXT-X-START`
- `#EXT-X-INDEPENDENT-SEGMENTS`
- `#EXT-X-DATERANGE`
- `#EXT-X-SERVER-CONTROL`
- `#EXT-X-PART`
- `#EXT-X-PART-INF`
- `#EXT-X-DEFINE`
- `#EXT-X-I-FRAMES-ONLY`

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details. 