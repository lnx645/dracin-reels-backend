package http

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/lnx645/dracin-reels-backend/internal/domain"
)

// Admin media upload (staging/preview phase only).
//
// Storage: MEDIA_DIR/series/ for covers, MEDIA_DIR/episodes/ for video,
// subtitle and thumbnail. The stored URL saved to the entity is a path
// relative to the API base, e.g. "media/series/12-cover-1700000000-poster.png".
// Clients request it as <api-base>/media/series/... — with the nginx proxy
// that is http://103.147.33.11:8095/dracin-api/media/series/...
//
// Limits: images 5 MB, video 500 MB, subtitles 5 MB.
// Validation: extension whitelist + multipart Content-Type sanity check
// (application/octet-stream is tolerated, because some clients send it).
// Filenames are sanitized (no path traversal) and made unique.
// Guard: requireAdmin — no token = 401, viewer token = 403. Uploads never
// return fake success: every failure is a JSON error, never a URL.

type uploadKind struct {
	subdir     string
	extensions map[string]bool
	mimePrefix []string
	maxBytes   int64
	field      string // entity field updated
}

var uploadKinds = map[string]uploadKind{
	"cover": {
		subdir:     "series",
		extensions: map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true},
		mimePrefix: []string{"image/"},
		maxBytes:   5 << 20,
		field:      "cover_url",
	},
	"video": {
		subdir:     "episodes",
		extensions: map[string]bool{".mp4": true, ".mov": true, ".mkv": true},
		mimePrefix: []string{"video/"},
		maxBytes:   500 << 20,
		field:      "video_url",
	},
	"subtitle": {
		subdir:     "episodes",
		extensions: map[string]bool{".vtt": true, ".srt": true},
		mimePrefix: []string{"text/", "application/x-subrip"},
		maxBytes:   5 << 20,
		field:      "subtitle_url",
	},
	"thumbnail": {
		subdir:     "episodes",
		extensions: map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true},
		mimePrefix: []string{"image/"},
		maxBytes:   5 << 20,
		field:      "thumbnail_url",
	},
}

func sanitizeFilename(name string) string {
	base := path.Base(name)
	base = strings.ReplaceAll(base, string(filepath.Separator), "_")
	var b strings.Builder
	for _, r := range base {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "._")
	if out == "" {
		out = "file"
	}
	return out
}

func (h *Handlers) saveUpload(w http.ResponseWriter, r *http.Request, kindName string) (string, bool) {
	kind, ok := uploadKinds[kindName]
	if !ok {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "Jenis upload tidak valid."))
		return "", false
	}
	// Cap the whole request body slightly above the file limit.
	r.Body = http.MaxBytesReader(w, r.Body, kind.maxBytes+ (1 << 20))
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeErr(w, domain.ErrBadRequest("FILE_TOO_LARGE",
			fmt.Sprintf("Ukuran file melebihi batas %d MB.", kind.maxBytes>>20)))
		return "", false
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "Field file (multipart) wajib diisi."))
		return "", false
	}
	defer file.Close()
	if header.Size > kind.maxBytes {
		writeErr(w, domain.ErrBadRequest("FILE_TOO_LARGE",
			fmt.Sprintf("Ukuran file melebihi batas %d MB.", kind.maxBytes>>20)))
		return "", false
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !kind.extensions[ext] {
		writeErr(w, domain.ErrBadRequest("INVALID_FILE_TYPE",
			"Tipe file tidak diizinkan untuk upload ini."))
		return "", false
	}
	if ct := header.Header.Get("Content-Type"); ct != "" && ct != "application/octet-stream" {
		allowed := false
		for _, p := range kind.mimePrefix {
			if strings.HasPrefix(ct, p) {
				allowed = true
				break
			}
		}
		if !allowed {
			writeErr(w, domain.ErrBadRequest("INVALID_FILE_TYPE",
				"Tipe file tidak diizinkan untuk upload ini."))
			return "", false
		}
	}
	if h.MediaDir == "" {
		h.MediaDir = "media"
	}
	dir := filepath.Join(h.MediaDir, kind.subdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeErr(w, err)
		return "", false
	}
	safe := sanitizeFilename(header.Filename)
	unique := fmt.Sprintf("%d-%s", time.Now().UnixNano(), safe)
	dstPath := filepath.Join(dir, unique)
	dst, err := os.Create(dstPath)
	if err != nil {
		writeErr(w, err)
		return "", false
	}
	if _, err := io.Copy(dst, file); err != nil {
		_ = dst.Close()
		_ = os.Remove(dstPath)
		writeErr(w, err)
		return "", false
	}
	if err := dst.Close(); err != nil {
		_ = os.Remove(dstPath)
		writeErr(w, err)
		return "", false
	}
	// Relative API path stored in the entity (see header comment).
	return "media/" + kind.subdir + "/" + unique, true
}

