package application

import (
	"context"
	"strings"

	"github.com/lnx645/dracin-reels-backend/internal/domain"
)

// SocialService handles bookmarks, likes and comments.
type SocialService struct{ store Store }

func NewSocialService(store Store) *SocialService { return &SocialService{store: store} }

func (s *SocialService) Bookmark(ctx context.Context, userID, seriesID int64) error {
	if _, err := s.store.GetSeries(ctx, seriesID); err != nil {
		return domain.ErrNotFound("Series")
	}
	return s.store.SetBookmark(ctx, userID, seriesID)
}

func (s *SocialService) Unbookmark(ctx context.Context, userID, seriesID int64) error {
	return s.store.RemoveBookmark(ctx, userID, seriesID)
}

func (s *SocialService) Bookmarks(ctx context.Context, userID int64) ([]domain.Series, error) {
	return s.store.ListBookmarks(ctx, userID)
}

func (s *SocialService) Like(ctx context.Context, userID, episodeID int64) error {
	if _, err := s.store.GetEpisode(ctx, episodeID); err != nil {
		return domain.ErrNotFound("Episode")
	}
	return s.store.SetLike(ctx, userID, episodeID)
}

func (s *SocialService) Unlike(ctx context.Context, userID, episodeID int64) error {
	return s.store.RemoveLike(ctx, userID, episodeID)
}

func (s *SocialService) Comments(ctx context.Context, episodeID int64, page, limit int) ([]domain.Comment, int, error) {
	if _, err := s.store.GetEpisode(ctx, episodeID); err != nil {
		return nil, 0, domain.ErrNotFound("Episode")
	}
	return s.store.ListComments(ctx, episodeID, page, limit)
}

func (s *SocialService) AddComment(ctx context.Context, userID int64, c *domain.Comment) error {
	if strings.TrimSpace(c.Body) == "" {
		return domain.ErrBadRequest("VALIDATION_ERROR", "Isi komentar wajib diisi.")
	}
	if _, err := s.store.GetEpisode(ctx, c.EpisodeID); err != nil {
		return domain.ErrNotFound("Episode")
	}
	c.UserID = userID
	return s.store.CreateComment(ctx, c)
}
