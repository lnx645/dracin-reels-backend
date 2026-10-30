package http

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func box(typ string, payload []byte) []byte {
	out := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(out[0:4], uint32(8+len(payload)))
	copy(out[4:8], typ)
	copy(out[8:], payload)
	return out
}

func mvhdV0(timescale, duration uint32) []byte {
	p := make([]byte, 100)
	p[0] = 0 // version 0
	binary.BigEndian.PutUint32(p[12:16], timescale)
	binary.BigEndian.PutUint32(p[16:20], duration)
	return box("mvhd", p)
}

func mvhdV1(timescale uint32, duration uint64) []byte {
	p := make([]byte, 100)
	p[0] = 1 // version 1
	binary.BigEndian.PutUint32(p[20:24], timescale)
	binary.BigEndian.PutUint64(p[24:32], duration)
	return box("mvhd", p)
}

func writeTemp(t *testing.T, ext string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test"+ext)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestProbeMP4V0(t *testing.T) {
	// ftyp + moov(mvhd 123456/1000) + mdat: moov BEFORE mdat.
	data := append(box("ftyp", []byte("isom\x00\x00\x00\x00isom")), box("moov", mvhdV0(1000, 123456))...)
	data = append(data, box("mdat", make([]byte, 64))...)
	if got := probeVideoDuration(writeTemp(t, ".mp4", data)); got != 123 {
		t.Fatalf("v0: got %d, want 123", got)
	}
}

func TestProbeMP4V1MoovAfterMdat(t *testing.T) {
	// mdat FIRST, moov after: parser must seek past mdat, not stop early.
	data := append(box("ftyp", []byte("isom\x00\x00\x00\x00isom")), box("mdat", make([]byte, 128))...)
	data = append(data, box("moov", mvhdV1(1000, 9000))...)
	if got := probeVideoDuration(writeTemp(t, ".mov", data)); got != 9 {
		t.Fatalf("v1 after mdat: got %d, want 9", got)
	}
}

func TestProbeMP4NoMoov(t *testing.T) {
	data := append(box("ftyp", []byte("isom\x00\x00\x00\x00isom")), box("mdat", make([]byte, 16))...)
	if got := probeVideoDuration(writeTemp(t, ".mp4", data)); got != 0 {
		t.Fatalf("no moov: got %d, want 0", got)
	}
}

func TestProbeMP4Garbage(t *testing.T) {
	if got := probeVideoDuration(writeTemp(t, ".mp4", []byte("this is not a video file at all........"))); got != 0 {
		t.Fatalf("garbage: got %d, want 0", got)
	}
}

// ebml helpers: IDs are written raw; sizes use the 1-byte form (0x80|len).
func ebmlElem(id []byte, payload []byte) []byte {
	out := []byte{}
	out = append(out, id...)
	if len(payload) < 127 {
		out = append(out, byte(0x80|len(payload)))
	} else {
		// Sizes in this test stay small; fail loudly otherwise.
		panic("test payload too large")
	}
	return append(out, payload...)
}

func TestProbeMKV(t *testing.T) {
	ts := []byte{0x0F, 0x42, 0x40} // 1_000_000
	dur := make([]byte, 8)
	binary.BigEndian.PutUint64(dur, math.Float64bits(90500.0))
	infoPayload := append(ebmlElem([]byte{0x2A, 0xD7, 0xB1}, ts), ebmlElem([]byte{0x44, 0x89}, dur)...)
	segPayload := ebmlElem([]byte{0x15, 0x49, 0xA9, 0x66}, infoPayload)
	data := ebmlElem([]byte{0x18, 0x53, 0x80, 0x67}, segPayload)
	if got := probeVideoDuration(writeTemp(t, ".mkv", data)); got != 91 {
		t.Fatalf("mkv: got %d, want 91", got)
	}
}

func TestProbeMKVNoDuration(t *testing.T) {
	segPayload := ebmlElem([]byte{0x15, 0x49, 0xA9, 0x66}, []byte{})
	data := ebmlElem([]byte{0x18, 0x53, 0x80, 0x67}, segPayload)
	if got := probeVideoDuration(writeTemp(t, ".mkv", data)); got != 0 {
		t.Fatalf("mkv no duration: got %d, want 0", got)
	}
}
