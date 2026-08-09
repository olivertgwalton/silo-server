package playback

import "testing"

func TestParseFFmpegMajorVersion(t *testing.T) {
	cases := []struct {
		name   string
		banner string
		want   int
	}{
		{"upstream release", "ffmpeg version 8.1.2 Copyright (c) 2000-2026 the FFmpeg developers", 8},
		{"jellyfin build", "ffmpeg version 7.1.1-Jellyfin Copyright (c) 2000-2024", 7},
		{"git snapshot", "ffmpeg version n8.0-12-gabc1234 Copyright (c) 2000-2025", 8},
		{"debian revision", "ffmpeg version 6.1.1-3ubuntu5 Copyright (c) 2000-2023", 6},
		{"unparseable", "some other tool version 9.9", 0},
		{"empty", "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseFFmpegMajorVersion([]byte(tc.banner)); got != tc.want {
				t.Fatalf("parseFFmpegMajorVersion(%q) = %d, want %d", tc.banner, got, tc.want)
			}
		})
	}
}

// The preserve recipe is only honoured on FFmpeg 8+, where a stream copy
// retains the DOVI configuration record. Anything older must read as
// incapable so the planner keeps the route off the table.
func TestDVCopyTaggingMinimumVersion(t *testing.T) {
	for major, want := range map[int]bool{6: false, 7: false, 8: true, 9: true} {
		if got := major >= minimumDVCopyTaggingMajorVersion; got != want {
			t.Fatalf("major %d: capable = %v, want %v", major, got, want)
		}
	}
}

// buildRemuxArgs must emit both halves of the recipe: the dvh1 sample entry
// that Media3 and AVPlayer key their decoder selection from, and the
// -strict experimental that makes FFmpeg 8 carry dvcC through the copy.
// The tag without the record is a decoder trap, so they travel together.
func TestBuildRemuxArgsPreservesDolbyVisionSignalling(t *testing.T) {
	for _, profile := range []int{5, 8} {
		args := buildRemuxArgs("/tmp/in.mkv", "mp4", 0, false, 0, profile, true)
		var sawTag, sawStrict bool
		for i, arg := range args {
			if arg == "-tag:v" && i+1 < len(args) && args[i+1] == "dvh1" {
				sawTag = true
			}
			if arg == "-strict" && i+1 < len(args) && args[i+1] == "experimental" {
				sawStrict = true
			}
		}
		if !sawTag {
			t.Fatalf("profile %d: expected -tag:v dvh1 in %v", profile, args)
		}
		if !sawStrict {
			t.Fatalf("profile %d: expected -strict experimental in %v", profile, args)
		}
	}
}

// Without the preserve recipe the output must stay honestly labelled: no
// Dolby Vision sample entry on a stream whose configuration record was not
// carried through.
func TestBuildRemuxArgsOmitsDolbyVisionTagWhenNotPreserving(t *testing.T) {
	args := buildRemuxArgs("/tmp/in.mkv", "mp4", 0, false, 0, 5, false)
	for i, arg := range args {
		if arg == "-tag:v" && i+1 < len(args) && (args[i+1] == "dvh1" || args[i+1] == "dvhe") {
			t.Fatalf("expected no Dolby Vision sample entry tag, got %v", args)
		}
	}
}