func (h *Handlers) UploadSeriesCover(w http.ResponseWriter, r *http.Request) {
	id, ok := paramID(r)
	if !ok {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "ID series tidak valid."))
		return
	}
	ser, err := h.storeSeries(r, id)
	if err != nil {
		writeErr(w, err)
		return
	}
	rel, ok := h.saveUpload(w, r, "cover")
	if !ok {
		return
	}
	ser.CoverURL = &rel
	if err := h.Catalog.UpdateSeries(r.Context(), ser, nil); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, map[string]interface{}{
		"series_id": id, "cover_url": rel, "url": rel,
	}, nil)
}

func (h *Handlers) UploadEpisodeVideo(w http.ResponseWriter, r *http.Request) {
	h.uploadEpisodeField(w, r, "video", func(ep *domain.Episode, rel string) {
		ep.VideoURL = rel
	})
}

func (h *Handlers) UploadEpisodeSubtitle(w http.ResponseWriter, r *http.Request) {
	h.uploadEpisodeField(w, r, "subtitle", func(ep *domain.Episode, rel string) {
		ep.SubtitleURL = &rel
	})
}

func (h *Handlers) UploadEpisodeThumbnail(w http.ResponseWriter, r *http.Request) {
	h.uploadEpisodeField(w, r, "thumbnail", func(ep *domain.Episode, rel string) {
		ep.ThumbnailURL = &rel
	})
}

func (h *Handlers) uploadEpisodeField(w http.ResponseWriter, r *http.Request, kindName string, set func(*domain.Episode, string)) {
	id, ok := paramID(r)
	if !ok {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "ID episode tidak valid."))
		return
	}
	ep, err := h.Catalog.GetEpisode(r.Context(), id, 0, true)
	if err != nil {
		writeErr(w, err)
		return
	}
	rel, ok := h.saveUpload(w, r, kindName)
	if !ok {
		return
	}
	set(ep, rel)
	if err := h.Catalog.UpdateEpisode(r.Context(), ep); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, map[string]interface{}{
		"episode_id": id, uploadKinds[kindName].field: rel, "url": rel,
	}, nil)
}

// storeSeries fetches a series for admin mutation (genres preserved on update
// because UpdateSeries only replaces genres when genreIDs != nil).
func (h *Handlers) storeSeries(r *http.Request, id int64) (*domain.Series, error) {
	// Catalog.GetSeries with isAdmin=true returns series + episodes; reuse it
	// and drop the episodes. It also enforces existence (404 otherwise).
	ser, _, err := h.Catalog.GetSeries(r.Context(), id, 0, true)
	return ser, err
}

// MediaHandler serves uploaded files as staging/preview only.
// No auth gate: these are staging URLs, NOT mass-public distribution.
// Before any mass-public launch, media MUST move to object storage + CDN
// with signed URLs (see README). Directory listings are disabled.
func (h *Handlers) MediaHandler(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/media/")
	clean := path.Clean("/" + rel)
	if clean == "/" || strings.Contains(clean, "..") {
		http.NotFound(w, r)
		return
	}
	full := filepath.Join(h.MediaDir, filepath.FromSlash(clean))
	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, full)
}
