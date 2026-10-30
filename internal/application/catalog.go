package application

import (
	"context"
	"net/url"
	"strings"

	"github.com/lnx645/dracin-reels-backend/internal/domain"
)

// validateVideoURL enforces the two-source rule for episode video:
//   - empty is allowed (video may come later via admin file upload);
//   - a relative "media/..." path is allowed (result of a prior admin upload);
//   - otherwise it must be a valid http(s) URL with a host.
//   - any other scheme (javascript:, file:, ftp:, ...) is rejected.
func validateVideoURL(raw string) error {
	v := strings.TrimSpace(raw)
	if v == "" || strings.HasPrefix(v, "media/") {
		return nil
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return domain.ErrBadRequest("INVALID_VIDEO_URL",
			"URL video tidak valid. Gunakan tautan http(s) yang valid, atau kosongkan bila video akan diupload.")
	}
	return nil
}

// CatalogService serves series, episodes and genres.
type CatalogService struct{ store Store }

func NewCatalogService(store Store) *CatalogService { return &CatalogService{store: store} }

func (s *CatalogService) ListSeries(ctx context.Context, genreSlug string, genreID int64, page, limit int) ([]domain.Series, int, error) {
	return s.store.ListSeries(ctx, genreSlug, genreID, domain.StatusPublished, page, limit)
}

func (s *CatalogService) GetSeries(ctx context.Context, id int64, userID int64, isAdmin bool) (*domain.Series, []domain.Episode, error) {
	series, err := s.store.GetSeries(ctx, id)
	if err != nil {
		return nil, nil, domain.ErrNotFound("Series")
	}
	if series.Status != domain.StatusPublished && !isAdmin {
		return nil, nil, domain.ErrNotFound("Series")
	}
	eps, err := s.store.ListEpisodesBySeries(ctx, id, !isAdmin)
	if err != nil {
		return nil, nil, err
	}
	eps = applyLock(ctx, s.store, series, eps, userID, isAdmin)
	return series, eps, nil
}

func (s *CatalogService) ListEpisodes(ctx context.Context, seriesID int64, userID int64, isAdmin bool) ([]domain.Episode, error) {
	series, err := s.store.GetSeries(ctx, seriesID)
	if err != nil {
		return nil, domain.ErrNotFound("Series")
	}
	eps, err := s.store.ListEpisodesBySeries(ctx, seriesID, !isAdmin)
	if err != nil {
		return nil, err
	}
	return applyLock(ctx, s.store, series, eps, userID, isAdmin), nil
}

// GetEpisode returns one episode, hiding video_url when locked (rule 11).
func (s *CatalogService) GetEpisode(ctx context.Context, id int64, userID int64, isAdmin bool) (*domain.Episode, error) {
	ep, err := s.store.GetEpisode(ctx, id)
	if err != nil {
		return nil, domain.ErrNotFound("Episode")
	}
	if ep.Status != domain.StatusPublished && !isAdmin {
		return nil, domain.ErrNotFound("Episode")
	}
	series, err := s.store.GetSeries(ctx, ep.SeriesID)
	if err != nil {
		return nil, domain.ErrNotFound("Series")
	}
	locked, err := s.isLocked(ctx, series, ep, userID, isAdmin)
	if err != nil {
		return nil, err
	}
	ep.Locked = locked
	if locked {
		ep.VideoURL = ""
		if ep.SubtitleURL != nil {
			empty := ""
			ep.SubtitleURL = &empty
		}
	}
	return ep, nil
}

// isLocked implements rules 1-4: free range, VIP, prior unlock.
func (s *CatalogService) isLocked(ctx context.Context, series *domain.Series, ep *domain.Episode, userID int64, isAdmin bool) (bool, error) {
	if isAdmin {
		return false, nil
	}
	if series.IsFree(ep.Number) || ep.CoinPrice <= 0 {
		return false, nil
	}
	if userID == 0 {
		return true, nil
	}
	if vip, err := s.store.ActiveVIP(ctx, userID); err == nil && vip != nil {
		return false, nil
	}
	unlocked, err := s.store.IsEpisodeUnlocked(ctx, userID, ep.ID)
	if err != nil {
		return false, err
	}
	return !unlocked, nil
}

