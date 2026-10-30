package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/lnx645/dracin-reels-backend/internal/application"
	"github.com/lnx645/dracin-reels-backend/internal/domain"
)

type ctxKey string

const (
	ctxUserID ctxKey = "userID"
	ctxRole   ctxKey = "role"
)

// TranscodeEnqueuer is the HLS worker entry point (implemented by
// internal/infrastructure/transcode). It is an interface here so the
// presentation layer does not import infrastructure.
type TranscodeEnqueuer interface {
	Enqueue(episodeID int64, sourceRel, sourceDisk string) bool
}

// Handlers bundles the usecases.
type Handlers struct {
	Auth       *application.AuthService
	Catalog    *application.CatalogService
	Wallet     *application.WalletService
	Social     *application.SocialService
	MediaDir   string
	Transcoder TranscodeEnqueuer
}

func userFromCtx(ctx context.Context) (int64, string) {
	id, _ := ctx.Value(ctxUserID).(int64)
	role, _ := ctx.Value(ctxRole).(string)
	return id, role
}

func writeOK(w http.ResponseWriter, status int, data interface{}, meta interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := map[string]interface{}{"success": true, "data": data}
	if meta != nil {
		resp["meta"] = meta
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func writeErr(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "INTERNAL_ERROR"
	msg := "Terjadi kesalahan di server."
	if ae, ok := err.(*domain.AppError); ok {
		status, code, msg = ae.Status, ae.Code, ae.Message
	} else if err != nil && strings.Contains(err.Error(), "no rows") {
		status, code, msg = 404, "NOT_FOUND", "Data tidak ditemukan."
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   map[string]string{"code": code, "message": msg},
	})
}

func decode(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "Body JSON tidak valid."))
		return false
	}
	return true
}

func pagination(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return page, limit
}

func pageMeta(page, limit, total int) map[string]int {
	return map[string]int{"page": page, "limit": limit, "total": total}
}

// --- Middleware ---

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handlers) optionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id, role, err := h.parseBearer(r); err == nil {
			r = r.WithContext(context.WithValue(r.Context(), ctxUserID, id))
			r = r.WithContext(context.WithValue(r.Context(), ctxRole, role))
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handlers) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, role, err := h.parseBearer(r)
		if err != nil {
			writeErr(w, domain.ErrUnauthorized("Silakan masuk dulu untuk mengakses ini."))
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), ctxUserID, id))
		r = r.WithContext(context.WithValue(r.Context(), ctxRole, role))
		next.ServeHTTP(w, r)
	})
}

func (h *Handlers) requireAdmin(next http.Handler) http.Handler {
	return h.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, role := userFromCtx(r.Context())
		if role != domain.RoleAdmin {
			writeErr(w, domain.ErrForbidden("Hanya admin yang boleh melakukan ini."))
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func (h *Handlers) parseBearer(r *http.Request) (int64, string, error) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return 0, "", domain.ErrUnauthorized("Token tidak ditemukan.")
	}
	return h.Auth.ParseToken(strings.TrimPrefix(auth, "Bearer "))
}
