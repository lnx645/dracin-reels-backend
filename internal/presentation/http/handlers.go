package http

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/lnx645/dracin-reels-backend/internal/domain"
)

func paramID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

// --- Auth ---
// Kontrak login (berlaku sama untuk web admin & mobile Flutter):
//   POST /api/v1/auth/login {email, password} -> {token, user}
// user selalu memuat role (viewer/admin) dan JWT claims memuat role yang
// sama, sehingga guard admin bisa diuji dari token saja.

func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	u, err := h.Auth.Register(r.Context(), req.Name, req.Email, req.Password)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusCreated, u, nil)
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	token, u, err := h.Auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, map[string]interface{}{"token": token, "user": u}, nil)
}

func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	id, _ := userFromCtx(r.Context())
	u, err := h.Auth.Me(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, u, nil)
}

// --- Public catalog ---

func (h *Handlers) ListSeries(w http.ResponseWriter, r *http.Request) {
	page, limit := pagination(r)
	list, total, err := h.Catalog.ListSeries(r.Context(), r.URL.Query().Get("genre"), page, limit)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, list, pageMeta(page, limit, total))
}

func (h *Handlers) GetSeries(w http.ResponseWriter, r *http.Request) {
	id, ok := paramID(r)
	if !ok {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "ID series tidak valid."))
		return
	}
	userID, role := userFromCtx(r.Context())
	series, eps, err := h.Catalog.GetSeries(r.Context(), id, userID, role == domain.RoleAdmin)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, map[string]interface{}{"series": series, "episodes": eps}, nil)
}

func (h *Handlers) ListEpisodes(w http.ResponseWriter, r *http.Request) {
	id, ok := paramID(r)
	if !ok {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "ID series tidak valid."))
		return
	}
	userID, role := userFromCtx(r.Context())
	eps, err := h.Catalog.ListEpisodes(r.Context(), id, userID, role == domain.RoleAdmin)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, eps, nil)
}

func (h *Handlers) GetEpisode(w http.ResponseWriter, r *http.Request) {
	id, ok := paramID(r)
	if !ok {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "ID episode tidak valid."))
		return
	}
	userID, role := userFromCtx(r.Context())
	ep, err := h.Catalog.GetEpisode(r.Context(), id, userID, role == domain.RoleAdmin)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, ep, nil)
}

func (h *Handlers) ListGenres(w http.ResponseWriter, r *http.Request) {
	list, err := h.Catalog.ListGenres(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, list, nil)
}

func (h *Handlers) ListCoinPackages(w http.ResponseWriter, r *http.Request) {
	list, err := h.Wallet.CoinPackages(r.Context(), true)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, list, nil)
}

func (h *Handlers) ListRewardTasks(w http.ResponseWriter, r *http.Request) {
	list, err := h.Wallet.ListRewardTasks(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, list, nil)
}

// --- Authenticated user endpoints ---

func (h *Handlers) GetWallet(w http.ResponseWriter, r *http.Request) {
	userID, _ := userFromCtx(r.Context())
	wallet, err := h.Wallet.GetWallet(r.Context(), userID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, wallet, nil)
}

func (h *Handlers) ListTransactions(w http.ResponseWriter, r *http.Request) {
	userID, _ := userFromCtx(r.Context())
	page, limit := pagination(r)
	list, total, err := h.Wallet.Transactions(r.Context(), userID, page, limit)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, list, pageMeta(page, limit, total))
}

func (h *Handlers) Unlock(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EpisodeID int64 `json:"episode_id"`
	}
	if !decode(w, r, &req) {
		return
	}
	userID, _ := userFromCtx(r.Context())
	wallet, err := h.Wallet.UnlockEpisode(r.Context(), userID, req.EpisodeID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, wallet, nil)
}

func (h *Handlers) Checkin(w http.ResponseWriter, r *http.Request) {
	userID, _ := userFromCtx(r.Context())
	wallet, reward, err := h.Wallet.Checkin(r.Context(), userID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, map[string]interface{}{"wallet": wallet, "coin_reward": reward}, nil)
}

