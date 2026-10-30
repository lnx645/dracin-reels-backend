package transcode

import (
	"strings"
	"testing"
)

func TestRenditionsForHeight(t *testing.T) {
	tests := []struct {
		srcW, srcH int
		wantNames  []string
	}{
		{1280, 720, []string{"360p", "480p", "720p"}},
		{854, 480, []string{"360p", "480p"}},
		{640, 360, []string{"360p"}},
		{426, 240, []string{"240p"}},
		{1920, 1080, []string{"360p", "480p", "720p"}},
	}
	for _, tc := range tests {
		got := renditionsForHeight(tc.srcW, tc.srcH)
		if len(got) != len(tc.wantNames) {
			t.Fatalf("%dx%d: got %d renditions, want %d", tc.srcW, tc.srcH, len(got), len(tc.wantNames))
		}
		for i, name := range tc.wantNames {
			if got[i].Name != name {
				t.Errorf("%dx%d: rendition %d = %s, want %s", tc.srcW, tc.srcH, i, got[i].Name, name)
			}
			if got[i].Width%2 != 0 {
				t.Errorf("%dx%d: rendition %s width %d is odd (libx264 needs even)", tc.srcW, tc.srcH, name, got[i].Width)
			}
			if got[i].Height > tc.srcH {
				t.Errorf("%dx%d: rendition %s upscales (%d > %d)", tc.srcW, tc.srcH, name, got[i].Height, tc.srcH)
			}
		}
	}
}

func TestMasterPlaylist(t *testing.T) {
	pl := masterPlaylist([]rendition{
		{Name: "360p", Height: 360, Width: 640, Bitrate: 800_000},
		{Name: "720p", Height: 720, Width: 1280, Bitrate: 2_800_000},
	}, true)
	if !strings.HasPrefix(pl, "#EXTM3U\n") {
		t.Errorf("playlist must start with #EXTM3U, got:\n%s", pl)
	}
	for _, want := range []string{
		"#EXT-X-STREAM-INF:BANDWIDTH=928000,RESOLUTION=640x360\n360p.m3u8",
		"#EXT-X-STREAM-INF:BANDWIDTH=2928000,RESOLUTION=1280x720\n720p.m3u8",
	} {
		if !strings.Contains(pl, want) {
			t.Errorf("playlist missing %q, got:\n%s", want, pl)
		}
	}
}

func TestDiskPath(t *testing.T) {
	if got := diskPath("media", "media/episodes/abc.mp4"); got != "media/episodes/abc.mp4" {
		t.Errorf("uploaded rel: got %q", got)
	}
	if got := diskPath("/opt/x/media", "media/episodes/abc.mp4"); got != "/opt/x/media/episodes/abc.mp4" {
		t.Errorf("absolute media dir: got %q", got)
	}
	if got := diskPath("media", "https://example.com/v.mp4"); got != "" {
		t.Errorf("external link must resolve to empty, got %q", got)
	}
	if got := diskPath("media", "http://example.com/v.mp4"); got != "" {
		t.Errorf("external http link must resolve to empty, got %q", got)
	}
}
