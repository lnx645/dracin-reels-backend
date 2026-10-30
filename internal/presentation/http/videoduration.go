package http

import (
	"encoding/binary"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// probeVideoDuration reads a video file's duration in whole seconds using
// only pure Go header parsing — no ffprobe or any other external tool exists
// on the host, so the server must not depend on one.
//
// Supported:
//   - MP4/MOV: top-level box walk to moov/mvhd, timescale + duration.
//     mvhd version 0 (32-bit fields) and version 1 (64-bit duration).
//     The walk seeks past mdat/free boxes, so moov AFTER mdat (non-faststart
//     files) is handled too. A corrupt or moov-less file returns 0.
//   - MKV: best-effort EBML scan for Segment/Info/Duration (+ TimestampScale,
//     default 1_000_000 ns). Any parse failure returns 0.
//
// Anything unparseable returns 0 — never a fabricated number. Callers treat
// 0 as "unknown" (the app hides the duration instead of showing "0:00").

// probeVideoDuration dispatches on file extension.
func probeVideoDuration(diskPath string) int {
	switch strings.ToLower(filepath.Ext(diskPath)) {
	case ".mp4", ".mov":
		return probeMP4Duration(diskPath)
	case ".mkv":
		return probeMKVEBMLDuration(diskPath)
	default:
		return 0
	}
}

const maxMP4Boxes = 10000 // sanity bound against pathological box chains

// probeMP4Duration parses moov/mvhd from an MP4/MOV file.
func probeMP4Duration(diskPath string) int {
	f, err := os.Open(diskPath)
	if err != nil {
		return 0
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size() < 8 {
		return 0
	}
	moov, ok := findMP4Box(f, 0, info.Size(), "moov")
	if !ok {
		return 0
	}
	mvhd, ok := findMP4Box(f, moov.contentOff, moov.contentOff+moov.contentLen, "mvhd")
	if !ok {
		return 0
	}
	return parseMVHD(f, mvhd.contentOff, mvhd.contentLen)
}

type mp4Box struct {
	contentOff int64
	contentLen int64
}

// findMP4Box scans sibling boxes in [start, end) for the first box of the
// given type, seeking past (never reading) large payload boxes like mdat.
func findMP4Box(f *os.File, start, end int64, want string) (mp4Box, bool) {
	off := start
	for i := 0; i < maxMP4Boxes && off+8 <= end; i++ {
		if _, err := f.Seek(off, io.SeekStart); err != nil {
			return mp4Box{}, false
		}
		var hdr [8]byte
		if _, err := io.ReadFull(f, hdr[:]); err != nil {
			return mp4Box{}, false
		}
		size := int64(binary.BigEndian.Uint32(hdr[0:4]))
		typ := string(hdr[4:8])
		headerLen := int64(8)
		if size == 1 { // largesize
			var big [8]byte
			if _, err := io.ReadFull(f, big[:]); err != nil {
				return mp4Box{}, false
			}
			size = int64(binary.BigEndian.Uint64(big[:]))
			headerLen = 16
		} else if size == 0 { // extends to end of parent
			size = end - off
		}
		if size < headerLen {
			return mp4Box{}, false
		}
		if typ == want {
			return mp4Box{contentOff: off + headerLen, contentLen: size - headerLen}, true
		}
		off += size
	}
	return mp4Box{}, false
}

// parseMVHD reads timescale + duration out of an mvhd box payload.
func parseMVHD(f *os.File, off, length int64) int {
	if length < 20 {
		return 0
	}
	if _, err := f.Seek(off, io.SeekStart); err != nil {
		return 0
	}
	n := length
	if n > 40 {
		n = 40
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(f, buf); err != nil {
		return 0
	}
	version := buf[0]
	var timescale uint64
	var duration uint64
	switch version {
	case 0:
		if len(buf) < 20 {
			return 0
		}
		timescale = uint64(binary.BigEndian.Uint32(buf[12:16]))
		duration = uint64(binary.BigEndian.Uint32(buf[16:20]))
	case 1:
		if len(buf) < 32 {
			return 0
		}
		timescale = uint64(binary.BigEndian.Uint32(buf[20:24]))
		duration = binary.BigEndian.Uint64(buf[24:32])
	default:
		return 0
	}
	if timescale == 0 || duration == 0 {
		return 0
	}
	// Round to the nearest whole second.
	return int((duration + timescale/2) / timescale)
}

// --- MKV (EBML) best-effort parsing ---------------------------------------
//
// Element layout: variable-length ID, variable-length size, payload.
// ID/size length is encoded by the position of the first set bit in the
// first byte (0x80 -> 1 byte, 0x40 -> 2, 0x20 -> 3, 0x10 -> 4). For sizes,
// an all-ones value means "unknown size" (extends to the parent's end).
// We only walk Segment -> Info -> {TimestampScale, Duration} and skip
// everything else by size, with a byte budget so a hostile file cannot
// make us scan forever.

const (
	ebmlIDSegment        = 0x18538067
	ebmlIDInfo           = 0x1549A966
	ebmlIDTimestampScale = 0x2AD7B1
	ebmlIDDuration       = 0x4489
)

const maxEBMLScanBytes = 32 << 20 // Info lives near the start of Segment

type ebmlReader struct {
	f       *os.File
	pos     int64
	end     int64
	scanned int64
}

func (r *ebmlReader) readByte() (byte, bool) {
	if r.pos >= r.end || r.scanned >= maxEBMLScanBytes {
		return 0, false
	}
	var b [1]byte
	if _, err := r.f.ReadAt(b[:], r.pos); err != nil {
		return 0, false
	}
	r.pos++
	r.scanned++
	return b[0], true
}

// readVint reads an EBML variable-length integer. For IDs the marker bit
// is part of the value; for sizes it is stripped, and all-ones means
// unknown (ok=false, unknown=true).
func (r *ebmlReader) readVint(forID bool) (uint64, bool, bool) {
	first, ok := r.readByte()
	if !ok {
		return 0, false, false
	}
	length := 1
	mask := byte(0x80)
	for first&mask == 0 {
		mask >>= 1
		length++
		if length > 8 {
			return 0, false, false
		}
	}
	var value uint64
	if forID {
		value = uint64(first)
	} else {
		value = uint64(first & (mask - 1))
	}
	for i := 1; i < length; i++ {
		b, ok := r.readByte()
		if !ok {
			return 0, false, false
		}
		value = value<<8 | uint64(b)
	}
	if !forID {
		allOnes := uint64(1)<<(uint(7*length)) - 1
		if value == allOnes {
			return 0, true, true
		}
	}
	return value, true, false
}

// readElement returns the next element's ID, payload offset and payload
// length (parentEnd when the size is unknown).
func (r *ebmlReader) readElement(parentEnd int64) (uint64, int64, int64, bool) {
	id, ok, _ := r.readVint(true)
	if !ok {
		return 0, 0, 0, false
	}
	size, ok, unknown := r.readVint(false)
	if !ok {
		return 0, 0, 0, false
	}
	payloadOff := r.pos
	var payloadLen int64
	if unknown {
		payloadLen = parentEnd - payloadOff
	} else {
		payloadLen = int64(size)
	}
	if payloadLen < 0 || payloadOff+payloadLen > parentEnd {
		return 0, 0, 0, false
	}
	return id, payloadOff, payloadLen, true
}

func (r *ebmlReader) skipTo(off int64) bool {
	if off < r.pos || off > r.end {
		return false
	}
	r.scanned += off - r.pos
	r.pos = off
	return r.scanned <= maxEBMLScanBytes
}

func (r *ebmlReader) readUint(off, length int64) (uint64, bool) {
	if length <= 0 || length > 8 {
		return 0, false
	}
	buf := make([]byte, length)
	if _, err := r.f.ReadAt(buf, off); err != nil {
		return 0, false
	}
	var v uint64
	for _, b := range buf {
		v = v<<8 | uint64(b)
	}
	return v, true
}

func (r *ebmlReader) readFloat(off, length int64) (float64, bool) {
	buf := make([]byte, length)
	if _, err := r.f.ReadAt(buf, off); err != nil {
		return 0, false
	}
	switch length {
	case 4:
		return float64(math.Float32frombits(binary.BigEndian.Uint32(buf))), true
	case 8:
		return math.Float64frombits(binary.BigEndian.Uint64(buf)), true
	default:
		return 0, false
	}
}

// probeMKVEBMLDuration scans Segment/Info for Duration + TimestampScale.
func probeMKVEBMLDuration(diskPath string) int {
	f, err := os.Open(diskPath)
	if err != nil {
		return 0
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size() < 8 {
		return 0
	}
	fileSize := info.Size()

	// Top level: find Segment.
	top := &ebmlReader{f: f, pos: 0, end: fileSize}
	var segOff, segLen int64
	for {
		id, off, length, ok := top.readElement(fileSize)
		if !ok {
			return 0
		}
		if id == ebmlIDSegment {
			segOff, segLen = off, length
			break
		}
		if !top.skipTo(off + length) {
			return 0
		}
	}
	segEnd := segOff + segLen

	// Inside Segment: find Info.
	seg := &ebmlReader{f: f, pos: segOff, end: segEnd}
	var infoOff, infoLen int64
	found := false
	for seg.pos < segEnd {
		id, off, length, ok := seg.readElement(segEnd)
		if !ok {
			return 0
		}
		if id == ebmlIDInfo {
			infoOff, infoLen = off, length
			found = true
			break
		}
		if !seg.skipTo(off + length) {
			return 0
		}
	}
	if !found {
		return 0
	}
	infoEnd := infoOff + infoLen

	// Inside Info: TimestampScale (default 1_000_000) + Duration.
	inf := &ebmlReader{f: f, pos: infoOff, end: infoEnd}
	timescale := uint64(1000000)
	var duration float64
	haveDuration := false
	for inf.pos < infoEnd {
		id, off, length, ok := inf.readElement(infoEnd)
		if !ok {
			return 0
		}
		switch id {
		case ebmlIDTimestampScale:
			if v, ok := inf.readUint(off, length); ok && v > 0 {
				timescale = v
			}
		case ebmlIDDuration:
			if v, ok := inf.readFloat(off, length); ok && v > 0 {
				duration = v
				haveDuration = true
			}
		}
		if !inf.skipTo(off + length) {
			return 0
		}
	}
	if !haveDuration {
		return 0
	}
	// Duration is in TimestampScale ticks (each tick = TimestampScale nanoseconds).
	seconds := duration * float64(timescale) / 1e9
	if seconds <= 0 {
		return 0
	}
	return int(seconds + 0.5)
}
