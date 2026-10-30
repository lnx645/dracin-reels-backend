// Package transcode turns an uploaded episode video into an adaptive HLS
// set (360p/480p/720p renditions + master playlist) so the player stays
// smooth on a slow connection instead of pulling one giant MP4.
//
// Design (the VPS is small: 3 CPUs, ~2.7 GB RAM):
//   - ONE worker goroutine, one ffmpeg job at a time, ffmpeg capped at
//     2 threads. Uploads never wait for a transcode: the upload returns
//     immediately with transcode_status "processing" and the player uses
//     the original MP4 until hls_url becomes "ready".
//   - Renditions are only produced at or below the source resolution
//     (a 480p source never gets upscaled to 720p; a sub-360p source gets
//     a single native-resolution rendition).
//   - Output goes to a temp dir first and is renamed into place only on
//     full success; the temp dir is always removed afterwards.
//   - Stale jobs are skipped, never applied: the worker re-reads the
//     episode and only writes a result when video_url still matches the
//     job's source (a re-upload replaces the source, so an old job must
//     not overwrite the new job's status or URL).
//   - Failure is honest: status "failed" + a plain reason, hls_url left
//     at its previous value (or null), original file still playable.
//   - ffmpeg/ffprobe come from PATH (RPM Fusion build on AlmaLinux),
//     overridable with FFMPEG_BIN / FFPROBE_BIN. If the binaries are
//     missing, jobs fail fast with a clear reason instead of hanging.
package transcode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lnx645/dracin-reels-backend/internal/domain"
)

// EpisodeStore is the narrow persistence surface the worker needs.
type EpisodeStore interface {
	GetEpisode(ctx context.Context, id int64) (*domain.Episode, error)
	UpdateEpisodeTranscode(ctx context.Context, id int64, hlsURL *string, status string, errMsg *string) error
	ListProcessingEpisodes(ctx context.Context) ([]domain.Episode, error)
}

// Job is one transcode request. SourceRel is the episode's video_url at
// enqueue time (used for the staleness check); SourceDisk is the on-disk
// path of that exact file.
type Job struct {
	EpisodeID  int64
	SourceRel  string
	SourceDisk string
}

// Transcoder is the single-worker HLS pipeline.
type Transcoder struct {
	store      EpisodeStore
	mediaDir   string
	ffmpegBin  string
	ffprobeBin string

	queue  chan Job
	mu     sync.Mutex
	queued map[int64]string // episodeID -> sourceRel of its latest job
}

const (
	// maxQueueJobs bounds memory: uploads beyond this get a clear
	// "queue full, re-upload" failure instead of blocking the request.
	maxQueueJobs = 128
	// jobTimeout bounds a runaway ffmpeg (a huge source on a small VPS).
	jobTimeout = 3 * time.Hour
	// ffmpegThreads leaves one CPU free for the API + database.
	ffmpegThreads = 2
	// segmentSeconds is the HLS segment length; the 2s GOP divides it.
	segmentSeconds = 6
)

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// New builds a Transcoder. Binaries are resolved lazily at job time, but a
// missing binary is logged loudly at startup so the operator notices.
func New(store EpisodeStore, mediaDir string) *Transcoder {
	ffmpegBin := getenv("FFMPEG_BIN", "ffmpeg")
	ffprobeBin := getenv("FFPROBE_BIN", "ffprobe")
	if _, err := exec.LookPath(ffmpegBin); err != nil {
		log.Printf("hls: WARNING: ffmpeg binary %q not found — transcode jobs will fail fast (MP4 fallback still works)", ffmpegBin)
	}
	if _, err := exec.LookPath(ffprobeBin); err != nil {
		log.Printf("hls: WARNING: ffprobe binary %q not found — transcode jobs will fail fast (MP4 fallback still works)", ffprobeBin)
	}
	return &Transcoder{
		store:      store,
		mediaDir:   mediaDir,
		ffmpegBin:  ffmpegBin,
		ffprobeBin: ffprobeBin,
		queue:      make(chan Job, maxQueueJobs),
		queued:     map[int64]string{},
	}
}

// Start launches the single worker goroutine.
func (t *Transcoder) Start() {
	go func() {
		for job := range t.queue {
			t.process(job)
			t.mu.Lock()
			if t.queued[job.EpisodeID] == job.SourceRel {
				delete(t.queued, job.EpisodeID)
			}
			t.mu.Unlock()
		}
	}()
}

// Enqueue schedules a job. It returns false when the exact same source is
// already queued/processing, or when the queue is full — the caller turns
// that into a clear failure instead of blocking the upload request.
func (t *Transcoder) Enqueue(episodeID int64, sourceRel, sourceDisk string) bool {
	t.mu.Lock()
	if t.queued[episodeID] == sourceRel {
		t.mu.Unlock()
		return false
	}
	t.queued[episodeID] = sourceRel
	t.mu.Unlock()
	select {
	case t.queue <- Job{EpisodeID: episodeID, SourceRel: sourceRel, SourceDisk: sourceDisk}:
		return true
	default:
		t.mu.Lock()
		if t.queued[episodeID] == sourceRel {
			delete(t.queued, episodeID)
		}
		t.mu.Unlock()
		return false
	}
}

