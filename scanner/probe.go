package scanner

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// MediaInfo holds probed video file metadata.
type MediaInfo struct {
	Duration        float64 // seconds
	VideoCodec      string  // e.g. "HEVC", "H.264"
	AudioCodec      string  // e.g. "AAC", "AC-3"
	AudioChannels   int     // e.g. 2, 6
	AudioSampleRate int     // e.g. 48000
	Width           int
	Height          int
	BitrateBps      int64 // overall file bitrate in bits/sec (computed from size & duration)
}

// DurationText returns duration as HH:MM:SS.
func (m *MediaInfo) DurationText() string {
	if m.Duration <= 0 {
		return ""
	}
	total := int(m.Duration)
	h := total / 3600
	min := (total % 3600) / 60
	sec := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, min, sec)
	}
	return fmt.Sprintf("%d:%02d", min, sec)
}

// Resolution returns e.g. "3840x2160".
func (m *MediaInfo) Resolution() string {
	if m.Width > 0 && m.Height > 0 {
		return fmt.Sprintf("%dx%d", m.Width, m.Height)
	}
	return ""
}

// BitrateText returns e.g. "28.3 Mbps".
func (m *MediaInfo) BitrateText() string {
	if m.BitrateBps <= 0 {
		return ""
	}
	mbps := float64(m.BitrateBps) / 1_000_000
	if mbps >= 10 {
		return fmt.Sprintf("%.0f Mbps", mbps)
	}
	return fmt.Sprintf("%.1f Mbps", mbps)
}

// Probe runs mkvmerge -J on the file and parses the result.
func Probe(path string) (*MediaInfo, error) {
	cmd := exec.Command("mkvmerge", "-J", path)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("mkvmerge -J: %w", err)
	}

	var result mkvIdentify
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parse mkvmerge output: %w", err)
	}

	info := &MediaInfo{}

	// Duration from container
	if result.Container.Properties.Duration > 0 {
		info.Duration = float64(result.Container.Properties.Duration) / 1_000_000_000 // ns → s
	}

	for _, track := range result.Tracks {
		codec := simplifyCodec(track.Codec)
		switch track.Type {
		case "video":
			if info.VideoCodec == "" {
				info.VideoCodec = codec
				info.Width, info.Height = parsePixelDimensions(track.Properties.PixelDimensions)
			}
		case "audio":
			if info.AudioCodec == "" {
				info.AudioCodec = codec
				info.AudioChannels = track.Properties.AudioChannels
				info.AudioSampleRate = track.Properties.AudioSampleRate
			}
		}
	}

	return info, nil
}

// mkvmerge -J output structure (only fields we need)
type mkvIdentify struct {
	Container struct {
		Properties struct {
			Duration int64 `json:"duration"` // nanoseconds (available for MKV, not MP4)
		} `json:"properties"`
	} `json:"container"`
	Tracks []struct {
		Type       string `json:"type"` // "video", "audio", "subtitles"
		Codec      string `json:"codec"`
		Properties struct {
			PixelDimensions string `json:"pixel_dimensions"` // e.g. "8192x4096"
			AudioChannels   int    `json:"audio_channels"`
			AudioSampleRate int    `json:"audio_sampling_frequency"`
		} `json:"properties"`
	} `json:"tracks"`
}

// parsePixelDimensions parses "8192x4096" into width, height.
func parsePixelDimensions(s string) (int, int) {
	var w, h int
	fmt.Sscanf(s, "%dx%d", &w, &h)
	return w, h
}

// simplifyCodec normalizes codec names for display.
func simplifyCodec(codec string) string {
	c := strings.ToUpper(codec)
	switch {
	case strings.Contains(c, "H.265"), strings.Contains(c, "HEVC"), strings.Contains(c, "MPEGH"):
		return "HEVC"
	case strings.Contains(c, "H.264"), strings.Contains(c, "AVC"), strings.Contains(c, "MPEG4P10"):
		return "H.264"
	case strings.Contains(c, "AAC"):
		return "AAC"
	case strings.Contains(c, "AC-3"), strings.Contains(c, "AC3"):
		return "AC-3"
	case strings.Contains(c, "DTS"):
		return "DTS"
	case strings.Contains(c, "OPUS"):
		return "Opus"
	case strings.Contains(c, "VORBIS"):
		return "Vorbis"
	case strings.Contains(c, "FLAC"):
		return "FLAC"
	case strings.Contains(c, "MP3"), strings.Contains(c, "MPEG AUDIO"):
		return "MP3"
	default:
		return codec
	}
}
