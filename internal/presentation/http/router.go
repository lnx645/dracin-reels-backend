package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Router wires all routes. This backend is API ONLY: no HTML pages,
// no server-rendered views. Web admin and the Flutter app are both
// plain API clients of /api/v1.
func (h *Handlers) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(cors)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeOK(w, http.StatusOK, map[string]string{"status": "ok"}, nil)
	})
	r.Get("/up", func(w http.ResponseWriter, r *http.Request) {
		writeOK(w, http.StatusOK, map[string]string{"status": "ok"}, nil)
	})
	// Staging/preview media (see upload.go header for the CDN warning).
	r.Handle("/media/*", http.HandlerFunc(h.MediaHandler))

	r.Route("/api/v1", func(r chi.Router) {
		// Auth — same contract for web admin and mobile app.
		r.Post("/auth/register", h.Register)
		r.Post("/auth/login", h.Login)
		r.With(h.requireAuth).Get("/auth/me", h.Me)

		// Public catalog.
		r.With(h.optionalAuth).Get("/series/{id}/ratings", h.GetSeriesRatings)
		r.With(h.optionalAuth).Get("/series", h.ListSeries)
		r.With(h.optionalAuth).Get("/series/{id}", h.GetSeries)
		r.With(h.optionalAuth).Get("/series/{id}/episodes", h.ListEpisodes)
		r.With(h.optionalAuth).Get("/episodes/{id}", h.GetEpisode)
		r.Get("/genres", h.ListGenres)
		r.Get("/coin-packages", h.ListCoinPackages)
		r.Get("/rewards/tasks", h.ListRewardTasks)
		r.With(h.optionalAuth).Get("/episodes/{id}/comments", h.ListComments)

		// Authenticated user endpoints.
		r.Group(func(r chi.Router) {
			r.Use(h.requireAuth)
			r.Get("/wallet", h.GetWallet)
			r.Get("/wallet/transactions", h.ListTransactions)
			r.Post("/unlock", h.Unlock)
			r.Post("/checkin", h.Checkin)
			r.Get("/vip", h.VIPStatus)
			r.Post("/vip/claim", h.ClaimVIP)
			r.Put("/watch-progress", h.SaveWatchProgress)
			r.Get("/continue-watching", h.ContinueWatching)
			r.Get("/bookmarks", h.ListBookmarks)
			r.Post("/bookmarks/{id}", h.Bookmark)
			r.Delete("/bookmarks/{id}", h.Unbookmark)
			r.Post("/likes/{id}", h.Like)
			r.Delete("/likes/{id}", h.Unlike)
			r.Post("/series/{id}/ratings", h.RateSeries)
			r.Post("/comments/{id}/like", h.LikeComment)
			r.Delete("/comments/{id}/like", h.UnlikeComment)
			r.Post("/episodes/{id}/comments", h.AddComment)
		})

		// Admin endpoints — identical for web admin and Flutter admin mode.
		// Guard: JWT required (401 without token) + role=admin (403 for viewers).
		r.Group(func(r chi.Router) {
			r.Use(h.requireAdmin)
			r.Get("/admin/series", h.AdminListSeries)
			r.Post("/admin/series", h.AdminCreateSeries)
			r.Put("/admin/series/{id}", h.AdminUpdateSeries)
			r.Delete("/admin/series/{id}", h.AdminDeleteSeries)
			r.Post("/admin/episodes", h.AdminCreateEpisode)
			r.Put("/admin/episodes/{id}", h.AdminUpdateEpisode)
			r.Delete("/admin/episodes/{id}", h.AdminDeleteEpisode)
			r.Post("/admin/genres", h.AdminCreateGenre)
			r.Put("/admin/genres/{id}", h.AdminUpdateGenre)
			r.Delete("/admin/genres/{id}", h.AdminDeleteGenre)
			r.Get("/admin/coin-packages", h.AdminListCoinPackages)
			r.Post("/admin/coin-packages", h.AdminCreateCoinPackage)
			r.Put("/admin/coin-packages/{id}", h.AdminUpdateCoinPackage)
			r.Delete("/admin/coin-packages/{id}", h.AdminDeleteCoinPackage)
			// Admin panel: dashboard, user/member management, full ledger.
			r.Get("/admin/stats", h.AdminStats)
			r.Get("/admin/users", h.AdminListUsers)
			r.Get("/admin/users/{id}", h.AdminGetUser)
			r.Put("/admin/users/{id}", h.AdminUpdateUser)
			r.Get("/admin/vip-subscriptions", h.AdminListVIPSubscriptions)
			r.Get("/admin/transactions", h.AdminListTransactions)
			// Admin media uploads (multipart, field name "file").
			r.Post("/admin/series/{id}/cover", h.UploadSeriesCover)
			r.Post("/admin/episodes/{id}/video", h.UploadEpisodeVideo)
			r.Post("/admin/episodes/{id}/subtitle", h.UploadEpisodeSubtitle)
			r.Post("/admin/episodes/{id}/thumbnail", h.UploadEpisodeThumbnail)
		})
	})
	return r
}