// RequeueProcessing re-schedules jobs left "processing" by a restart.
// Episodes whose source file is gone are marked failed (previous HLS URL,
// if any, is kept) instead of being queued forever.
func (t *Transcoder) RequeueProcessing(ctx context.Context) error {
	eps, err := t.store.ListProcessingEpisodes(ctx)
	if err != nil {
		return err
	}
	for _, ep := range eps {
		disk := diskPath(t.mediaDir, ep.VideoURL)
		if disk == "" {
			msg := "sumber video bukan file upload (transcode dibatalkan saat server dimulai ulang)"
			_ = t.store.UpdateEpisodeTranscode(ctx, ep.ID, ep.HLSURL, "failed", &msg)
			continue
		}
		if _, err := os.Stat(disk); err != nil {
			msg := "file sumber tidak ditemukan saat server dimulai ulang"
			_ = t.store.UpdateEpisodeTranscode(ctx, ep.ID, ep.HLSURL, "failed", &msg)
			continue
		}
		if !t.Enqueue(ep.ID, ep.VideoURL, disk) {
			log.Printf("hls: episode %d: gagal mengantre ulang (duplikat/antrean penuh)", ep.ID)
		}
	}
	return nil
}

// diskPath resolves an episode's stored video_url to an on-disk path.
// External http(s) links return "" (they are never transcoded).
func diskPath(mediaDir, videoURL string) string {
	if strings.HasPrefix(videoURL, "http://") || strings.HasPrefix(videoURL, "https://") {
		return ""
	}
	rel := strings.TrimPrefix(videoURL, "media/")
	return filepath.Join(mediaDir, rel)
}

func (t *Transcoder) process(job Job) {
	ctx, cancel := context.WithTimeout(context.Background(), jobTimeout)
	defer cancel()
	ep, err := t.store.GetEpisode(ctx, job.EpisodeID)
	if err != nil {
		log.Printf("hls: episode %d: tidak dapat dibaca, job dibuang: %v", job.EpisodeID, err)
		return
	}
	// Staleness guard: a re-upload changes video_url (and re-marks the
	// episode processing with its own job), so an old job must not touch
	// the new state.
	if ep.VideoURL != job.SourceRel || ep.TranscodeStatus != "processing" {
		log.Printf("hls: episode %d: job basi dilewati (video sudah diganti)", job.EpisodeID)
		return
	}
	start := time.Now()
	rel, err := t.transcode(ctx, job)
	if err != nil {
		reason := strings.TrimSpace(err.Error())
		if len(reason) > 500 {
			reason = reason[:500]
		}
		log.Printf("hls: episode %d: gagal setelah %s: %s", job.EpisodeID, time.Since(start).Round(time.Second), reason)
		_ = t.store.UpdateEpisodeTranscode(context.Background(), job.EpisodeID, ep.HLSURL, "failed", &reason)
		return
	}
	log.Printf("hls: episode %d: siap dalam %s -> %s", job.EpisodeID, time.Since(start).Round(time.Second), rel)
	_ = t.store.UpdateEpisodeTranscode(context.Background(), job.EpisodeID, &rel, "ready", nil)
}

// rendition is one HLS variant.
type rendition struct {
	Name    string // e.g. "480p" (also the playlist file stem)
	Height  int
	Width   int
	Bitrate int // video bits per second
}

var renditionCandidates = []struct {
	name    string
	height  int
	bitrate int
}{
	{"360p", 360, 800_000},
	{"480p", 480, 1_400_000},
	{"720p", 720, 2_800_000},
}

// renditionsForHeight returns the variants at or below the source height,
// never upscaling. A source below 360p gets a single native rendition.
func renditionsForHeight(srcW, srcH int) []rendition {
	var out []rendition
	for _, c := range renditionCandidates {
		if srcH < c.height {
			continue
		}
		out = append(out, rendition{Name: c.name, Height: c.height, Width: scaledWidth(srcW, srcH, c.height), Bitrate: c.bitrate})
	}
	if len(out) == 0 {
		out = append(out, rendition{Name: fmt.Sprintf("%dp", srcH), Height: srcH, Width: srcW &^ 1, Bitrate: 600_000})
	}
	return out
}

// scaledWidth preserves aspect ratio and forces an even width (required
// by libx264 with yuv420p).
func scaledWidth(srcW, srcH, targetH int) int {
	if srcH <= 0 {
		return 2
	}
	w := srcW * targetH / srcH
	if w%2 == 1 {
		w--
	}
	if w < 2 {
		w = 2
	}
	return w
}

