package http

import (
	"net/http"

	"github.com/lnx645/dracin-reels-backend/internal/domain"
)

type seriesPayload struct {
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	CoverURL         *string `json:"cover_url"`
	Status           string  `json:"status"`
	TotalEpisode     int     `json:"total_episode"`
	FreeEpisodeCount int     `json:"free_episode_count"`
	Rating           float64 `json:"rating"`
	GenreIDs         []int64 `json:"genre_ids"`
}

type episodePayload struct {
	SeriesID        int64   `json:"series_id"`
	Number          int     `json:"number"`
	Title           string  `json:"title"`
	Description     string  `json:"description"`
	VideoURL        string  `json:"video_url"`
	SubtitleURL     *string `json:"subtitle_url"`
	DurationSeconds int     `json:"duration_seconds"`
	CoinPrice       int     `json:"coin_price"`
	Status          string  `json:"status"`
	ThumbnailURL    *string `json:"thumbnail_url"`
}

func (h *Handlers) AdminListSeries(w http.ResponseWriter, r *http.Request) {
	page, limit := pagination(r)
	list, total, err := h.Catalog.AdminListSeries(r.Context(), page, limit)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, list, pageMeta(page, limit, total))
}

func (h *Handlers) AdminCreateSeries(w http.ResponseWriter, r *http.Request) {
	var req seriesPayload
	if !decode(w, r, &req) {
		return
	}
	s := &domain.Series{Title: req.Title, Description: req.Description, CoverURL: req.CoverURL,
		Status: req.Status, TotalEpisode: req.TotalEpisode, FreeEpisodeCount: req.FreeEpisodeCount, Rating: req.Rating}
	if err := h.Catalog.CreateSeries(r.Context(), s, req.GenreIDs); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusCreated, s, nil)
}

func (h *Handlers) AdminUpdateSeries(w http.ResponseWriter, r *http.Request) {
	id, ok := paramID(r)
	if !ok {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "ID series tidak valid."))
		return
	}
	var req seriesPayload
	if !decode(w, r, &req) {
		return
	}
	s := &domain.Series{ID: id, Title: req.Title, Description: req.Description, CoverURL: req.CoverURL,
		Status: req.Status, TotalEpisode: req.TotalEpisode, FreeEpisodeCount: req.FreeEpisodeCount, Rating: req.Rating}
	if err := h.Catalog.UpdateSeries(r.Context(), s, req.GenreIDs); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, s, nil)
}

func (h *Handlers) AdminDeleteSeries(w http.ResponseWriter, r *http.Request) {
	id, ok := paramID(r)
	if !ok {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "ID series tidak valid."))
		return
	}
	if err := h.Catalog.DeleteSeries(r.Context(), id); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, map[string]string{"status": "ok"}, nil)
}

func (h *Handlers) AdminCreateEpisode(w http.ResponseWriter, r *http.Request) {
	var req episodePayload
	if !decode(w, r, &req) {
		return
	}
	e := &domain.Episode{SeriesID: req.SeriesID, Number: req.Number, Title: req.Title,
		Description: req.Description, VideoURL: req.VideoURL, SubtitleURL: req.SubtitleURL,
		DurationSeconds: req.DurationSeconds, CoinPrice: req.CoinPrice, Status: req.Status,
		ThumbnailURL: req.ThumbnailURL}
	if err := h.Catalog.CreateEpisode(r.Context(), e); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusCreated, e, nil)
}

func (h *Handlers) AdminUpdateEpisode(w http.ResponseWriter, r *http.Request) {
	id, ok := paramID(r)
	if !ok {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "ID episode tidak valid."))
		return
	}
	var req episodePayload
	if !decode(w, r, &req) {
		return
	}
	e := &domain.Episode{ID: id, SeriesID: req.SeriesID, Number: req.Number, Title: req.Title,
		Description: req.Description, VideoURL: req.VideoURL, SubtitleURL: req.SubtitleURL,
		DurationSeconds: req.DurationSeconds, CoinPrice: req.CoinPrice, Status: req.Status,
		ThumbnailURL: req.ThumbnailURL}
	if err := h.Catalog.UpdateEpisode(r.Context(), e); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, e, nil)
}

func (h *Handlers) AdminDeleteEpisode(w http.ResponseWriter, r *http.Request) {
	id, ok := paramID(r)
	if !ok {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "ID episode tidak valid."))
		return
	}
	if err := h.Catalog.DeleteEpisode(r.Context(), id); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, map[string]string{"status": "ok"}, nil)
}

func (h *Handlers) AdminCreateGenre(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if !decode(w, r, &req) {
		return
	}
	g := &domain.Genre{Name: req.Name, Slug: req.Slug}
	if err := h.Catalog.CreateGenre(r.Context(), g); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusCreated, g, nil)
}

func (h *Handlers) AdminUpdateGenre(w http.ResponseWriter, r *http.Request) {
	id, ok := paramID(r)
	if !ok {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "ID genre tidak valid."))
		return
	}
	var req struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if !decode(w, r, &req) {
		return
	}
	g := &domain.Genre{ID: id, Name: req.Name, Slug: req.Slug}
	if err := h.Catalog.UpdateGenre(r.Context(), g); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, g, nil)
}

func (h *Handlers) AdminDeleteGenre(w http.ResponseWriter, r *http.Request) {
	id, ok := paramID(r)
	if !ok {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "ID genre tidak valid."))
		return
	}
	if err := h.Catalog.DeleteGenre(r.Context(), id); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, map[string]string{"status": "ok"}, nil)
}

func (h *Handlers) AdminListCoinPackages(w http.ResponseWriter, r *http.Request) {
	list, err := h.Wallet.CoinPackages(r.Context(), false)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, list, nil)
}

func (h *Handlers) AdminCreateCoinPackage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string `json:"name"`
		CoinAmount int    `json:"coin_amount"`
		PriceIDR   int    `json:"price_idr"`
		BonusCoins int    `json:"bonus_coins"`
		Active     bool   `json:"active"`
	}
	if !decode(w, r, &req) {
		return
	}
	p := &domain.CoinPackage{Name: req.Name, CoinAmount: req.CoinAmount, PriceIDR: req.PriceIDR,
		BonusCoins: req.BonusCoins, Active: req.Active}
	if err := h.Wallet.CreateCoinPackage(r.Context(), p); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusCreated, p, nil)
}

func (h *Handlers) AdminUpdateCoinPackage(w http.ResponseWriter, r *http.Request) {
	id, ok := paramID(r)
	if !ok {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "ID paket tidak valid."))
		return
	}
	var req struct {
		Name       string `json:"name"`
		CoinAmount int    `json:"coin_amount"`
		PriceIDR   int    `json:"price_idr"`
		BonusCoins int    `json:"bonus_coins"`
		Active     bool   `json:"active"`
	}
	if !decode(w, r, &req) {
		return
	}
	p := &domain.CoinPackage{ID: id, Name: req.Name, CoinAmount: req.CoinAmount, PriceIDR: req.PriceIDR,
		BonusCoins: req.BonusCoins, Active: req.Active}
	if err := h.Wallet.UpdateCoinPackage(r.Context(), p); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, p, nil)
}

func (h *Handlers) AdminDeleteCoinPackage(w http.ResponseWriter, r *http.Request) {
	id, ok := paramID(r)
	if !ok {
		writeErr(w, domain.ErrBadRequest("VALIDATION_ERROR", "ID paket tidak valid."))
		return
	}
	if err := h.Wallet.DeleteCoinPackage(r.Context(), id); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, http.StatusOK, map[string]string{"status": "ok"}, nil)
}