func applyLock(ctx context.Context, store Store, series *domain.Series, eps []domain.Episode, userID int64, isAdmin bool) []domain.Episode {
	svc := &CatalogService{store: store}
	for i := range eps {
		locked, err := svc.isLocked(ctx, series, &eps[i], userID, isAdmin)
		if err != nil {
			locked = true
		}
		eps[i].Locked = locked
		if locked {
			eps[i].VideoURL = ""
		}
	}
	return eps
}

func (s *CatalogService) ListGenres(ctx context.Context) ([]domain.Genre, error) {
	return s.store.ListGenres(ctx)
}

// --- Admin catalog ---

func (s *CatalogService) AdminListSeries(ctx context.Context, page, limit int) ([]domain.Series, int, error) {
	return s.store.ListSeries(ctx, "", 0, "", page, limit)
}

func (s *CatalogService) CreateSeries(ctx context.Context, series *domain.Series, genreIDs []int64) error {
	if series.Title == "" {
		return domain.ErrBadRequest("VALIDATION_ERROR", "Judul series wajib diisi.")
	}
	if series.Status == "" {
		series.Status = domain.StatusDraft
	}
	return s.store.CreateSeries(ctx, series, genreIDs)
}

func (s *CatalogService) UpdateSeries(ctx context.Context, series *domain.Series, genreIDs []int64) error {
	if _, err := s.store.GetSeries(ctx, series.ID); err != nil {
		return domain.ErrNotFound("Series")
	}
	return s.store.UpdateSeries(ctx, series, genreIDs)
}

func (s *CatalogService) DeleteSeries(ctx context.Context, id int64) error {
	return s.store.DeleteSeries(ctx, id)
}

func (s *CatalogService) CreateEpisode(ctx context.Context, ep *domain.Episode) error {
	if err := validateVideoURL(ep.VideoURL); err != nil {
		return err
	}
	if _, err := s.store.GetSeries(ctx, ep.SeriesID); err != nil {
		return domain.ErrNotFound("Series")
	}
	if ep.Status == "" {
		ep.Status = domain.StatusDraft
	}
	return s.store.CreateEpisode(ctx, ep)
}

func (s *CatalogService) UpdateEpisode(ctx context.Context, ep *domain.Episode) error {
	if err := validateVideoURL(ep.VideoURL); err != nil {
		return err
	}
	if _, err := s.store.GetEpisode(ctx, ep.ID); err != nil {
		return domain.ErrNotFound("Episode")
	}
	return s.store.UpdateEpisode(ctx, ep)
}

// UpdateEpisodeTranscode writes only the HLS fields (see postgres store).
func (s *CatalogService) UpdateEpisodeTranscode(ctx context.Context, id int64, hlsURL *string, status string, errMsg *string) error {
	return s.store.UpdateEpisodeTranscode(ctx, id, hlsURL, status, errMsg)
}

func (s *CatalogService) DeleteEpisode(ctx context.Context, id int64) error {
	return s.store.DeleteEpisode(ctx, id)
}

func (s *CatalogService) CreateGenre(ctx context.Context, g *domain.Genre) error {
	if g.Name == "" || g.Slug == "" {
		return domain.ErrBadRequest("VALIDATION_ERROR", "Nama dan slug genre wajib diisi.")
	}
	return s.store.CreateGenre(ctx, g)
}

func (s *CatalogService) UpdateGenre(ctx context.Context, g *domain.Genre) error {
	return s.store.UpdateGenre(ctx, g)
}

func (s *CatalogService) DeleteGenre(ctx context.Context, id int64) error {
	return s.store.DeleteGenre(ctx, id)
}