const audioBitrate = 128_000

// masterPlaylist builds the variant playlist referencing each rendition.
func masterPlaylist(renditions []rendition, hasAudio bool) string {
	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n")
	audio := 0
	if hasAudio {
		audio = audioBitrate
	}
	for _, r := range renditions {
		fmt.Fprintf(&b, "#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d\n%s.m3u8\n",
			r.Bitrate+audio, r.Width, r.Height, r.Name)
	}
	return b.String()
}

// probeMedia returns source dimensions + audio presence via ffprobe.
func (t *Transcoder) probeMedia(ctx context.Context, diskPath string) (width, height int, hasAudio bool, err error) {
	cmd := exec.CommandContext(ctx, t.ffprobeBin,
		"-v", "error",
		"-show_entries", "stream=codec_type,width,height",
		"-of", "json",
		diskPath)
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, false, fmt.Errorf("ffprobe gagal: %w", err)
	}
	var parsed struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return 0, 0, false, fmt.Errorf("ffprobe: output tidak dapat diparse: %w", err)
	}
	for _, s := range parsed.Streams {
		switch s.CodecType {
		case "video":
			if width == 0 && s.Width > 0 && s.Height > 0 {
				width, height = s.Width, s.Height
			}
		case "audio":
			hasAudio = true
		}
	}
	if width == 0 || height == 0 {
		return 0, 0, false, fmt.Errorf("ffprobe: tidak ada stream video di file sumber")
	}
	return width, height, hasAudio, nil
}

// tailWriter keeps only the last N bytes (ffmpeg progress output is long;
// only the tail is useful for a failure reason).
type tailWriter struct {
	buf bytes.Buffer
	max int
}

func (w *tailWriter) Write(p []byte) (int, error) {
	if len(p) >= w.max {
		w.buf.Reset()
		w.buf.Write(p[len(p)-w.max:])
		return len(p), nil
	}
	if w.buf.Len()+len(p) > w.max {
		over := w.buf.Len() + len(p) - w.max
		w.buf.Next(over)
	}
	w.buf.Write(p)
	return len(p), nil
}

func (t *Transcoder) transcode(ctx context.Context, job Job) (string, error) {
	width, height, hasAudio, err := t.probeMedia(ctx, job.SourceDisk)
	if err != nil {
		return "", err
	}
	renditions := renditionsForHeight(width, height)

	episodesDir := filepath.Join(t.mediaDir, "episodes")
	tmpDir, err := os.MkdirTemp(episodesDir, fmt.Sprintf(".hls_tmp_%d_", job.EpisodeID))
	if err != nil {
		return "", fmt.Errorf("tidak dapat membuat direktori sementara: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	for _, r := range renditions {
		args := []string{
			"-y",
			"-i", job.SourceDisk,
			"-vf", fmt.Sprintf("scale=-2:%d", r.Height),
			"-c:v", "libx264", "-preset", "veryfast", "-threads", fmt.Sprint(ffmpegThreads),
			"-b:v", fmt.Sprint(r.Bitrate), "-maxrate", fmt.Sprint(r.Bitrate), "-bufsize", fmt.Sprint(r.Bitrate * 2),
			"-profile:v", "high",
			"-g", "120", "-keyint_min", "60", "-sc_threshold", "0",
			"-force_key_frames", "expr:gte(t,n_forced*2)",
		}
		if hasAudio {
			args = append(args, "-c:a", "aac", "-b:a", fmt.Sprint(audioBitrate), "-ac", "2")
		} else {
			args = append(args, "-an")
		}
		args = append(args,
			"-f", "hls",
			"-hls_time", fmt.Sprint(segmentSeconds),
			"-hls_playlist_type", "vod",
			"-hls_segment_filename", filepath.Join(tmpDir, r.Name+"_%03d.ts"),
			filepath.Join(tmpDir, r.Name+".m3u8"),
		)
		tail := &tailWriter{max: 4096}
		cmd := exec.CommandContext(ctx, t.ffmpegBin, args...)
		cmd.Stdout = tail
		cmd.Stderr = tail
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("ffmpeg rendisi %s gagal: %w (keluaran: %s)", r.Name, err, strings.TrimSpace(tail.buf.String()))
		}
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "master.m3u8"),
		[]byte(masterPlaylist(renditions, hasAudio)), 0o644); err != nil {
		return "", fmt.Errorf("tidak dapat menulis master playlist: %w", err)
	}

	finalDir := filepath.Join(episodesDir, fmt.Sprintf("hls_%d", job.EpisodeID))
	_ = os.RemoveAll(finalDir)
	if err := os.Rename(tmpDir, finalDir); err != nil {
		return "", fmt.Errorf("tidak dapat mempublikasikan hasil transcode: %w", err)
	}
	return fmt.Sprintf("media/episodes/hls_%d/master.m3u8", job.EpisodeID), nil
}
