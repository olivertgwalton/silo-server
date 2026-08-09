package playback

import (
	"bytes"
	"context"
	"log/slog"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// Dolby Vision preservation through a stream copy is an FFmpeg 8 capability.
//
// Measured against a Profile 5 source (dvhe sample entry + dvcC box):
//
//	FFmpeg 8.1  -tag:v dvh1 -strict experimental  -> dvh1 + dvcC preserved
//	FFmpeg 8.1  -tag:v dvh1                       -> dvh1, dvcC dropped
//	FFmpeg 8.1  -tag:v dvhe                       -> muxer error, no output
//	FFmpeg 7.1  -tag:v dvhe                       -> dvhe + dvvC preserved
//	FFmpeg 7.1  -tag:v dvh1                       -> dvh1, box dropped
//
// So the recipe that works is version-specific, and on 7 there is no recipe
// that produces the dvh1 the Apple clients need. Rather than emit a file that
// claims Dolby Vision and decodes as plain HEVC, the preserve route is gated
// on the major version of the binary that will actually run it.
const minimumDVCopyTaggingMajorVersion = 8

var (
	dvCopyTaggingMu    sync.Mutex
	dvCopyTaggingCache map[string]bool
)

// ffmpegVersionPattern matches the major version in the first line of
// `ffmpeg -version`, covering the plain upstream form ("ffmpeg version
// 8.1.2"), Debian/Jellyfin builds ("ffmpeg version 7.1.1-Jellyfin") and
// git snapshots ("ffmpeg version n8.0-12-gabc1234").
var ffmpegVersionPattern = regexp.MustCompile(`ffmpeg version n?(\d+)\.`)

// parseFFmpegMajorVersion extracts the major version from `ffmpeg -version`
// output. Returns 0 when the banner cannot be understood, which callers treat
// as "not capable" rather than guessing.
func parseFFmpegMajorVersion(banner []byte) int {
	match := ffmpegVersionPattern.FindSubmatch(banner)
	if len(match) != 2 {
		return 0
	}
	major, err := strconv.Atoi(string(match[1]))
	if err != nil {
		return 0
	}
	return major
}

// supportsDVCopyTagging reports whether the given FFmpeg binary can label a
// copied Dolby Vision stream `dvh1` while retaining its configuration record.
// Probed once per binary path, like the dovi_rpu capability.
func supportsDVCopyTagging(bin string) bool {
	dvCopyTaggingMu.Lock()
	defer dvCopyTaggingMu.Unlock()
	if available, ok := dvCopyTaggingCache[bin]; ok {
		return available
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "-hide_banner", "-version").Output()
	major := 0
	if err == nil {
		major = parseFFmpegMajorVersion(bytes.ToLower(out))
	}
	available := major >= minimumDVCopyTaggingMajorVersion
	if !available {
		slog.Warn("ffmpeg cannot preserve Dolby Vision through a stream copy (needs FFmpeg 8+); the validated preserve remux is disabled",
			"ffmpeg", bin, "major_version", major)
	}
	if dvCopyTaggingCache == nil {
		dvCopyTaggingCache = make(map[string]bool)
	}
	dvCopyTaggingCache[bin] = available
	return available
}