func (h *Handlers) VIPStatus(w http.ResponseWriter, r *http.Request) {
	userID, _ := userFromCtx(r.Context())
	vip, err := h.Wallet.VIPStatus(r.Context(), userID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, vip, nil)
}

func (h *Handlers) ClaimVIP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Plan string `json:"plan"`
	}
	if !decode(w, r, &req) {
		return
	}
	userID, _ := userFromCtx(r.Context())
	vip, err := h.Wallet.ClaimVIP(r.Context(), userID, req.Plan)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, vip, nil)
}

func (h *Handlers) SaveWatchProgress(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EpisodeID       int64 `json:"episode_id"`
		PositionSeconds int   `json:"position_seconds"`
		Finished        bool  `json:"finished"`
	}
	if !decode(w, r, &req) {
		return
	}
	userID, _ := userFromCtx(r.Context())
	err := h.Wallet.SaveWatchProgress(r.Context(), userID, &domain.WatchProgress{
		EpisodeID: req.EpisodeID, PositionSeconds: req.PositionSeconds, Finished: req.Finished,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, map[string]string{"status": "ok"}, nil)
}

func (h *Handlers) ContinueWatching(w http.ResponseWriter, r *http.Request) {
	userID, _ := userFromCtx(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := h.Wallet.ContinueWatching(r.Context(), userID, limit)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, list, nil)
}

func (h *Handlers) Bookmark(w http.ResponseWriter, r *http.Request) {
	id, ok := paramID(r)
	if !ok {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "ID series tidak valid."))
		return
	}
	userID, _ := userFromCtx(r.Context())
	if err := h.Social.Bookmark(r.Context(), userID, id); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, map[string]string{"status": "ok"}, nil)
}

func (h *Handlers) Unbookmark(w http.ResponseWriter, r *http.Request) {
	id, ok := paramID(r)
	if !ok {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "ID series tidak valid."))
		return
	}
	userID, _ := userFromCtx(r.Context())
	if err := h.Social.Unbookmark(r.Context(), userID, id); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, map[string]string{"status": "ok"}, nil)
}

func (h *Handlers) ListBookmarks(w http.ResponseWriter, r *http.Request) {
	userID, _ := userFromCtx(r.Context())
	list, err := h.Social.Bookmarks(r.Context(), userID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, list, nil)
}

func (h *Handlers) Like(w http.ResponseWriter, r *http.Request) {
	id, ok := paramID(r)
	if !ok {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "ID episode tidak valid."))
		return
	}
	userID, _ := userFromCtx(r.Context())
	if err := h.Social.Like(r.Context(), userID, id); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, map[string]string{"status": "ok"}, nil)
}

func (h *Handlers) Unlike(w http.ResponseWriter, r *http.Request) {
	id, ok := paramID(r)
	if !ok {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "ID episode tidak valid."))
		return
	}
	userID, _ := userFromCtx(r.Context())
	if err := h.Social.Unlike(r.Context(), userID, id); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, map[string]string{"status": "ok"}, nil)
}

func (h *Handlers) ListComments(w http.ResponseWriter, r *http.Request) {
	id, ok := paramID(r)
	if !ok {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "ID episode tidak valid."))
		return
	}
	page, limit := pagination(r)
	list, total, err := h.Social.Comments(r.Context(), id, page, limit)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, list, pageMeta(page, limit, total))
}

func (h *Handlers) AddComment(w http.ResponseWriter, r *http.Request) {
	id, ok := paramID(r)
	if !ok {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "ID episode tidak valid."))
		return
	}
	var req struct {
		Body string `json:"body"`
	}
	if !decode(w, r, &req) {
		return
	}
	userID, _ := userFromCtx(r.Context())
	c := &domain.Comment{EpisodeID: id, Body: req.Body}
	if err := h.Social.AddComment(r.Context(), userID, c); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusCreated, c, nil)
}
