package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/ar13101085/go-m3u8-parser/m3u8/parser"
)

func main() {
	// Check if a file path is provided as an argument
	filePath := "input.m3u8"
	if len(os.Args) > 1 {
		filePath = os.Args[1]
	}
	// Read the file
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}
	// Parse the m3u8 content
	p := parser.NewParser(map[string]interface{}{
		"uri": filePath,
	})

	p.Push(string(data))
	p.End()

	// Output the parsed manifest details
	fmt.Printf("Manifest parsed successfully:\n")
	fmt.Printf("Is Master Playlist: %v\n", p.IsMasterPlaylist())
	fmt.Printf("Version: %d\n", p.Manifest.Version)
	fmt.Printf("Allow Cache: %v\n", p.Manifest.AllowCache)
	fmt.Printf("Media Sequence: %v\n", p.Manifest.MediaSequence)
	fmt.Printf("Target Duration: %v\n", p.Manifest.TargetDuration)
	fmt.Printf("EndList: %v\n", p.Manifest.EndList)
	fmt.Printf("Playlist Type: %v\n", p.Manifest.PlaylistType)
	fmt.Printf("Independent Segments: %v\n", p.Manifest.IndependentSegments)
	fmt.Printf("IFrames Only: %v\n", p.Manifest.IFramesOnly)

	if p.Manifest.Start != nil {
		fmt.Printf("\nStart:\n")
		fmt.Printf("  Time Offset: %.1f\n", p.Manifest.Start.TimeOffset)
		fmt.Printf("  Precise: %v\n", p.Manifest.Start.Precise)
	}

	if len(p.Manifest.DiscontinuityStarts) > 0 {
		fmt.Printf("\nDiscontinuity Starts: %v\n", p.Manifest.DiscontinuityStarts)
	}

	if p.Manifest.DateTimeString != "" {
		fmt.Printf("\nProgram Date Time: %s\n", p.Manifest.DateTimeString)
	}

	if len(p.Manifest.DateRanges) > 0 {
		fmt.Printf("\nDate Ranges:\n")
		for i, dateRange := range p.Manifest.DateRanges {
			fmt.Printf("  Date Range %d:\n", i+1)
			fmt.Printf("    ID: %s\n", dateRange.ID)
			fmt.Printf("    Start Date: %s\n", dateRange.StartDate)
			fmt.Printf("    Duration: %.1f\n", dateRange.Duration)
		}
	}

	if p.Manifest.ServerControl != nil {
		fmt.Printf("\nServer Control:\n")
		for k, v := range p.Manifest.ServerControl {
			fmt.Printf("  %s: %v\n", k, v)
		}
	}

	if p.Manifest.PartInf != nil {
		fmt.Printf("\nPart Inf:\n")
		for k, v := range p.Manifest.PartInf {
			fmt.Printf("  %s: %v\n", k, v)
		}
	}

	if len(p.Manifest.Segments) > 0 {
		fmt.Printf("\nSegments: %d\n", len(p.Manifest.Segments))
		// Print only a few segments to keep output manageable
		for i, segment := range p.Manifest.Segments[:5] {
			fmt.Printf("  Segment %d:\n", i+1)
			fmt.Printf("    URI: %s\n", segment.URI)
			fmt.Printf("    Duration: %.3f\n", segment.Duration)
			fmt.Printf("    Timeline: %d\n", segment.Timeline)

			if segment.Title != "" {
				fmt.Printf("    Title: %s\n", segment.Title)
			}

			if segment.Discontinuity {
				fmt.Printf("    Discontinuity: true\n")
			}

			if segment.ProgramDateTime != 0 {
				fmt.Printf("    Program Date Time: %d\n", segment.ProgramDateTime)
			}

			if segment.Map != nil {
				fmt.Printf("    Map:\n")
				fmt.Printf("      URI: %s\n", segment.Map.URI)
				if segment.Map.Byterange != nil {
					fmt.Printf("      Byterange: Length=%d, Offset=%d\n",
						segment.Map.Byterange.Length, segment.Map.Byterange.Offset)
				}
			}

			if segment.Byterange != nil {
				fmt.Printf("    Byterange: Length=%d, Offset=%d\n",
					segment.Byterange.Length, segment.Byterange.Offset)
			}

			if segment.Key != nil {
				fmt.Printf("    Encryption:\n")
				fmt.Printf("      Method: %s\n", segment.Key.Method)
				fmt.Printf("      URI: %s\n", segment.Key.URI)
				if segment.Key.IV != "" {
					fmt.Printf("      IV: %s\n", segment.Key.IV)
				}
			}
		}
		if len(p.Manifest.Segments) > 5 {
			fmt.Printf("\n  ... and %d more segments\n", len(p.Manifest.Segments)-5)
		}
	}

	if len(p.Manifest.Playlists) > 0 {
		fmt.Printf("\nVariant Playlists: %d\n", len(p.Manifest.Playlists))
		for i, playlist := range p.Manifest.Playlists {
			fmt.Printf("  Playlist %d:\n", i+1)
			fmt.Printf("    URI: %s\n", playlist.URI)

			if playlist.Attributes != nil {
				// Display bandwidth information if available
				if bandwidth, ok := playlist.Attributes["BANDWIDTH"]; ok {
					fmt.Printf("    Bandwidth: %s\n", bandwidth)
				}
				// Display resolution if available
				if resolution, ok := playlist.Attributes["RESOLUTION"]; ok {
					fmt.Printf("    Resolution: %s\n", resolution)
				}
				// Display codecs if available
				if codecs, ok := playlist.Attributes["CODECS"]; ok {
					fmt.Printf("    Codecs: %s\n", codecs)
				}
			}
		}
	}

	if len(p.Manifest.IFramePlaylists) > 0 {
		fmt.Printf("\nI-Frame Playlists: %d\n", len(p.Manifest.IFramePlaylists))
		for i, playlist := range p.Manifest.IFramePlaylists {
			fmt.Printf("  I-Frame Playlist %d:\n", i+1)
			fmt.Printf("    URI: %s\n", playlist.URI)
			if playlist.Attributes != nil {
				fmt.Printf("    Bandwidth: %s\n", playlist.Attributes["BANDWIDTH"])
				fmt.Printf("    Resolution: %s\n", playlist.Attributes["RESOLUTION"])
			}
		}
	}

	if len(p.Manifest.MediaGroups) > 0 {
		fmt.Printf("\nMedia Groups:\n")
		for groupType, groups := range p.Manifest.MediaGroups {
			fmt.Printf("  Type: %s\n", groupType)
			for groupId, renditions := range groups {
				fmt.Printf("    Group ID: %s\n", groupId)
				for name, rendition := range renditions {
					fmt.Printf("      Name: %s\n", name)
					fmt.Printf("        Default: %v\n", rendition.Default)
					fmt.Printf("        Autoselect: %v\n", rendition.Autoselect)
					if rendition.Language != "" {
						fmt.Printf("        Language: %s\n", rendition.Language)
					}
					if rendition.URI != "" {
						fmt.Printf("        URI: %s\n", rendition.URI)
					}
				}
			}
		}
	}
}
