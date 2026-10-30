package http

import (
	"net/http"
	"strconv"

	"github.com/lnx645/dracin-reels-backend/internal/domain"
)

// AdminStats returns the live dashboard summary (counts + latest rows).
func (h *Handlers) AdminStats(w http.ResponseWriter, r *http.Request) {
	st, err := h.Admin.Stats(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, st, nil)
}

// AdminListUsers lists accounts with live balances and VIP flags.
// Query params: q (search name/email, optional), page, limit.
func (h *Handlers) AdminListUsers(w http.ResponseWriter, r *http.Request) {
	page, limit := pagination(r)
	list, total, err := h.Admin.ListUsers(r.Context(), r.URL.Query().Get("q"), page, limit)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, list, pageMeta(page, limit, total))
}

// AdminGetUser returns one account's admin view.
func (h *Handlers) AdminGetUser(w http.ResponseWriter, r *http.Request) {
	id, ok := paramID(r)
	if !ok {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "ID pengguna tidak valid."))
		return
	}
	v, err := h.Admin.GetUser(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, v, nil)
}

// AdminUpdateUser changes only role and/or active status. Passwords are
// never accepted here, users are never deleted here, and an admin cannot
// change their own role or status (that would lock the only admin out).
func (h *Handlers) AdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := paramID(r)
	if !ok {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "ID pengguna tidak valid."))
		return
	}
	callerID, _ := userFromCtx(r.Context())
	if callerID == id {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "Tidak dapat mengubah akun sendiri."))
		return
	}
	var req struct {
		Role     *string `json:"role"`
		IsActive *bool   `json:"is_active"`
	}
	if !decode(w, r, &req) {
		return
	}
	v, err := h.Admin.UpdateUser(r.Context(), id, req.Role, req.IsActive)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, v, nil)
}

// AdminListVIPSubscriptions lists VIP memberships with effective status.
// Query params: status (active|expired, optional), page, limit.
func (h *Handlers) AdminListVIPSubscriptions(w http.ResponseWriter, r *http.Request) {
	page, limit := pagination(r)
	list, total, err := h.Admin.ListVIPSubscriptions(r.Context(), r.URL.Query().Get("status"), page, limit)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, list, pageMeta(page, limit, total))
}

// AdminListTransactions lists ALL coin transactions (not just the
// caller's wallet). Query params: user_id, type, page, limit.
func (h *Handlers) AdminListTransactions(w http.ResponseWriter, r *http.Request) {
	page, limit := pagination(r)
	var userID int64
	if raw := r.URL.Query().Get("user_id"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v < 0 {
			writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "user_id tidak valid."))
			return
		}
		userID = v
	}
	list, total, err := h.Admin.ListTransactions(r.Context(), userID, r.URL.Query().Get("type"), page, limit)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, list, pageMeta(page, limit, total))
}
